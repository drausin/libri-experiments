# author sim
libri_exp_version = "snapshot-949fb64"
duration = "90m"

num_authors = 1
docs_per_day = 256000
shares_per_upload = 2
num_uploaders = 64
num_downloaders = 192

# this shape and rate imply a mean of ~256 KB and a 95% CI of [~18, ~794] KB.
content_size_kb_gamma_shape = 1.5
content_size_kb_gamma_rate = 0.00588

cluster_host = "gcp"

# librarians
num_librarians = 8
librarian_libri_version = "snapshot-2a6e75e"
librarian_disk_size_gb = 10
librarian_disk_type = "pd-ssd"
librarian_cpu_limit = "250m"
librarian_ram_limit = "5G"

librarian_public_port_start = 30100
librarian_local_port = 20100
librarian_local_metrics_port = 20200

# monitoring
grafana_port = 30300
prometheus_port = 30090

# Kubernetes cluster
num_cluster_nodes = 4
cluster_node_machine_type = "n1-highmem-2"  # 2 CPUs, 13 GB RAM each
