variable "name" {
  description = "Execution EKS cluster name."
  type        = string

  validation {
    condition     = length(var.name) >= 3 && length(var.name) <= 100
    error_message = "name must contain between 3 and 100 characters."
  }
}

variable "environment" {
  description = "Deployment environment."
  type        = string

  validation {
    condition = contains(
      ["dev", "staging", "prod"],
      var.environment,
    )
    error_message = "environment must be dev, staging, or prod."
  }
}

variable "region" {
  description = "AWS region."
  type        = string
}

variable "kubernetes_version" {
  description = "EKS Kubernetes minor version."
  type        = string
  default     = "1.36"
}

variable "vpc_cidr" {
  description = "CIDR for the execution-plane VPC."
  type        = string
}

variable "azs" {
  description = "Availability zones."
  type        = list(string)

  validation {
    condition     = length(var.azs) >= 2
    error_message = "At least two availability zones are required."
  }
}

variable "single_nat_gateway" {
  description = "Use one shared NAT gateway. Recommended only for development."
  type        = bool
  default     = false
}

variable "deletion_protection" {
  description = "Protect the EKS cluster from accidental deletion."
  type        = bool
  default     = true
}

variable "cluster_admin_role_arn" {
  description = "IAM role granted EKS cluster-admin access."
  type        = string

  validation {
    condition     = startswith(var.cluster_admin_role_arn, "arn:aws:iam::")
    error_message = "cluster_admin_role_arn must be an AWS IAM ARN."
  }
}

variable "break_glass_role_arn" {
  description = "Optional emergency cluster administrator."
  type        = string
  default     = null
  nullable    = true
}

variable "control_plane_log_retention_days" {
  description = "CloudWatch retention for EKS control-plane logs."
  type        = number
  default     = 90
}

variable "vpc_flow_log_retention_days" {
  description = "CloudWatch retention for VPC flow logs."
  type        = number
  default     = 90
}

variable "system_instance_types" {
  type    = list(string)
  default = ["m7i.large", "m6i.large"]
}

variable "system_min_size" {
  type    = number
  default = 2
}

variable "system_desired_size" {
  type    = number
  default = 3
}

variable "system_max_size" {
  type    = number
  default = 6
}

variable "runtime_instance_types" {
  type    = list(string)
  default = ["m7i.xlarge", "m6i.xlarge"]
}

variable "runtime_min_size" {
  type    = number
  default = 2
}

variable "runtime_desired_size" {
  type    = number
  default = 3
}

variable "runtime_max_size" {
  type    = number
  default = 20
}

variable "sandbox_instance_types" {
  type    = list(string)
  default = ["m7i.xlarge", "m6i.xlarge"]
}

variable "sandbox_min_size" {
  type    = number
  default = 1
}

variable "sandbox_desired_size" {
  type    = number
  default = 2
}

variable "sandbox_max_size" {
  type    = number
  default = 30
}

variable "sandbox_capacity_type" {
  description = "ON_DEMAND by default. Switch to SPOT only after interruption-aware sandbox scheduling exists."
  type        = string
  default     = "ON_DEMAND"

  validation {
    condition = contains(
      ["ON_DEMAND", "SPOT"],
      var.sandbox_capacity_type,
    )
    error_message = "sandbox_capacity_type must be ON_DEMAND or SPOT."
  }
}

variable "system_root_volume_size_gib" {
  type    = number
  default = 80
}

variable "runtime_root_volume_size_gib" {
  type    = number
  default = 120
}

variable "sandbox_root_volume_size_gib" {
  type    = number
  default = 120
}

variable "addon_versions" {
  description = "Optional explicitly pinned EKS add-on versions."
  type = object({
    coredns            = optional(string)
    kube_proxy         = optional(string)
    vpc_cni            = optional(string)
    pod_identity_agent = optional(string)
  })
  default = {}
}

variable "tags" {
  description = "Additional resource tags."
  type        = map(string)
  default     = {}
}
