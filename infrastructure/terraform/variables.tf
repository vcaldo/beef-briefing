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
