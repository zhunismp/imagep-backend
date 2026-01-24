resource "google_container_cluster" "this" {
  name     = var.name
  location = var.zone

  remove_default_node_pool = true
  initial_node_count       = 2

  workload_identity_config {
    workload_pool = "${var.project_id}.svc.id.goog"
  }

  deletion_protection = false
}

resource "google_service_account" "gke_nodes" {
    account_id = "gke-nodes"
    display_name = "gke-nodes"
}

resource "google_project_iam_member" "node_default_role" {
    project = var.project_id
    role = "roles/container.defaultNodeServiceAccount"
    member = "serviceAccount:${google_service_account.gke_nodes.email}"
}

resource "google_container_node_pool" "default" {
  name       = "default-pool"
  cluster    = google_container_cluster.this.name
  location   = var.zone
  node_count = var.node_count

  node_config {
    disk_size_gb = 20
    disk_type = "pd-balanced"
    machine_type = var.machine_type
    oauth_scopes = [
      "https://www.googleapis.com/auth/cloud-platform"
    ]
    service_account = google_service_account.gke_nodes.email
  }
}