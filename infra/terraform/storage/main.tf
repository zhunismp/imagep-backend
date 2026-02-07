resource "google_storage_bucket" "imagep-bucket" {
    name = var.bucket_name
    location = var.location
    project = var.project_id

    storage_class = "STANDARD"

    uniform_bucket_level_access = true
    public_access_prevention = "enforced"

    versioning {
        enabled = true
    }

    lifecycle_rule {
        condition {
            age = 1
        }

        action {
            type = "Delete"
        }
    }

    cors {
        origin          = ["https://imagep.zhunismp.dev"]
        method          = ["GET", "HEAD"]
        response_header = ["Content-Type"]
        max_age_seconds = 3600
    }
}