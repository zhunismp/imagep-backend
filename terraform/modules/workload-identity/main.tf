resource "kubernetes_service_account" "this" {
  metadata {
    name      = var.ksa_name
    namespace = var.namespace

    annotations = {
      "iam.gke.io/gcp-service-account" = var.gsa_email
    }
  }
}

resource "google_service_account_iam_member" "binding" {
  service_account_id = var.gsa_name
  role               = "roles/iam.workloadIdentityUser"

  member = "serviceAccount:${var.workload_pool}[${var.namespace}/${var.ksa_name}]"
}