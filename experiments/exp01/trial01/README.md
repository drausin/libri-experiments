## exp01a: How does performance degrade over 100, 1K, 10K daily active users?

#### methods

**fixed parameters**
- shares per doc upload: 2
- doc_size_mean: 250KB
- doc_size_var: ??
- doc_size_dist: Gammma
- num_librarians: 8
- librarian_libri_version: 0.2.0
- librarian_cpu_limit: 100m
- librarian_ram_limit: 2GB
- librarian_disk_size_gb: 25
- librarian_disk_type: standard
- num_cluster_nodes: 2
- cluster_node_machine_type: n1-highmem-2

**indepdent variables**
- num_authors: [100, 1000, 10000]
- duration: [3h, 12h, 24h]

**dependent variables**
- Put/Get p50, p95
- availability %
- librarian mem usage
- librarian CPU usage


