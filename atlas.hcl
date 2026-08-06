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