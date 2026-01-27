output "endpoint" {
  value = google_container_cluster.this.endpoint
}

output "ca_certificate" {
  value = google_container_cluster.this.master_auth[0].cluster_ca_certificate
}

output "workload_pool" {
  value = "${var.project_id}.svc.id.goog"
}
