# author sim
libri_exp_version = "snapshot-6adf9dd"
duration = "30m"

# translates to 1000 UPD or ~0.09 QPS
num_authors = 100
docs_per_day = 2560
shares_per_upload = 2
num_uploaders = 32
num_downloaders = 96

# this shape and rate imply a mean of ~256 KB and a 95% CI of [~18, ~794] KB.
content_size_kb_gamma_shape = 1.5
content_size_kb_gamma_rate = 0.00588

cluster_host = "gcp"

# librarians
num_librarians = 8
librarian_libri_version = "snapshot-4089638"
librarian_disk_size_gb = 10
librarian_cpu_limit = "200m"
librarian_ram_limit = "2G"

librarian_public_port_start = 30100
librarian_local_port = 20100
librarian_local_metrics_port = 20200

# monitoring
grafana_port = 30300
prometheus_port = 30090

# Kubernetes cluster
num_cluster_nodes = 2
cluster_node_machine_type = "n1-highmem-2"  # 2 CPUs, 6.5 GB RAM
