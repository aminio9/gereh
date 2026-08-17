locals {
  tags = merge(
    {
      Project     = "gereh"
      Environment = var.environment
      Plane       = "execution"
      ManagedBy   = "terraform"
    },
    var.tags,
  )

  # Assumes the environment VPC CIDR is /16.
  #
  # Worker subnets:
  #   /20 netnums 0,1,2
  #
  # Public/NAT subnets:
  #   /24 netnums 224,225,226
  #
  # Control-plane intra subnets:
  #   /24 netnums 240,241,242
  private_subnets = [
    for i in range(length(var.azs)) :
    cidrsubnet(var.vpc_cidr, 4, i)
  ]

  public_subnets = [
    for i in range(length(var.azs)) :
    cidrsubnet(var.vpc_cidr, 8, 224 + i)
  ]

  intra_subnets = [
    for i in range(length(var.azs)) :
    cidrsubnet(var.vpc_cidr, 8, 240 + i)
  ]

  cluster_admin_access = {
    platform_admin = {
      principal_arn = var.cluster_admin_role_arn

      policy_associations = {
        cluster_admin = {
          policy_arn = "arn:aws:eks::aws:cluster-access-policy/AmazonEKSClusterAdminPolicy"

          access_scope = {
            type = "cluster"
          }
        }
      }
    }
  }

  break_glass_access = var.break_glass_role_arn == null ? {} : {
    break_glass = {
      principal_arn = var.break_glass_role_arn

      policy_associations = {
        cluster_admin = {
          policy_arn = "arn:aws:eks::aws:cluster-access-policy/AmazonEKSClusterAdminPolicy"

          access_scope = {
            type = "cluster"
          }
        }
      }
    }
  }

  access_entries = merge(
    local.cluster_admin_access,
    local.break_glass_access,
  )
}

module "vpc" {
  source  = "terraform-aws-modules/vpc/aws"
  version = "6.6.1"

  name = "${var.name}-vpc"
  cidr = var.vpc_cidr

  azs             = var.azs
  private_subnets = local.private_subnets
  public_subnets  = local.public_subnets
  intra_subnets   = local.intra_subnets

  enable_dns_support   = true
  enable_dns_hostnames = true

  enable_nat_gateway     = true
  single_nat_gateway     = var.single_nat_gateway
  one_nat_gateway_per_az = !var.single_nat_gateway

  private_subnet_tags = {
    "kubernetes.io/role/internal-elb"   = "1"
    "kubernetes.io/cluster/${var.name}" = "shared"
  }

  public_subnet_tags = {
    "kubernetes.io/role/elb"            = "1"
    "kubernetes.io/cluster/${var.name}" = "shared"
  }

  enable_flow_log                                 = true
  create_flow_log_cloudwatch_log_group            = true
  create_flow_log_cloudwatch_iam_role             = true
  flow_log_traffic_type                           = "ALL"
  flow_log_max_aggregation_interval               = 60
  flow_log_cloudwatch_log_group_retention_in_days = var.vpc_flow_log_retention_days

  tags = local.tags
}

data "aws_iam_policy_document" "vpc_cni_pod_identity_assume" {
  statement {
    effect = "Allow"

    actions = [
      "sts:AssumeRole",
      "sts:TagSession",
    ]

    principals {
      type = "Service"

      identifiers = [
        "pods.eks.amazonaws.com",
      ]
    }
  }
}

resource "aws_iam_role" "vpc_cni" {
  name               = "${var.name}-vpc-cni"
  assume_role_policy = data.aws_iam_policy_document.vpc_cni_pod_identity_assume.json

  tags = local.tags
}

resource "aws_iam_role_policy_attachment" "vpc_cni" {
  role       = aws_iam_role.vpc_cni.name
  policy_arn = "arn:aws:iam::aws:policy/AmazonEKS_CNI_Policy"
}

module "eks" {
  source  = "terraform-aws-modules/eks/aws"
  version = "21.25.0"

  name               = var.name
  region             = var.region
  kubernetes_version = var.kubernetes_version

  deletion_protection = var.deletion_protection

  upgrade_policy = {
    support_type = "STANDARD"
  }

  authentication_mode                      = "API"
  enable_cluster_creator_admin_permissions = false

  access_entries = local.access_entries

  endpoint_private_access = true
  endpoint_public_access  = false

  vpc_id                   = module.vpc.vpc_id
  subnet_ids               = module.vpc.private_subnets
  control_plane_subnet_ids = module.vpc.intra_subnets

  enabled_log_types = [
    "api",
    "audit",
    "authenticator",
    "controllerManager",
    "scheduler",
  ]

  cloudwatch_log_group_retention_in_days = var.control_plane_log_retention_days

  create_kms_key                  = true
  kms_key_description             = "Gereh ${var.environment} execution-cluster Kubernetes secret encryption"
  kms_key_deletion_window_in_days = 30
  enable_kms_key_rotation         = true

  encryption_config = {
    resources = ["secrets"]
  }

  # Execution workloads should use EKS Pod Identity instead of
  # long-lived IAM credentials embedded in Pods.
  enable_irsa = false

  addons = {
    coredns = {
      most_recent   = try(var.addon_versions.coredns, null) == null
      addon_version = try(var.addon_versions.coredns, null)
    }

    kube-proxy = {
      most_recent   = try(var.addon_versions.kube_proxy, null) == null
      addon_version = try(var.addon_versions.kube_proxy, null)
    }

    eks-pod-identity-agent = {
      before_compute = true
      most_recent = (
        try(var.addon_versions.pod_identity_agent, null) == null
      )
      addon_version = try(
        var.addon_versions.pod_identity_agent,
        null,
      )
    }

    vpc-cni = {
      before_compute = true

      most_recent = (
        try(var.addon_versions.vpc_cni, null) == null
      )

      addon_version = try(
        var.addon_versions.vpc_cni,
        null,
      )

      configuration_values = jsonencode({
        enableNetworkPolicy = "true"

        nodeAgent = {
          healthProbeBindAddr = "8163"
          metricsBindAddr     = "8162"
        }
      })

      pod_identity_association = [
        {
          role_arn        = aws_iam_role.vpc_cni.arn
          service_account = "aws-node"
        }
      ]
    }
  }

  eks_managed_node_groups = {
    system = {
      name = "${var.name}-system"

      ami_type                       = "AL2023_x86_64_STANDARD"
      use_latest_ami_release_version = true
      capacity_type                  = "ON_DEMAND"
      instance_types                 = var.system_instance_types

      min_size     = var.system_min_size
      desired_size = var.system_desired_size
      max_size     = var.system_max_size

      iam_role_attach_cni_policy = false

      labels = {
        "gereh.ai/node-pool" = "system"
        "gereh.ai/plane"     = "execution"
      }

      node_repair_config = {
        enabled = true
      }

      update_config = {
        max_unavailable_percentage = 33
      }

      metadata_options = {
        http_endpoint               = "enabled"
        http_tokens                 = "required"
        http_put_response_hop_limit = 1
        instance_metadata_tags      = "disabled"
      }

      block_device_mappings = {
        root = {
          device_name = "/dev/xvda"

          ebs = {
            encrypted             = true
            delete_on_termination = true
            volume_type           = "gp3"
            volume_size           = var.system_root_volume_size_gib
            iops                  = 3000
            throughput            = 125
          }
        }
      }
    }

    runtime = {
      name = "${var.name}-runtime"

      ami_type                       = "AL2023_x86_64_STANDARD"
      use_latest_ami_release_version = true
      capacity_type                  = "ON_DEMAND"
      instance_types                 = var.runtime_instance_types

      min_size     = var.runtime_min_size
      desired_size = var.runtime_desired_size
      max_size     = var.runtime_max_size

      iam_role_attach_cni_policy = false

      labels = {
        "gereh.ai/node-pool" = "runtime"
        "gereh.ai/plane"     = "execution"
      }

      taints = {
        runtime = {
          key    = "gereh.ai/workload"
          value  = "runtime"
          effect = "NO_SCHEDULE"
        }
      }

      node_repair_config = {
        enabled = true
      }

      update_config = {
        max_unavailable_percentage = 25
      }

      metadata_options = {
        http_endpoint               = "enabled"
        http_tokens                 = "required"
        http_put_response_hop_limit = 1
        instance_metadata_tags      = "disabled"
      }

      block_device_mappings = {
        root = {
          device_name = "/dev/xvda"

          ebs = {
            encrypted             = true
            delete_on_termination = true
            volume_type           = "gp3"
            volume_size           = var.runtime_root_volume_size_gib
            iops                  = 3000
            throughput            = 125
          }
        }
      }
    }

    sandbox = {
      name = "${var.name}-sandbox"

      ami_type                       = "AL2023_x86_64_STANDARD"
      use_latest_ami_release_version = true
      capacity_type                  = var.sandbox_capacity_type
      instance_types                 = var.sandbox_instance_types

      min_size     = var.sandbox_min_size
      desired_size = var.sandbox_desired_size
      max_size     = var.sandbox_max_size

      iam_role_attach_cni_policy = false

      labels = {
        "gereh.ai/node-pool" = "sandbox"
        "gereh.ai/plane"     = "execution"
      }

      taints = {
        sandbox = {
          key    = "gereh.ai/workload"
          value  = "sandbox"
          effect = "NO_SCHEDULE"
        }
      }

      node_repair_config = {
        enabled = true
      }

      update_config = {
        max_unavailable_percentage = 25
      }

      metadata_options = {
        http_endpoint               = "enabled"
        http_tokens                 = "required"
        http_put_response_hop_limit = 1
        instance_metadata_tags      = "disabled"
      }

      block_device_mappings = {
        root = {
          device_name = "/dev/xvda"

          ebs = {
            encrypted             = true
            delete_on_termination = true
            volume_type           = "gp3"
            volume_size           = var.sandbox_root_volume_size_gib
            iops                  = 3000
            throughput            = 125
          }
        }
      }
    }
  }

  tags = local.tags

  depends_on = [
    aws_iam_role_policy_attachment.vpc_cni,
  ]
}
