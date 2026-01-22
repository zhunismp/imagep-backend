resource "google_container_cluster" "this" {
  name     = var.name
  location = var.zone

  remove_default_node_pool = true
  initial_node_count       = var.node_count

  workload_identity_config {
    workload_pool = "${var.project_id}.svc.id.goog"
  }
}

resource "google_container_node_pool" "default" {
  name       = "default-pool"
  cluster    = google_container_cluster.this.name
  location   = var.zone
  node_count = var.node_count

  node_config {
    machine_type = var.machine_type
    oauth_scopes = [
      "https://www.googleapis.com/auth/cloud-platform"
    ]
  }
}

resource "kubernetes_namespace" "imagep" {
  metadata {
    name = "imagep"
  }
}
