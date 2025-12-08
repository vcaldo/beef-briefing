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
  description = "Domain name to configure (e.g., barra-pesada.online)"
  type        = string
  default     = "barra-pesada.online"
}

variable "domain_email" {
  description = "Email address for domain SOA record"
  type        = string
  default     = "admin@barra-pesada.online"
}

variable "ssh_public_key_path" {
  description = "Path to SSH public key file"
  type        = string
  default     = "~/.ssh/id_rsa.pub"
}
