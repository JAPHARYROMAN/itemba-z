output "deployment_contract" {
  description = "Provider-neutral deployment inputs consumed by the selected cloud module."
  value = {
    environment    = var.environment
    primary_region = var.primary_region
    tags           = local.required_tags
    safety         = var.deployment_safety
  }
}

output "security_custody_contract" {
  description = "Non-secret custody identifiers consumed by the selected production provider module."
  value = {
    secret_custody           = var.secret_custody
    runtime_secret_names     = sort(keys(var.runtime_secret_references))
    runtime_secret_reference = "injected-by-managed-workload-identity"
  }
}
