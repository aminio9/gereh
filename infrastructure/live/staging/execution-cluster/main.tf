module "execution_cluster" {
  source = "../../../modules/execution-cluster-aws"

  name               = var.cluster_name
  environment        = var.environment
  region             = var.aws_region
  kubernetes_version = "1.36"

  vpc_cidr = var.vpc_cidr
  azs      = var.azs

  single_nat_gateway  = var.single_nat_gateway
  deletion_protection = var.deletion_protection

  cluster_admin_role_arn = var.cluster_admin_role_arn
  break_glass_role_arn   = var.break_glass_role_arn

  control_plane_log_retention_days = var.control_plane_log_retention_days
  vpc_flow_log_retention_days      = var.vpc_flow_log_retention_days

  system_min_size     = var.system_min_size
  system_desired_size = var.system_desired_size
  system_max_size     = var.system_max_size

  runtime_min_size     = var.runtime_min_size
  runtime_desired_size = var.runtime_desired_size
  runtime_max_size     = var.runtime_max_size

  sandbox_min_size      = var.sandbox_min_size
  sandbox_desired_size  = var.sandbox_desired_size
  sandbox_max_size      = var.sandbox_max_size
  sandbox_capacity_type = var.sandbox_capacity_type

  addon_versions = var.addon_versions
}

output "cluster_name" {
  value = module.execution_cluster.cluster_name
}

output "cluster_endpoint" {
  value = module.execution_cluster.cluster_endpoint
}

output "cluster_version" {
  value = module.execution_cluster.cluster_version
}

output "vpc_id" {
  value = module.execution_cluster.vpc_id
}

output "kms_key_arn" {
  value = module.execution_cluster.kms_key_arn
}
