locals {
  required_tags = merge(
    {
      application = var.service_name
      environment = var.environment
      managed_by  = "terraform"
      data_class  = "restricted"
    },
    var.tags
  )
}
