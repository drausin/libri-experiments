## exp01 - How does performance degrade with increasing user load?

What happens to libri network performance as user load increases? Network performance is measured
primarily by metrics on the aggregate librarians:
- CPU usage
- memory usage
- Put/Get response times (p50, p95)
- availability %

### Methods & Results

#### Naive user load increases

The main independent variable for now is the number of uploads per day (UPD), which is just the 
number of authors times the assumed number of documents each author uploads per day. We keep 
number of authors fixed at 100 to avoid practical issues like too many open RocksDB files, but
the "real" assumption is that UPL is a rough proxy for number of authors (i.e., users), since we 
don't expect each author to upload more that one document per day (at least for now).  

Each upload translates to 8 Put/Get queries:
- Put entry (always will just be a single-page entry b/c content distribution size is always < 2 MB)
- Put envelope (self reader)
- 2x Put envelope (shared authors)
- 2x Get envelope (shared authors)
- 2x Get entry (shared authors)

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

In trial 9, we used the profiling endpoint added in []libri #157](https://github.com/drausin/libri/pull/157)
to get heap dumps from actively running librarians as their memory usage increased. We also tried
increasing the log level to `ERROR` to reduce possible memory pressure due to logging.

We got the heap dump from a single librarian via something like
```
kubectl exec librarians-1 -- wget -O - http://localhost:20300/debug/pprof/heap > librarians-1.heap.prof
go tool pprof librarians-1.heap.prof
```

Suprisingly, the active heap was much less (~100-200 MB) than what was reported via Prometheus 
metrics (which come from cadvisor via Kubernetes). After some investigation, we realized that the
`container_memory_usage_bytes` cadvisor Prometheus metric measures both working set memory as well
as page cache, whereas the heap dump excludes page cache (and possibly also memory managed in 
RocksDB C library). We also found that change the log level to `ERROR`has no effect on the reported
(by Prometheus) memory usage, indicating that logging doesn't seem to contribute significantly to 
page cache.

In trial 10, we bumped the UDP up to 256K and monitored the latencies and memory usage. For most of 
the experiment duration, we the cluster had reasonable Put & Get latencies (~500ms & 400ms for p95, 
respectively), but at around 13:35, they started running out of page cache (see 
[this Prometheus screenshot](trial10/img/Pod.MemoryPageCache.png)), and the latency performance 
decreases markedly, especially clearly in the p50s. 

At the end of the experiment, we realized that we still had the libri-experimenter CPU pod limit 
set to 100m (i.e., 10% of node CPU), which was likely throttling the actual queries it could emit.

In trial 11, we bumped the experimenter limit up to 300m and librarian limits up to 200m and 3GB 
RAM. We saw, for the first time, significant performance degradation, with multi-second p50 & p95s 
as well as a few librarian crashes and the first observation of `Store` query errors from one of 
the librarians.   

Given the importance of the page cache shown in trial 10 and the performance issues in trial 11, we
were left thinking that the performance issues are likely due to our completely-untuned RocksDB 
usage. The next set of experiments should focus on this area via the following:
- moving the libri author library to avoid RocksDB altogether for its uploads and downloads (since 
it's used as scratch storage anyway) and use an in-memory replacement
- enable profiling on the experimenter to confirm that its memory usage isn't coming from anywhere 
else (other than RocksDB)
- move librarians from standard spinning to network-attached SSDs (which RocksDB is primarily built 
for) to (perhaps ?) decrease write bottleneck from page cache to disk
- tune some RocksDB options, including
    - background thread parallelism (1 -> 4)
    - optimizing for point lookups (with bloom filters and a block cache of 1GB)
    - (monitoring only) increasing stats dump to every 10 mins
     
    
