variable "environment" {
  description = "Deployment environment name."
  type        = string

  validation {
    condition     = contains(["development", "staging", "production"], var.environment)
    error_message = "environment must be development, staging, or production."
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
