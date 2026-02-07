variable "project_id" {}

variable "zone" {}

variable "name" {}

variable "node_count" {
  default = 3
}

variable "machine_type" {
  default = "e2-highmem-2"
}