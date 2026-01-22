locals {
  project_name = "imagep"
}


module "gke" {
  source     = "./modules/gke"
  project_id = var.project_id
  zone     = var.zone
  name       = "${local.project_name}-gke"
}

module "gsa" {
  source       = "./modules/gsa"
  project_id   = var.project_id
  account_id   = "${local.project_name}-gsa"
  display_name = "imagep service account"

  roles = [
    "roles/storage.objectViewer",
    "roles/secretmanager.secretAccessor"
  ]
}

module "workload_identity" {
  source = "./modules/workload-identity"

  namespace      = "${local.project_name}"
  ksa_name       = "${local.project_name}-sa"
  gsa_email      = module.gsa.email
  gsa_name       = module.gsa.name
  workload_pool  = module.gke.workload_pool
}