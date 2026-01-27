variable "project_id" {}

variable "image_apis_account_id" {}

variable "image_compressor_account_id" {}

variable "image_apis_roles" {
  type = list(string)
}

variable "image_compressor_roles" {
  type = list(string)
}
