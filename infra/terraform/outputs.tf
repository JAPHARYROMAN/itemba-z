output "deployment_contract" {
  description = "Provider-neutral deployment inputs consumed by the selected cloud module."
  value = {
    environment    = var.environment
    primary_region = var.primary_region
    tags           = local.required_tags
  }
}
