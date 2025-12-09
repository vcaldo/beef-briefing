output "instance_id" {
  description = "The ID of the Linode instance"
  value       = linode_instance.beef_briefing.id
}

output "instance_ip" {
  description = "The public IP address of the Linode instance"
  value       = tolist(linode_instance.beef_briefing.ipv4)[0]
}

output "instance_label" {
  description = "The label of the Linode instance"
  value       = linode_instance.beef_briefing.label
}

output "root_password" {
  description = "The root password for the Linode instance (sensitive)"
  value       = random_password.root_password.result
  sensitive   = true
}

output "domain_name" {
  description = "The configured domain name"
  value       = linode_domain.beef_briefing_domain.domain
}

output "domain_id" {
  description = "The ID of the Linode domain"
  value       = linode_domain.beef_briefing_domain.id
}

output "ssh_connection" {
  description = "SSH connection command"
  value       = "ssh admin@${tolist(linode_instance.beef_briefing.ipv4)[0]}"
}

output "ssh_user_host" {
  description = "SSH user and host"
  value       = "admin@${tolist(linode_instance.beef_briefing.ipv4)[0]}"
}

# Block storage outputs removed - volume is managed outside Terraform
# Use linode-cli volumes list to check volume status