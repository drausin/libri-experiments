# author sim
libri_exp_version = "snapshot-bb975b8"
duration = "1h"

num_authors = 1
docs_per_day = 256000
shares_per_upload = 2
num_uploaders = 64
num_downloaders = 64

# this shape and rate imply a mean of ~256 KB and a 95% CI of [~18, ~794] KB.
content_size_kb_gamma_shape = 1.5
content_size_kb_gamma_rate = 0.00588

cluster_host = "gcp"
cluster_admin_user = "experimenter@libri-170711.iam.gserviceaccount.com"

# librarians
num_librarians = 32
librarian_libri_version = "snapshot-2cd68f0"
librarian_disk_size_gb = 10
librarian_disk_type = "pd-ssd"
librarian_cpu_limit = "200m"
librarian_ram_limit = "3G"

librarian_public_port_start = 30100
librarian_local_port = 20100
librarian_local_metrics_port = 20200

# monitoring
grafana_port = 30300
prometheus_port = 30090

# Kubernetes cluster
num_cluster_nodes = 6
cluster_node_machine_type = "n1-highmem-4"  # 4 CPUs & 26 GB RAM each
