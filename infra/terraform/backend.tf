terraform {
  backend "gcs" {
    bucket  = "imagep-terraform-state"
    prefix  = "gke/workload-identity"
  }
}