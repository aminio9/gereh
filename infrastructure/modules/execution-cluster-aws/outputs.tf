output "cluster_name" {
  description = "Execution EKS cluster name."
  value       = module.eks.cluster_name
}

output "cluster_endpoint" {
  description = "Private Kubernetes API endpoint."
  value       = module.eks.cluster_endpoint
}

output "cluster_version" {
  description = "Kubernetes version."
  value       = module.eks.cluster_version
}

output "cluster_status" {
  description = "EKS cluster status."
  value       = module.eks.cluster_status
}

output "cluster_arn" {
  description = "EKS cluster ARN."
  value       = module.eks.cluster_arn
}

output "cluster_security_group_id" {
  description = "EKS cluster security group."
  value       = module.eks.cluster_security_group_id
}

output "node_security_group_id" {
  description = "Shared EKS node security group."
  value       = module.eks.node_security_group_id
}

output "kms_key_arn" {
  description = "KMS key used for Kubernetes secret envelope encryption."
  value       = module.eks.kms_key_arn
}

output "vpc_id" {
  description = "Execution VPC ID."
  value       = module.vpc.vpc_id
}

output "private_subnet_ids" {
  description = "Execution worker subnet IDs."
  value       = module.vpc.private_subnets
}

output "control_plane_subnet_ids" {
  description = "EKS control-plane subnet IDs."
  value       = module.vpc.intra_subnets
}

output "public_subnet_ids" {
  description = "NAT/public subnet IDs."
  value       = module.vpc.public_subnets
}

output "vpc_cni_role_arn" {
  description = "Pod Identity role used by the AWS VPC CNI."
  value       = aws_iam_role.vpc_cni.arn
}
