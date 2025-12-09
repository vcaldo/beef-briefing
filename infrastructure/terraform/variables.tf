variable "linode_token" {
  description = "Linode API token for authentication"
  type        = string
  sensitive   = true
}

variable "region" {
  description = "Linode region for deployment"
  type        = string
  default     = "eu-central"
}

variable "domain_name" {
  description = "Domain name to configure (e.g., mydomain.example)"
  type        = string
  default     = "mydomain.example"
}

variable "domain_email" {
  description = "Email address for domain SOA record"
  type        = string
  default     = "admin@mydomain.example"
}

variable "ssh_public_key_path" {
  description = "Path to SSH public key file"
  type        = string
  default     = "~/.ssh/id_rsa.pub"
}

variable "hostname" {
  description = "Hostname for the Linode instance"
  type        = string
  default     = "beef-briefing"
}

variable "new_relic_license_key" {
  description = "New Relic license key for infrastructure monitoring"
  type        = string
  sensitive   = true
  default     = ""
}

variable "new_relic_account_id" {
  description = "New Relic account ID for infrastructure monitoring"
  type        = string
  sensitive   = true
  default     = ""
}

variable "new_relic_region" {
  description = "New Relic region (e.g., US or EU)"
  type        = string
  default     = "EU"
}

variable "instance_type" {
  description = "Linode instance type (e.g., g6-standard-2)"
  type        = string
  default     = "g6-standard-2"
}

variable "block_storage_size" {
  description = "Size of the block storage volume in GB for PostgreSQL data"
  type        = number
  default     = 10
}

variable "block_storage_label" {
  description = "Label for the block storage volume"
  type        = string
  default     = "beef-briefing-postgres-data"
}

variable "block_storage_mount_path" {
  description = "Mount path for the block storage volume on the instance"
  type        = string
  default     = "/mnt/postgres-data"
}

variable "linode_volume_id" {
  description = "ID of the existing Linode volume to attach (if null, a new volume will be created)"
  type        = number
  default     = null
}
