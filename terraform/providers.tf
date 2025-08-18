# Terraform >= 1.5
terraform {
  required_version = ">= 1.5.0"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}

# Provider will read AWS credentials/region from environment variables
# (set by GitHub Actions aws-actions/configure-aws-credentials).
provider "aws" {}
