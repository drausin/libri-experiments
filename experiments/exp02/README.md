# exp02 - What are the request/response balances between well-behaved peers?

What are the request/response patterns between well-behaved (i.e., non-malicious) peers? If we 
understand these dynamics, we can model them and exclude outliers (leechers or possibly malicious 
peers) to these models from participating in the network.

The rationale behind this approach is that over time, peers will develop "relationships" and 
"goodwill" with each other, based on (long) histories of mutual requests and responses, much like
people in a community over time develop trust with one another after many interactions.

This set of experiments is intended to
- characterize request/response balances between well-behaved peers
- introduce models for these balances
- use these models to detect outlier balances stemming from malicious peers

### Methods & Results

#### Experiment setup

The experimental setup is similar to that of the trials in [exp01](../exp01). Each trial is defined
by a new libri cluster and user simulator pattern.

Instead of taking Grafana screen shots as in exp01, we wrote very simple (and rough) 
[collector](analysis/collect.go) to query Prometheus directly and save the results to local files. 
These results are saved in the `/data` directory of each trial. We collected the following
- Get & Put endpoint p50 & p95 over the duration of the experiment
- peer query counts for every (requestor ID, responder ID, endpoint, success/error outcome)

exp01 trials all involved a small, 8-node cluster. In exploring these balances, we realize that we 
need to test on larger clusters, up to 64 nodes in order for the routing table to split into 
multiple buckets, and for those buckets to fill up.

The trial writeups below are more summaries and discussion. Raw results and code can be found in 
the [analysis Jupyter notebook](analysis/Analysis.ipynb).

#### Find request/response ratio divergence with increasing cluster size

Trials 1-4 increased the libri cluster size from 8 to 16 to 32 to 64 peers. By 64 peers in trial 4, 
we noticed a diverence in the Find endpoint request/response ratio that was not present in smaller
clusters. 

This divergence comes from how the routing table buckets were intended to behave. They fill up with 
peers and then are effectively closed to new peers. This means that peer A may have peer B in its 
routing table and makes Find requests to it, but peer B doesn't have peer A and thus doesn't make 
Find requests to it. This assymmetry would explain the divergence we're seeing.


#### Tightening Find request/response ratio divergence

Trial 5 tried to replicate the divergence issue seen in trial 4 on a 32-node cluster with 8 max 
bucket peers. We were unable to replicate the divergence, but there was a pretty broad distribution
of request/response fractions.

Trial 6 tested a tweak to how peers were preferred ([#184](https://github.com/drausin/libri/pull/184))
without much difference from trial 5.

Trial 7 tested a change ([39e5717](https://github.com/drausin/libri/commit/39e5717e4abd29f288bfa4643e632532ce277349))
to how peers are selected from within a bucket. Instead of selecting peers based on response time 
or counts, we select them based on proximity to a given target. As expected/desired, this change 
resulted in tighter request/response fraction distribution. 

Trial 8 tested a tweak ([52b8089](https://github.com/drausin/libri/commit/52b808993c755f750b104be8cb6b4a9dab932e76)) 
to how peers within a bucket are preferred, without much effect.

Trial 9 ran the same code from trial 8 on a 64-node cluster for 8 hours to test whether the changes
tested in trials 6-8 tighted/avoided the divergence that we saw in trial 4. The divergence didn't
occur.

Trial 10 was very similar to trial 9 except using 16 max bucket peers using similar prefer logic
([#185](https://github.com/drausin/libri/commit/2cd68f0a0742382cf01aafe0884f2c37dfe52b8a)) as 
trial 9. The divergence results were fairly comparable to those in trial 9.

Trial 11 was similar to trial 10 but for a 32-node cluster. Divergence was quite good.

#### Asymmetric Put/Get load

Trials 12-14 tested asymmetric load on a 32-node cluster. In trial 12, librarians 0-23 accepted Put 
requests and 8-31 accepted Good requests. Trial 13 used 0-15 for Puts and 16-31 for Gets. Trial 14
used 0-8 for Puts and 24-31 for Gets.

In each trial, Find & Store request/response ratios were stable but often quite varying across the
peers. Scatter plots of of the number of requests and responses for each endpoint of a given peer
also didn't reveal many patterns.

### Discussion

If under a very controlled experiment we were unable to effectively model and predict what the 
expected behavior of nodes under asymmetrical (i.e., more realistic) Get & Put load, we believe it
will be very hard to do so in the wild. Despite the amount of effort already sunk into "Goodwill"
as a way for promoting productive, long-term relationships between well-intentioned peers and 
thwarting malicious peers, it seems that it will be hard, if not impossible, to effectively 
implement.

A simpler approach (proposed by John Urbanik) would be for each peer to just rate-limit requests
from other peers. Each peer would also rate-limit the number of unique peers it is willing to accept 
requests from, so a malicious acttor could not launch a [Sybil attacks](https://en.wikipedia.org/wiki/Sybil_attack).
Furthermore, we could add tiered rate limiting, so peers that are configured to be "known" (formal 
definition still TBD) would have (potentially much) higher limits than those that are unknown. We 
might assume that even if malicious actors (which we assume to be "unknown") launched an attack,
the fraction of requests they could make would be much lower than the requests from "known" peers.

So, the conclusion of experiment 02 is that request/response patterns between peers are 
sufficiently hard to predict that it's not worth building our incentive system around it. Instead,
we will use a much simpler, easier to understand and configure approach for rate limiting 
([#186](https://github.com/drausin/libri/pull/186)).   
 
 
   




