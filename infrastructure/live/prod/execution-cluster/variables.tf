variable "environment" {
  type = string
}

variable "aws_region" {
  type    = string
  default = "eu-north-1"
}

variable "cluster_name" {
  type = string
}

variable "vpc_cidr" {
  type = string
}

variable "azs" {
  type = list(string)
}

variable "single_nat_gateway" {
  type = bool
}

variable "deletion_protection" {
  type = bool
}

variable "control_plane_log_retention_days" {
  type = number
}

variable "vpc_flow_log_retention_days" {
  type = number
}

variable "cluster_admin_role_arn" {
  type      = string
  sensitive = false
}

variable "break_glass_role_arn" {
  type      = string
  default   = null
  nullable  = true
  sensitive = false
}

variable "system_min_size" {
  type = number
}

variable "system_desired_size" {
  type = number
}

variable "system_max_size" {
  type = number
}

variable "runtime_min_size" {
  type = number
}

variable "runtime_desired_size" {
  type = number
}

variable "runtime_max_size" {
  type = number
}

variable "sandbox_min_size" {
  type = number
}

variable "sandbox_desired_size" {
  type = number
}

variable "sandbox_max_size" {
  type = number
}

variable "sandbox_capacity_type" {
  type = string
}

variable "addon_versions" {
  type = object({
    coredns            = optional(string)
    kube_proxy         = optional(string)
    vpc_cni            = optional(string)
    pod_identity_agent = optional(string)
  })

  default = {}
}
