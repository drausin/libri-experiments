cluster_host = "gcp"
cluster_admin_user = "experimenter@libri-170711.iam.gserviceaccount.com"

# author sim
libri_exp_version = "snapshot-b5bda27"
duration = "60m"

num_authors = 1
docs_per_day = 64000  # <- independent variable 1
shares_per_upload = 2
num_uploaders = 256
num_downloaders = 256

# this shape and rate imply a mean of ~256 KB and a 95% CI of [~18, ~794] KB.
content_size_kb_gamma_shape = 1.5
content_size_kb_gamma_rate = 0.00588


# librarians
num_librarians = 64  # <- independent variable 2
librarian_libri_version = "snapshot-fa7e6f2"
librarian_disk_size_gb = 10
librarian_disk_type = "pd-ssd"
librarian_cpu_limit = "150m"
librarian_ram_limit = "2G"

librarian_public_port_start = 30100
librarian_local_port = 20100
librarian_local_metrics_port = 20200

# monitoring
grafana_port = 30300
prometheus_port = 30090
grafana_ram_limit = "250M"
prometheus_ram_limit = "1G"
grafana_cpu_limit = "100m"
prometheus_cpu_limit = "250m"

# Kubernetes cluster
num_cluster_nodes = 6
cluster_node_machine_type = "n1-highmem-4"  # 4 CPUs, 26 GB RAM
