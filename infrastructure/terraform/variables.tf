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

variable "postgres_volume_size" {
  description = "Size of the PostgreSQL volume in GB"
  type        = number
  default     = 10
}

variable "postgres_volume_label" {
  description = "Label for the PostgreSQL volume"
  type        = string
  default     = "beef-briefing-postgres-data"
}

variable "postgres_volume_mount_path" {
  description = "Mount path for the PostgreSQL volume on the instance"
  type        = string
  default     = "/mnt/postgres-data"
}

variable "object_storage_region" {
  description = "Linode Object Storage region (e.g., es-mad, eu-central, us-east)"
  type        = string
  default     = "es-mad"
}

variable "object_storage_bucket_suffix" {
  description = "Suffix for the Object Storage bucket (prefixed with sanitized domain name for uniqueness)"
  type        = string
  default     = "telegram-media"
}

variable "object_storage_acl" {
  description = "Access control list for the Object Storage bucket"
  type        = string
  default     = "private"
}

variable "object_storage_versioning" {
  description = "Enable versioning for the Object Storage bucket"
  type        = bool
  default     = true
}

variable "object_storage_lifecycle_expiration_days" {
  description = "Number of days after which objects expire and are deleted"
  type        = number
  default     = 365
}

variable "object_storage_noncurrent_version_expiration_days" {
  description = "Number of days after which noncurrent versions expire and are deleted"
  type        = number
  default     = 5
}
