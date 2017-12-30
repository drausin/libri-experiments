## exp01: How does performance degrade with increasing user load?

What happens to libri network performance as user load increases? Network performance is measured
primarily by metrics on the aggregate librarians:
- CPU usage
- memory usage
- Put/Get response times (p50, p95)
- availability %

Our goal over the course of this experiment was to achieve sub-second latency and 99.99% 
availability for Get and Put requests at a million average users with reasonable resource usage 
(e.g., less than 1 CPU and 8 GB RAM per librarian).

### Methods & Results

#### Experiment setup

The main independent variable for now is the number of uploads per day (UPD), which is just the 
number of authors times the assumed number of documents each author uploads per day. We keep 
number of authors fixed at 100 to avoid practical issues like too many open RocksDB files, but
the "real" assumption is that UPD is a rough proxy for number of authors (i.e., users), since we 
don't expect each author to upload more that one document per day (at least for now).  

Each upload translates to 8 Put/Get queries:
- Put entry (always will just be a single-page entry b/c content size distribution is always < 2 MB)
- Put envelope (self reader)
- 2x Put envelope (shared authors)
- 2x Get envelope (shared authors)
- 2x Get entry (shared authors)

We store the configuration and results of each trial in a separate directory containing the 
following
- `terraform.tfvars` defining the parameters (i.e., independent variables) of the experiment, both 
for the libri cluster and the user simulator  
- `img` directory with Grafana (and sometimes Prometheus) screenshots representing the results of 
the experiment
- `libri.yml` Kubernetes configuration file for the libri cluster
- `libri-sim.yml` Kubernetes configuration file for the user load simulator
- `variables.tf` Kubernetes variable definition file (only for libri variables)  

The shell commands used to initialize the experiment were roughly
```bash
cd ~/.go/src/github.com/drausin/libri/deploy/cloud
EXP_NUM="01"
TRIAL_NUM="01"
CLUSTER_DIR="~/.go/src/github.com/drausin/libri-experiments/experiments/exp${EXP_NUM}/trial${TRIAL_NUM}"
CLUSTER_NAME="exp${EXP_NUM}-trial${TRIAL_NUM}"
go run cluster.go init gcp --clusterDir "${CLUSTER_DIR}" --clusterName ${CLUSTER_NAME} \
--bucket my-clusters-bucket --gcpProject my-gcp-project
```
We then edited the `terraform.tfvars` (often just copying it from the previous trial and modifying 
slightly) to set the appropriate variables for the libri cluster and experimenter pod and generated 
`libri-sim.yml` via
```bash
cd ~/.go/src/github.com/drausin/libri-experiments/deploy/cloud/
go run gen.go -e "${CLUSTER_DIR}/terraform.tfvars" -d ${CLUSTER_DIR}
```
Occasionally we would hand-modify `libri-sim.yml` when we wanted to set a parameter not templated
and handled by `gen.go` (e.g., experimenter pod resource limits). 

To create the infrastructure (via Terraform) and start the cluster (via Kubernetes), we
```bash
cd ~/.go/src/github.com/drausin/libri/deploy/cloud
go run cluster.go apply --clusterDir ${CLUSTER_DIR}
```
Once all the services are up, it would look like
```bash
$ gcloud compute instances list
NAME                                          ZONE        MACHINE_TYPE  PREEMPTIBLE  INTERNAL_IP  EXTERNAL_IP     STATUS
gke-exp01-trial01-default-pool-a13cf8e1-hpxm  us-east1-b  n1-highmem-2               10.142.0.4   35.196.131.101  RUNNING
gke-exp01-trial01-default-pool-a13cf8e1-l0vf  us-east1-b  n1-highmem-2               10.142.0.5   35.196.233.175  RUNNING
$ kubectl get pods -o wide
NAME                          READY     STATUS    RESTARTS   AGE       IP           NODE
grafana-68785230-n7jrz        1/1       Running   0          4m        10.12.2.4    gke-exp01-trial01-default-pool-a13cf8e1-dczj
librarians-0                  1/1       Running   0          4m        10.12.0.4    gke-exp01-trial01-default-pool-a13cf8e1-hpxm
librarians-1                  1/1       Running   0          4m        10.12.3.5    gke-exp01-trial01-default-pool-a13cf8e1-l0vf
librarians-2                  1/1       Running   0          3m        10.12.2.6    gke-exp01-trial01-default-pool-a13cf8e1-dczj
librarians-3                  1/1       Running   0          3m        10.12.1.4    gke-exp01-trial01-default-pool-a13cf8e1-m15c
librarians-4                  1/1       Running   0          2m        10.12.3.6    gke-exp01-trial01-default-pool-a13cf8e1-l0vf
librarians-5                  1/1       Running   0          2m        10.12.0.5    gke-exp01-trial01-default-pool-a13cf8e1-hpxm
librarians-6                  1/1       Running   0          2m        10.12.2.7    gke-exp01-trial01-default-pool-a13cf8e1-dczj
librarians-7                  1/1       Running   0          1m        10.12.1.5    gke-exp01-trial01-default-pool-a13cf8e1-m15c
node-exporter-4b5dm           1/1       Running   0          4m        10.142.0.4   gke-exp01-trial01-default-pool-a13cf8e1-hpxm
node-exporter-7cglv           1/1       Running   0          4m        10.142.0.5   gke-exp01-trial01-default-pool-a13cf8e1-l0vf
prometheus-3613992385-hc62q   1/1       Running   0          4m        10.12.2.5    gke-exp01-trial01-default-pool-a13cf8e1-dczj
```
At this point, we can start the user simulator
```bash
kubectl create -f "${CLUSTER_DIR}/libri-sim.yml"
```

Once the experiment was complete and all results were gathered, we cleaned everything up with
```bash
kubectl delete -f "${CLUSTER_DIR}/libri-sim.yml" -f "${CLUSTER_DIR}/libri.yml"
terraform destroy ${CLUSTER_DIR}
``` 

#### Naive user load increases

We first performed the 8 trials, doubling the UPD every trial, and observing primarily the 
Get/Put p50 & p95 latencies and librarian CPU & memory usage. Each trial ran for 30 minutes. 
The latencies and resources reported below are eyeballed from Grafana (via Prometheus metrics) 
screenshots. We report roughly the highest value across all the librarians over the duration of
the experiment. 

The results of these 8 trials are given below.

| trial | UPD   | Get p50   | Get p95   | Put p50   | Put p95   | CPU   | memory    |
| ----- | ----- | --------- | --------- | --------- | --------- | ----- | --------- |
| 1     | 1K    | 175 ms    | 450 ms    | 175 ms    | 450 ms    | 1%    | ~100 MB   |
| 2     | 2K    | 380 ms    | 500 ms    | 175 ms    | 450 ms    | 1%    | ~150 MB   |
| 3     | 4K    | 380 ms    | 500 ms    | 350 ms    | 500 ms    | 1%    | ~150 MB   |
| 4     | 8K    | 175 ms    | 450 ms    | 175 ms    | 450 ms    | 1%    | ~190 MB   |
| 5     | 16K   | 200 ms    | 450 ms    | 150 ms    | 450 ms    | 1%    | ~375 MB   |
| 6     | 32K   | 200 ms    | 450 ms    | 100 ms    | 450 ms    | 1%    | ~500 MB   |
| 7     | 64K   | 125 ms    | 400 ms    | 75 ms     | 400 ms    | 2%    | ~1 GB     |
| 8     | 128K  | 80 ms     | 400 ms    | 75 ms     | 400 ms    | 4%    | ~1.75 GB  |

A few results from these trials stand out:
- librarians appear to be memory (rather than CPU) bound
- Get & Put p95s are surprisingly stable over 7 doublings in UPD
- Get & Put p50s are also pretty stable, even (suprisingly) decreasing a bit in the higher UPD

While not shown in the table above, the error rate across all endpoints in all trials was 0%, so 
the availability is also very good so far. While the stability of the Get & Put latencies is 
reassuring, the (almost) linear growth in memory usage with UPD is a little concerning and merits 
future investigation. 

These trials also reveal a few other (minor) shortcomings/bugs:
- pod CPU dashboard incorrectly assumes CPU units as [0, 100] instead of [0, 1].
- client balancer has same seed for every author, causing all authors to balance between librarians
in the same order and thus not properly balancing load across all librarians

In addition to these minor issue, we also realize the following (relatively minor) features would
facilitate further debugging and measurement
- adding a profiling `/debug/pprof/` endpoint to the librarian  
- aggregate queries per second (QPS) across all librarian endpoints


#### Memory performance at high UDP

After the initial trials showing experimentor and librarian memory usage growing linearly over time,
we investigated some aspects of this memory usage at high UDP (128K & 256K).

In **trial 9**, we used the profiling endpoint added in [libri #157](https://github.com/drausin/libri/pull/157)
to get heap dumps from actively running librarians as their memory usage increased. We also tried
decreasing the log level to `ERROR` to reduce possible memory pressure due to logging.

We got the heap dump from a single librarian via something like
```
kubectl exec librarians-1 -- wget -O - http://localhost:20300/debug/pprof/heap > librarians-1.heap.prof
go tool pprof librarians-1.heap.prof
```

Suprisingly, the active heap was much less (~100-200 MB) than what was reported via Prometheus 
metrics (which come from cadvisor via Kubernetes). After some investigation, we realized that the
`container_memory_usage_bytes` cadvisor Prometheus metric measures both working set memory as well
as page cache, whereas the heap dump excludes page cache (and also memory managed in RocksDB C 
library). We also found that changing the log level to `ERROR`has no effect on the reported
(by Prometheus) memory usage, indicating that logging doesn't seem to contribute significantly to 
page cache.

In **trial 10**, we bumped the UDP up to 256K and monitored the latencies and memory usage. For most of 
the experiment duration, we the cluster had reasonable Put & Get latencies (~500ms & 400ms for p95, 
respectively), but at around 13:35, they started running out of page cache (see 
[this Prometheus screenshot](trial10/img/Pod.MemoryPageCache.png)), and the latency performance 
decreases markedly, especially clearly in the p50s. 

At the end of the experiment, we realized that we still had the libri-experimenter CPU pod limit 
set to 100m (i.e., 10% of node CPU), which was likely throttling the actual queries it could emit.

In **trial 11**, we bumped the UDP to 512K and along with the experimenter limit up to 300m and 
librarian limits up to 200m and 3GB RAM. We saw, for the first time, significant performance 
degradation, with multi-second p50 & p95s as well as a few librarian crashes and the first 
observation of `Store` query errors from one of the librarians.   

Given the importance of the page cache shown in trial 10 and the performance issues in trial 11, we
were left thinking that the performance issues are likely due to our completely-untuned RocksDB 
usage. The next set of experiments should focused on this area via the following:
- moving the libri author library to avoid RocksDB altogether for its uploads and downloads (since 
it's used as scratch storage anyway) and use an in-memory replacement
- enable profiling on the experimenter to confirm that its memory usage isn't coming from anywhere 
else (other than RocksDB)
- move librarians from standard spinning to network-attached SSDs (which RocksDB is primarily built 
for) to (perhaps ?) decrease write bottleneck from page cache to disk
- tune some RocksDB options, including
    - background thread parallelism (1 -> 2)
    - optimizing for point lookups (with bloom filters and a block cache of 1GB)
    - (monitoring only) increasing stats dump to every 10 mins
     
In **trial 12**, the experimenter authors used in-memory docSLDs (c.f., 
[libri #159](https://github.com/drausin/libri/pull/159)) with the profiler enabled (c.f., 
[libri-experiments #8](https://github.com/drausin/libri-experiments/pull/8)), and we were able to 
see as expected that the experimenter memory usage dropped to be (on average) below that of the 
librarians. Investigating the experimentor heap dump also shows that the majority of the its 
memory usage comes from simply serializing the bytes to the wire. 
```
$ go tool pprof -inuse_space libri-experimenter.heap.prof
...
(pprof) top5
Showing nodes accounting for 653.44MB, 92.96% of 702.91MB total
Dropped 120 nodes (cum <= 3.51MB)
Showing top 5 nodes out of 59
      flat  flat%   sum%        cum   cum%
  542.39MB 77.16% 77.16%   542.39MB 77.16%  github.com/drausin/libri-experiments/vendor/github.com/golang/protobuf/proto.(*Buffer).EncodeRawBytes /go/src/github.com/drausin/libri-experiments/vendor/github.com/golang/protobuf/proto/encode.go
   44.87MB  6.38% 83.55%    44.87MB  6.38%  bufio.NewWriterSize /usr/local/go/src/bufio/bufio.go
   41.78MB  5.94% 89.49%    41.78MB  5.94%  bufio.NewReaderSize /usr/local/go/src/bufio/bufio.go
   13.20MB  1.88% 91.37%    13.20MB  1.88%  github.com/drausin/libri-experiments/vendor/golang.org/x/net/http2.NewFramer.func1 /go/src/github.com/drausin/libri-experiments/vendor/golang.org/x/net/http2/frame.go
   11.19MB  1.59% 92.96%    11.19MB  1.59%  github.com/drausin/libri-experiments/vendor/golang.org/x/net/http2.(*Framer).WriteDataPadded /go/src/github.com/drausin/libri-experiments/vendor/golang.org/x/net/http2/frame.go
(pprof)
```
As in trial 10, performance started degrading once the librarians started running out of page cache.

In **trial 13**, we tried replacing the librarian spinning disks with SSDs (since RocksDB is optimized 
for those), thinking perhaps that a disk throughput bottleneck was causing the librarians to use 
more page cache than they should. This change ended up having little effect on the overall librarian
memory usage, though perhaps the p50s were a bit better.

In **trial 14**, we tried tweaking some of the RocksDB options (c.f., 
[libri #160](https://github.com/drausin/libri/pull/160)), in particular 
- 1 GB block cache
- 32 K block size
- 10-bit Bloom filters on tables
- 500 MB memory table

thinking that the larger block cache and bloom filters especially would relieve librarian memory 
pressure. These tweaks did improve p95 and p50 latencies compared to trial 13 as follows
- Get p95: 250-1000ms -> 250ms
- Get p50: 30-80ms -> 20-30ms
- Put p95: 400ms -> 150ms
- Put p50: 100-200ms -> 40-60ms
We omit the severe performance degradations caused by page cache exhaustion at the end when the 
librarians ran out of their 3GB memory budget. We were glad to see these improvements, but the 
continued existence of the underlying memory usage issue indicates that RocksDB probably isn't the 
main culprit. The other main culprit is probably grpc connections, so subsequent trials looked 
into
- setting `MaxConcurrentStreams` on the grpc server (as opposed to the unbounded default)
- try just having a single author (with 100x more requests per day), since maybe the problem is just
too many client connections to each librarian

In **trial 15**, we set `MaxConcurrentStreams = 128` (c.f., 
[libri #162](https://github.com/drausin/libri/pull/162)) and noticed that memory usage across
librarians was more consistent, but it didn't really change the cumulative memory usage of either
the experimenter or librarians.

In **trial 16**, we used moved from 100 -> 1 authors but from 2560 -> 256000 uploads per author. This
change completely solved the experimenter memory issue, with it now using ~50 MB RAM instead of 
previously it's consuming more and more until it reached the limit.

In **trial 17**, we bumped the librarian RAM limit up to 5 GB and ran the experimenter for 90 minutes, 
thinking that possibly the librarians just had steady-state memory usage higher than the 3 GB used
previously. Unfortunately, the linear memory growth still occurred, and the librarians eventually
hit the new memory limit.

In **trial 18**, we tested [libri #163](https://github.com/drausin/libri/pull/163), which updated the
librarian peer connection handling to actively disconnect one of the connections when merging two
peers that both have a connection. Our hypothesis was that the librarian servers were leaking
connections (i.e., creating redundant ones and not cleaning up existing ones) when peers were 
being added to the routing table. Unfortunately, the change didn't have much/any effect on the
librarian memory usage over the course of the experiment.

Trial 19 was essentially the same as trial 16 but with profiling enabled for the librarians so we
could look at goroutine and heap dumps. These confirmed our suspicians that an ever-increasing 
number of goroutines were being dedicated to grpc connection handling. These results convinced us
that managing the librarian connections on the `Peer` object is not a manageable approach.   

Trial 20 used the new connection pooling implemented in 
[libri #164](https://github.com/drausin/libri/pull/164), and we finally found librarian memory 
usage under control. Profiling a librarian a few times over the course of the experiment shows only 
8x goroutines for each of the 8 different types of goroutines run by a grpc connection. (See 
[librarians-0.goroutine.prof](trial20/librarians-0.goroutine.prof).)

#### High UDP performance tuning

With the librarian memory usage under control, we doubled our load up to 512K UDP in trial 21. A few
of the librarians had noticeably worse p95 latencies than the others, and closer inspection revealed
that they were receiving up to 4x more Store queries than some of the other librarians. We also
observed periodic RocksDB file operations (e.g., flush & compaction) to have a non-trivial effect 
on the p95 latencies when they happen.

In trial 22, we tested [libri #165](https://github.com/drausin/libri/pull/165), which fixes a bug
we found upon digging into the ordering (or lack thereof) of peers when being queried from a
librarians's routing table. This change reduced Store request differential from ~4x down to ~2x, but
it also reduced the latencies by at least 50%.

In trial 23, we tested [libri #166](https://github.com/drausin/libri/pull/166), which adjusts the 
peer ordering in a routing table bucket to favor peers less frequently queried when they have been 
queried almost as recently as more frequently queried peers. The intent was to more explicitly 
balance the queries out across peers. The change had the intended effect, with Store requests more 
evenly spread out across all librarians (though not perfectly, with one librarian receiving 
noticeably fewer requests). Latencies across all endpoints also appeared to be a bit better. 
Unfortunately, we see how RocksDB file operations disrupt especially Put and Store p95 latencies.

In trial 24, we tested reducing the RocksDB memtable size to 256 MB
([commit](https://github.com/drausin/libri/commit/774be6b6e48fac514df9f27c76f2354da5264828)), 
thinking that it might replace bursty write operations with more regular (and smaller) ones, 
reducing the impact of the worst of the writes on the p95 latencies. Unfortunately, this change
just seemed to slightly reduce overall latency performance.

In trial 25, we tested returning to a 512 MB memtable while keeping the write buffer size
the same (64 MB) ([commit](https://github.com/drausin/libri/commit/07a2a97bba1c67f19a680ef2fb429834e217423f)), 
thinking as in trial 24 that splitting the writes across more files would distribute the write 
load more evenly over time. This change doesn't seem to have made much difference in the latency 
performance, and we also noticed that three of the librarians (0, 4, 6) generally had slightly worse
latencies than the other nodes and also all happened to be on the same node as the Grafana pod. It's
possible that bursty CPU usage by Grafana (on regular page/metric refreshes) takes away from the 
librarians when they have high CPU needs (e.g., during RocksDB file writes). To avoid this possible
contamination, we generally avoided having Grafana dashboards open when running future experiments.

In trial 26, we doubled the load again to 1024K UDP and increased the experiment runtime to 60 
minutes, meeting our target load of 1M+ UDP. We increased the librarian CPU limit from 250m to 400m 
and memory limit from 3GB to 4GB. We also had to add "warm up" period of 30s to the experimenter 
([libri-experiments #11](https://github.com/drausin/libri-experiments/pull/11)), otherwise 
sometimes a librarian would get overwhelmed by the immediate volume of Store requests. Surprisingly, 
the latencies all had only modest increases except the Put p95, which jumped and stayed above 500ms 
once the RocksDB memtable started getting written to disk.

In trial 27, we trial switching from 4x `n1-highmem-2` nodes to 2x `n1-highmem-4`, thinking that
the 4 virtual CPUs available to each pod might help with some of the bursty CPU-intensive RocksDB 
operations, taking better advantage of the 4x threads configured for RocksDB. Unfortunately, almost 
every latency metric got worse over the course of the experiment. Perhaps this is due to more CPU
contention between the pods on the same host, but that seems insufficient to explain the difference
in the results from trial 26.

In trial 28, we reverted back to 4x `n1-highmem-2` nodes and the previous RocksDB configuration from 
trial 23. These latencies were the best so far for 1024K UDP, though Put p95 still spiked up to 
2000ms for a few minutes about 30 mins into the experiment. 

In trial 29, we decided to take one more stab 
([commit](https://github.com/drausin/libri/commit/f0f2b1ee60ba86a2621117c847c6b19327724175))
at some RocksDB parameter optimization from the "Total ordered database, flash storage" section of 
the [RocksDB Tuning Guide](https://github.com/facebook/rocksdb/wiki/RocksDB-Tuning-Guide). While
this change did succeed in lowering the peak of Put p95 from ~2000ms down to ~1750ms, it generally
degraded the other latencies.   

At this point, we decided to stop further optimization attempts and draw this experiment to a close.
Our goal at the outset of this experiment was to have Put & Get p95s below 1000ms and 99.99%+ 
availability at 1M UDP. Over the 60 minutes at 1024K UDP, the best latencies (from trial 28) are 
roughly
- Get p95: 100-150ms
- Put p95: 400-800ms w/ 10 (or 60) total mins of "burst" above 1000ms
- Get p50: 6-10ms
- Put p50: 75-125ms
and availability was 100%.

### Discussion

Other than the Put p95, these trial 28 latencies are signficantly better than where we started with 
1K UDP. Some of this improvement is likely just the benefit of a larger sample size at 1024K UDP 
vs. 1K, since the 95th percentile of a much smaller sample of points often becomes something close 
to the max of that sample. But over the course of these trials, we made a number of improvements 
that led us to these final numbers:
- randomizing client balancer seed
- basic RocksDB tuning 
- using a single author connection instead of many to query librarians
- librarian connection pooling
- better peer ordering for Store queries and within the routing table buckets

Future experiments and optimization will focus on 
- longer (6, 12, 24 hr) experiments
- clusters with more librarians
- impact of maintenance events like librarian restarts
- further RocksDB tuning
- larger distribution of document sizes (up to 32 MB)

But for the time being, libri performance appears sufficiently good to handle the modest 
initial load expected in the first 12 months of live deployment.   