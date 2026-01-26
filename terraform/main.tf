locals {
  project_name = "imagep"
}

module "gsa" {
  source       = "./modules/gsa"
  project_id   = var.project_id

  image_apis_account_id = "${local.project_name}-image-apis-gsa"
  image_apis_roles = [
    "roles/storage.objectAdmin",
    "roles/secretmanager.secretAccessor"
  ]

  image_compressor_account_id = "${local.project_name}-image-processor-gsa"
  image_compressor_roles = [
    "roles/storage.objectAdmin",
    "roles/secretmanager.secretAccessor"
  ]
}

module "gke" {
  source     = "./modules/gke"
  project_id = var.project_id
  zone       = var.zone
  name       = "${local.project_name}-gke"
}

