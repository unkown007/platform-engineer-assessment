output "public_ip" {
  description = "Public IP of the EC2 instance."
  value       = aws_instance.k3s.public_ip
}

output "instance_id" {
  description = "ID of the EC2 instance."
  value       = aws_instance.k3s.id
}

output "ssh_command" {
  description = "Convenience SSH command."
  value       = "ssh -i ~/.ssh/gh-actions-ec2 ubuntu@${aws_instance.k3s.public_ip}"
}
