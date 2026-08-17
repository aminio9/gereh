provider "aws" {
  region = var.aws_region

  default_tags {
    tags = {
      Project     = "gereh"
      Environment = var.environment
      Plane       = "execution"
      ManagedBy   = "terraform"
    }
  }
}
