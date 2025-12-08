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

  inbound_policy  = "DROP"
  outbound_policy = "ACCEPT"

  linodes = [linode_instance.beef_briefing.id]
}

# Cloud-init script to install Docker and Docker Compose
locals {
  cloud_init_script = <<-EOF
    #!/bin/bash
    set -e

    # Update system
    apt-get update
    apt-get upgrade -y

    # Install prerequisites
    apt-get install -y ca-certificates curl gnupg lsb-release

    # Add Docker's official GPG key
    install -m 0755 -d /etc/apt/keyrings
    curl -fsSL https://download.docker.com/linux/ubuntu/gpg | gpg --dearmor -o /etc/apt/keyrings/docker.gpg
    chmod a+r /etc/apt/keyrings/docker.gpg

    # Set up Docker repository
    echo \
      "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu \
      $(lsb_release -cs) stable" | tee /etc/apt/sources.list.d/docker.list > /dev/null

    # Install Docker Engine and Docker Compose
    apt-get update
    apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin

    # Enable and start Docker
    systemctl enable docker
    systemctl start docker

    # Add default user to docker group (optional, for non-root docker access)
    usermod -aG docker root

    # Verify installation
    docker --version
    docker compose version

    echo "Docker and Docker Compose installation completed successfully"
  EOF
}

# Linode instance
resource "linode_instance" "beef_briefing" {
  label           = "beef-briefing-server"
  region          = var.region
  type            = "g6-standard-2"
  image           = "linode/ubuntu24.04"
  root_pass       = random_password.root_password.result
  authorized_keys = [linode_sshkey.beef_briefing_key.ssh_key]
  tags            = ["beef-briefing", "production"]

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
