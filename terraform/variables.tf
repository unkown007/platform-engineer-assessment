variable "public_key_openssh" {
  description = "Your SSH public key in OpenSSH format (e.g., ssh-ed25519 AAAA...)."
  type        = string
}

variable "instance_type" {
  description = "EC2 instance type."
  type        = string
  default     = "t3.micro"
}

variable "name_prefix" {
  description = "Name prefix for created resources."
  type        = string
  default     = "pe-assessment"
}

variable "ssh_ingress_cidrs" {
  description = "CIDR blocks allowed to SSH into the instance."
  type        = list(string)
  default     = ["0.0.0.0/0"]
}

variable "http_ingress_cidrs" {
  description = "CIDR blocks allowed to access HTTP/HTTPS/NodePort."
  type        = list(string)
  default     = ["0.0.0.0/0"]
}

variable "open_k8s_api" {
  description = "Whether to open Kubernetes API server (6443) to the world (true) or restrict it to the runner via ssh tunnel later (false)."
  type        = bool
  default     = true
}
