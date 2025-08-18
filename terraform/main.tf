locals {
  common_tags = {
    Project     = "PlatformEngineerAssessment"
    ManagedBy   = "Terraform"
    Environment = "dev"
  }
}

data "aws_caller_identity" "current" {}

# Use default VPC for simplicity
data "aws_vpc" "default" {
  default = true
}

data "aws_subnets" "default" {
  filter {
    name   = "vpc-id"
    values = [data.aws_vpc.default.id]
  }
}

# Latest Ubuntu 22.04 LTS (Jammy) AMD64
data "aws_ami" "ubuntu" {
  most_recent = true
  owners      = ["099720109477"] # Canonical
  filter {
    name   = "name"
    values = ["ubuntu/images/hvm-ssd/ubuntu-jammy-22.04-amd64-server-*"]
  }
  filter {
    name   = "virtualization-type"
    values = ["hvm"]
  }
}

# Key pair from provided OpenSSH public key
resource "aws_key_pair" "ci" {
  key_name   = "${var.name_prefix}-key"
  public_key = var.public_key_openssh
  tags       = local.common_tags
}

resource "aws_security_group" "k3s" {
  name        = "${var.name_prefix}-sg"
  description = "Allow SSH, HTTP/S, NodePort 30080, and (optionally) K8s API 6443"
  vpc_id      = data.aws_vpc.default.id
  tags        = local.common_tags

  # Egress all
  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }
}

# SSH 22
resource "aws_vpc_security_group_ingress_rule" "ssh_multi" {
  for_each          = toset(var.ssh_ingress_cidrs)
  security_group_id = aws_security_group.k3s.id
  from_port         = 22
  to_port           = 22
  ip_protocol       = "tcp"
  cidr_ipv4         = each.value
  description       = "SSH"
}

# HTTP 80
resource "aws_vpc_security_group_ingress_rule" "http" {
  for_each          = toset(var.http_ingress_cidrs)
  security_group_id = aws_security_group.k3s.id
  from_port         = 80
  to_port           = 80
  ip_protocol       = "tcp"
  cidr_ipv4         = each.value
  description       = "HTTP"
}

# HTTPS 443
resource "aws_vpc_security_group_ingress_rule" "https" {
  for_each          = toset(var.http_ingress_cidrs)
  security_group_id = aws_security_group.k3s.id
  from_port         = 443
  to_port           = 443
  ip_protocol       = "tcp"
  cidr_ipv4         = each.value
  description       = "HTTPS"
}

# NodePort 30080 (service exposure)
resource "aws_vpc_security_group_ingress_rule" "nodeport" {
  for_each          = toset(var.http_ingress_cidrs)
  security_group_id = aws_security_group.k3s.id
  from_port         = 30080
  to_port           = 30080
  ip_protocol       = "tcp"
  cidr_ipv4         = each.value
  description       = "NodePort 30080"
}

# K8s API 6443 (optional)
resource "aws_vpc_security_group_ingress_rule" "k8s_api" {
  count             = var.open_k8s_api ? 1 : 0
  security_group_id = aws_security_group.k3s.id
  from_port         = 6443
  to_port           = 6443
  ip_protocol       = "tcp"
  cidr_ipv4         = "0.0.0.0/0"
  description       = "Kubernetes API (k3s)"
}

resource "aws_instance" "k3s" {
  ami                         = data.aws_ami.ubuntu.id
  instance_type               = var.instance_type
  subnet_id                   = data.aws_subnets.default.ids[0]
  vpc_security_group_ids      = [aws_security_group.k3s.id]
  key_name                    = aws_key_pair.ci.key_name
  associate_public_ip_address = true

  root_block_device {
    volume_size = 16
    volume_type = "gp3"
  }

  tags = merge(local.common_tags, { Name = "${var.name_prefix}-ec2" })
}
