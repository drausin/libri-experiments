## exp01 - How does performance degrade with increasing user load?

What happens to libri network performance as user load increases? Network performance is measured
primarily by metrics on the aggregate librarians:
- CPU usage
- memory usage
- Put/Get response times (p50, p95)
- availability %

### Methods

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


### Results

The results of the first 8 trials are given below.

| trial | UPD   | Get p50   | Get p95   | Put p50   | Put p95   | CPU   | memory    |
-------------------------------------------------------------------------------------
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
