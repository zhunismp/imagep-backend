locals {
  project_name = "imagep"
}

module "gsa" {
  source       = "./modules/gsa"
  project_id   = var.project_id

  image_apis_account_id = "${local.project_name}-image-apis-gsa"
  image_apis_roles = [
    "roles/storage.objectViewer",
    "roles/secretmanager.secretAccessor"
  ]

  image_compressor_account_id = "${local.project_name}-image-processor-gsa"
  image_compressor_roles = [
    "roles/storage.objectViewer",
    "roles/secretmanager.secretAccessor"
  ]
}

module "gke" {
  source     = "./modules/gke"
  project_id = var.project_id
  zone       = var.zone
  name       = "${local.project_name}-gke"
}

# module "workload_identity" {
#   source = "./modules/workload-identity"

#   namespace      = "${local.project_name}"
#   ksa_name       = "${local.project_name}-sa"
#   gsa_email      = module.gsa.email
#   gsa_name       = module.gsa.name
#   workload_pool  = module.gke.workload_pool
# }
