# resource "google_service_account_iam_member" "kafka_workload_identity" {
#   service_account_id = var.gsa_name
#   role               = "roles/iam.workloadIdentityUser"

#   member = "serviceAccount:${var.workload_pool}[kafka/${var.ksa_name}]"
# }