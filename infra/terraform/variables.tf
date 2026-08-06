variable "environment" {
  description = "Deployment environment name."
  type        = string

  validation {
    condition     = contains(["configuration", "test", "staging", "pilot", "production"], var.environment)
    error_message = "environment must be configuration, test, staging, pilot, or production. Local development is not a deployed environment."
  }
}

variable "primary_region" {
  description = "Approved primary region selected after Tanzania data-transfer review."
  type        = string
}

variable "service_name" {
  description = "Stable service namespace."
  type        = string
  default     = "itemba-z"
}

variable "tags" {
  description = "Additional ownership and cost-allocation tags."
  type        = map(string)
  default     = {}
}

variable "secret_custody" {
  description = "Approved provider-neutral references for secret, encryption-key, certificate, audit-log, and emergency-access custody. Values are identifiers, never secret material."
  type = object({
    provider                 = string
    vault_id                 = string
    encryption_key_id        = string
    certificate_manager_id   = string
    audit_log_destination_id = string
    rotation_owner           = string
    emergency_access_group   = string
  })

  validation {
    condition = alltrue([
      for value in values(var.secret_custody) : length(trimspace(value)) >= 3
    ])
    error_message = "Every secret-custody identifier and owner must be explicitly configured. Secret values must not be supplied to Terraform variables."
  }
}

variable "runtime_secret_references" {
  description = "Secret-manager reference URIs injected into runtime workloads; values must be references, not plaintext credentials."
  type        = map(string)

  validation {
    condition = length(var.runtime_secret_references) >= 5 && alltrue([
      for name, reference in var.runtime_secret_references :
      can(regex("^[a-z][a-z0-9+.-]*://", reference)) &&
      !can(regex("(?i)(password|secret|token|key)=", reference)) &&
      length(trimspace(name)) >= 3
    ])
    error_message = "At least five named runtime secrets must use provider reference URIs and must not embed credential values."
  }
}

variable "deployment_safety" {
  description = "Fail-closed platform controls that every selected provider module must implement."
  type = object({
    managed_tls                        = bool
    private_networking                 = bool
    managed_postgresql                 = bool
    encrypted_object_storage           = bool
    redis_is_non_authoritative         = bool
    least_privilege_runtime_identities = bool
    point_in_time_recovery             = bool
    immutable_backup_copy              = bool
    development_seed_disabled          = bool
    header_identity_disabled           = bool
    unsafe_debug_disabled              = bool
  })

  validation {
    condition     = alltrue(values(var.deployment_safety))
    error_message = "Every deployment safety control must be true; provider modules must fail closed."
  }
}
