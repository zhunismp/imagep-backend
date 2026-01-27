locals {
  project_name = "imagep"
}

module "imagep_role" {
  source       = "./role"
  project_id   = var.project_id

  image_apis_account_id = "${local.project_name}-image-apis-gsa"
  image_apis_roles = [
    "roles/storage.objectAdmin",
    "roles/secretmanager.secretAccessor"
  ]

  image_compressor_account_id = "${local.project_name}-image-compressor-gsa"
  image_compressor_roles = [
    "roles/storage.objectAdmin",
    "roles/secretmanager.secretAccessor",
    "roles/iam.serviceAccountTokenCreator"
  ]
}

module "imagep_storage" {
  source = "./storage"
  project_id = var.project_id
  location = var.region
  bucket_name = "process-image" # TODO: remove hardcoded
}

module "imagep_cluster" {
  source     = "./cluster"
  project_id = var.project_id
  zone       = var.zone
  name       = "${local.project_name}-gke"
}

