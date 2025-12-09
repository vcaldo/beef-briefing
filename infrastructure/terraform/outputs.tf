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

output "postgres_volume_id" {
  description = "The ID of the PostgreSQL volume"
  value       = linode_volume.beef_briefing_postgres_volume.id
}

output "postgres_volume_size" {
  description = "The size of the PostgreSQL volume in GB"
  value       = linode_volume.beef_briefing_postgres_volume.size
}

output "postgres_volume_filesystem_path" {
  description = "The filesystem path for the PostgreSQL volume"
  value       = linode_volume.beef_briefing_postgres_volume.filesystem_path
}

output "object_storage_endpoint" {
  description = "The endpoint URL for Linode Object Storage (without https://)"
  value       = "${linode_object_storage_bucket.telegram_media_bucket.region}.linodeobjects.com"
}

output "object_storage_access_key_id" {
  description = "The access key ID for Object Storage"
  value       = linode_object_storage_key.telegram_media_key.access_key
  sensitive   = true
}

output "object_storage_secret_access_key" {
  description = "The secret access key for Object Storage"
  value       = linode_object_storage_key.telegram_media_key.secret_key
  sensitive   = true
}

output "object_storage_bucket_name" {
  description = "The name of the Object Storage bucket"
  value       = linode_object_storage_bucket.telegram_media_bucket.label
}