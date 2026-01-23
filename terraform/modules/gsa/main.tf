# image-apis gsa
resource "google_service_account" "image_apis_gsa" {
  account_id   = var.image_apis_account_id
  display_name = "image-apis-gsa"
}

resource "google_project_iam_member" "image_apis_roles" {
  for_each = toset(var.image_apis_roles)

  project = var.project_id
  role    = each.value
  member  = "serviceAccount:${google_service_account.image_apis_gsa.email}"
}

resource "google_service_account_iam_member" "image_apis_wi" {
  service_account_id = google_service_account.image_apis_gsa.name
  role               = "roles/iam.workloadIdentityUser"
  member             = "serviceAccount:${var.project_id}.svc.id.goog[imageapis/sa]" # hard-coded namespace eh?
}

# image-compressor gsa
resource "google_service_account" "image_compressor_gsa" {
  account_id   = var.image_compressor_account_id
  display_name = "image-compressor-gsa"
}

resource "google_project_iam_member" "image_compressor_roles" {
  for_each = toset(var.image_compressor_roles)

  project = var.project_id
  role    = each.value
  member  = "serviceAccount:${google_service_account.image_compressor_gsa.email}"
}

resource "google_service_account_iam_member" "image_compressor_wi" {
  service_account_id = google_service_account.image_compressor_gsa.name
  role               = "roles/iam.workloadIdentityUser"
  member             = "serviceAccount:${var.project_id}.svc.id.goog[imagecompressor/sa]" # hard-coded namespace eh?
}

# external secret
resource "google_service_account" "eso_gsa" {
    account_id = "eso-gsa"
    display_name = "eso-gsa"
}

resource "google_project_iam_member" "eso_secret_access" {
  project = var.project_id
  role    = "roles/secretmanager.secretAccessor"
  member  = "serviceAccount:${google_service_account.eso_gsa.email}"
}

resource "google_service_account_iam_member" "eso_wi" {
  service_account_id = google_service_account.eso_gsa.name
  role               = "roles/iam.workloadIdentityUser"

  member = "serviceAccount:${var.project_id}.svc.id.goog[eso/sa]"
}
