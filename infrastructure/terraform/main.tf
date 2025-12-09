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
  token = var.linode_token
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

  inbound {
    label    = "allow-tcp-8081"
    action   = "ACCEPT"
    protocol = "TCP"
    ports    = "8081"
    ipv4     = ["0.0.0.0/0"]
    ipv6     = ["::/0"]
  }

  inbound_policy  = "DROP"
  outbound_policy = "ACCEPT"

  linodes = [linode_instance.beef_briefing.id]
}

# Cloud-init script to install Docker and Docker Compose
locals {
  cloud_init_script = templatefile("${path.module}/cloud-init.yaml", {
    ssh_public_key           = linode_sshkey.beef_briefing_key.ssh_key
    hostname                 = var.hostname
    new_relic_license_key    = var.new_relic_license_key
    new_relic_account_id     = var.new_relic_account_id
    new_relic_region         = var.new_relic_region
    block_storage_label      = var.block_storage_label
    block_storage_mount_path = var.block_storage_mount_path
  })
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

# Use existing volume if volume_id is provided, otherwise create a new one
resource "linode_volume" "beef_briefing_postgres_volume" {
  count     = var.volume_id == null ? 1 : 0
  label     = var.block_storage_label
  region    = var.region
  size      = var.block_storage_size
  linode_id = linode_instance.beef_briefing.id
  tags      = ["beef-briefing", "production", "postgres"]

  lifecycle {
    prevent_destroy = true
  }
}

# Attach existing volume to instance if volume_id is provided
resource "linode_volume_attachment" "beef_briefing_volume_attach" {
  count     = var.volume_id != null ? 1 : 0
  volume_id = var.volume_id
  linode_id = linode_instance.beef_briefing.id
}
