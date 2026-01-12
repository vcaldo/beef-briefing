terraform {
  required_providers {
    linode = {
      source  = "linode/linode"
      version = "~> 3.6.0"
    }
    random = {
      source  = "hashicorp/random"
      version = "~> 3.0"
    }
  }
  required_version = ">= 1.0"
}

provider "linode" {
  token             = var.linode_token
  obj_use_temp_keys = true
}

# Generate random root password
resource "random_password" "root_password" {
  length  = 32
  special = true
}

# SSH key for instance access
resource "linode_sshkey" "beef_briefing_key" {
  label   = "beef-briefing-deploy-key"
  ssh_key = chomp(file(pathexpand(var.ssh_public_key_path)))
}

# Firewall rules
resource "linode_firewall" "beef_briefing_firewall" {
  label = "beef-briefing-firewall"
  tags  = ["beef-briefing", "production"]

  inbound {
    label    = "allow-ssh"
    action   = "ACCEPT"
    protocol = "TCP"
    ports    = "22"
    ipv4     = ["0.0.0.0/0"]
    ipv6     = ["::/0"]
  }

  inbound {
    label    = "allow-http"
    action   = "ACCEPT"
    protocol = "TCP"
    ports    = "80"
    ipv4     = ["0.0.0.0/0"]
    ipv6     = ["::/0"]
  }

  inbound {
    label    = "allow-https"
    action   = "ACCEPT"
    protocol = "TCP"
    ports    = "443"
    ipv4     = ["0.0.0.0/0"]
    ipv6     = ["::/0"]
  }

  #  inbound {
  #   label    = "allow-http-8081"
  #   action   = "ACCEPT"
  #   protocol = "TCP"
  #   ports    = "8081"
  #   ipv4     = ["0.0.0.0/0"]
  #   ipv6     = ["::/0"]
  # }

  inbound_policy  = "DROP"
  outbound_policy = "ACCEPT"

  linodes = [linode_instance.beef_briefing.id]
}

# Cloud-init script to install Docker and Docker Compose
locals {
  cloud_init_script = templatefile("${path.module}/cloud-init.yaml", {
    ssh_public_key             = linode_sshkey.beef_briefing_key.ssh_key
    hostname                   = var.hostname
    new_relic_license_key      = var.new_relic_license_key
    new_relic_account_id       = var.new_relic_account_id
    new_relic_region           = var.new_relic_region
    postgres_volume_label      = var.postgres_volume_label
    postgres_volume_mount_path = var.postgres_volume_mount_path
  })

  # Sanitize domain name for use in bucket label (replace dots with dashes)
  # e.g., "barra-pesada.online" -> "barra-pesada-online"
  sanitized_domain = replace(var.domain_name, ".", "-")

  # Generate unique bucket label: {sanitized-domain}-{bucket-suffix}
  # e.g., "barra-pesada-online-telegram-media"
  object_storage_bucket_label = "${local.sanitized_domain}-${var.object_storage_bucket_suffix}"
}

# Linode instance
resource "linode_instance" "beef_briefing" {
  label     = "beef-briefing-server"
  region    = var.region
  type      = var.instance_type
  image     = "linode/ubuntu24.04"
  root_pass = random_password.root_password.result
  tags      = ["beef-briefing", "production"]

  metadata {
    user_data = base64encode(local.cloud_init_script)
  }

  # to recreate the linode it's necessary to taint the resouce
  # terraform taint linode_instance.beef_briefing
  lifecycle {
    ignore_changes = [metadata]
  }
}

# Domain management
resource "linode_domain" "beef_briefing_domain" {
  domain    = var.domain_name
  soa_email = var.domain_email
  type      = "master"
  tags      = ["beef-briefing", "production"]
}

# A record pointing to Linode instance
resource "linode_domain_record" "beef_briefing_a_record" {
  domain_id   = linode_domain.beef_briefing_domain.id
  name        = ""
  record_type = "A"
  target      = tolist(linode_instance.beef_briefing.ipv4)[0]
  ttl_sec     = 300
}

# WWW subdomain (optional, pointing to same IP)
resource "linode_domain_record" "beef_briefing_www_record" {
  domain_id   = linode_domain.beef_briefing_domain.id
  name        = "www"
  record_type = "A"
  target      = tolist(linode_instance.beef_briefing.ipv4)[0]
  ttl_sec     = 300
}

# API subdomain for external API access
resource "linode_domain_record" "beef_briefing_api_record" {
  domain_id   = linode_domain.beef_briefing_domain.id
  name        = "api"
  record_type = "A"
  target      = tolist(linode_instance.beef_briefing.ipv4)[0]
  ttl_sec     = 300
}

# Cards API subdomain for external card-image-generator access
resource "linode_domain_record" "beef_briefing_cards_api_record" {
  domain_id   = linode_domain.beef_briefing_domain.id
  name        = "cards-api"
  record_type = "A"
  target      = tolist(linode_instance.beef_briefing.ipv4)[0]
  ttl_sec     = 300
}

# Deck subdomain for Telegram Mini App access
resource "linode_domain_record" "beef_briefing_deck_record" {
  domain_id   = linode_domain.beef_briefing_domain.id
  name        = "deck"
  record_type = "A"
  target      = tolist(linode_instance.beef_briefing.ipv4)[0]
  ttl_sec     = 300
}

# Leaderboard subdomain for Telegram Mini App access
resource "linode_domain_record" "beef_briefing_leaderboard_record" {
  domain_id   = linode_domain.beef_briefing_domain.id
  name        = "leaderboard"
  record_type = "A"
  target      = tolist(linode_instance.beef_briefing.ipv4)[0]
  ttl_sec     = 300
}

# Arena subdomain for Telegram Mini App access
resource "linode_domain_record" "beef_briefing_arena_record" {
  domain_id   = linode_domain.beef_briefing_domain.id
  name        = "arena"
  record_type = "A"
  target      = tolist(linode_instance.beef_briefing.ipv4)[0]
  ttl_sec     = 300
}

# PostgreSQL persistent data volume
resource "linode_volume" "beef_briefing_postgres_volume" {
  label     = var.postgres_volume_label
  region    = var.region
  size      = var.postgres_volume_size
  linode_id = linode_instance.beef_briefing.id
  tags      = ["beef-briefing", "production", "postgres"]
}

# Object Storage bucket for media files
resource "linode_object_storage_bucket" "telegram_media_bucket" {
  region = var.object_storage_region
  label  = local.object_storage_bucket_label
  acl    = var.object_storage_acl

  lifecycle_rule {
    enabled = true
    id      = "expire-old-objects"

    expiration {
      days = var.object_storage_lifecycle_expiration_days
    }

    noncurrent_version_expiration {
      days = var.object_storage_noncurrent_version_expiration_days
    }
  }

  cors_enabled = true
  versioning   = var.object_storage_versioning
}

# Object Storage access keys
resource "linode_object_storage_key" "telegram_media_key" {
  label = "beef-briefing-telegram-media-key"

  bucket_access {
    bucket_name = linode_object_storage_bucket.telegram_media_bucket.label
    region      = linode_object_storage_bucket.telegram_media_bucket.region
    permissions = "read_write"
  }
}
