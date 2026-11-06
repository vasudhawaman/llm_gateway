# Untested starter: Cloud Run service for the gateway image.
# Usage: terraform init && terraform apply -var project=YOUR_PROJECT -var image=REGION-docker.pkg.dev/YOUR_PROJECT/repo/llm-gateway:latest
variable "project" { type = string }
variable "image"   { type = string }
variable "region"  {
  type    = string
  default = "asia-south1"
}

provider "google" {
  project = var.project
  region  = var.region
}

resource "google_cloud_run_v2_service" "gateway" {
  name     = "llm-gateway"
  location = var.region
  template {
    scaling {
      min_instance_count = 0
      max_instance_count = 2
    }
    containers {
      image = var.image
      resources { limits = { cpu = "1", memory = "256Mi" } }
    }
  }
}

resource "google_cloud_run_v2_service_iam_member" "public" {
  name     = google_cloud_run_v2_service.gateway.name
  location = var.region
  role     = "roles/run.invoker"
  member   = "allUsers"
}

output "url" { value = google_cloud_run_v2_service.gateway.uri }
