variable "tenant_database_url" {
  type    = string
  default = getenv("TENANT_MIGRATION_DATABASE_URL")
}

env "tenant" {
  url = var.tenant_database_url

  migration {
    dir    = "file://services/tenant/migrations"
    format = golang-migrate
  }
}

variable "organization_database_url" {
  type    = string
  default = getenv("ORGANIZATION_MIGRATION_DATABASE_URL")
}

env "organization" {
  url = var.organization_database_url

  migration {
    dir    = "file://services/organization-agent/migrations"
    format = golang-migrate
  }
}

variable "work_database_url" {
  type    = string
  default = getenv("WORK_MIGRATION_DATABASE_URL")
}

env "work" {
  url = var.work_database_url

  migration {
    dir    = "file://services/work-management/migrations"
    format = golang-migrate
  }
}

variable "policy_database_url" {
  type    = string
  default = getenv("POLICY_MIGRATION_DATABASE_URL")
}

env "policy" {
  url = var.policy_database_url

  migration {
    dir    = "file://services/policy-approval/migrations"
    format = golang-migrate
  }
}

variable "projection_database_url" {
  type    = string
  default = getenv("PROJECTION_MIGRATION_DATABASE_URL")
}

env "projection" {
  url = var.projection_database_url

  dev = getenv("PROJECTION_DEV_DATABASE_URL")

  migration {
    dir    = "file://services/projection/migrations?format=golang-migrate"
    format = golang-migrate
  }
}