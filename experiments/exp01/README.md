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

Each upload translates to 8 queries:
- Put entry (always will just be a single-page entry b/c content distribution size is always < 2 MB)
- Put envelope (self reader)
- 2x Put envelope (shared authors)
- 2x Get envelope (shared authors)
- 2x Get entry (shared authors)

We can approximate the queries per second (QPS) for a given number of UPD via 

    1000 uploads/day * 8 queries/upload / (24 hrs * 3600 secs/hr) = ~0.09 QPS 


#### Results

- trial 01
    - 1000 UPD (~0.09 QPS)
