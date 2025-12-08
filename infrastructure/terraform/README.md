# Beef Briefing - Linode Infrastructure

This directory contains Terraform configuration to provision the Linode infrastructure for the beef-briefing application.

## Infrastructure Overview

- **1 Linode Instance**: g6-standard-2 (2 vCPUs, 4GB RAM) in Madrid (eu-central)
- **Firewall**: Allows SSH (22), HTTP (80), HTTPS (443); blocks all other inbound traffic
- **DNS**: Managed domain `barra-pesada.online` with A record pointing to the instance
- **SSH Access**: Configured with your `~/.ssh/id_ed25519.pub` key
- **Docker**: Pre-installed via cloud-init (Docker Engine + Docker Compose)

## Prerequisites

1. **Linode API Token**: Create at https://cloud.linode.com/profile/tokens
2. **SSH Key**: Ensure `~/.ssh/id_ed25519.pub` exists
3. **Terraform**: Install from https://www.terraform.io/downloads
4. **Domain**: `barra-pesada.online` must be registered and ready for DNS management

## Setup Instructions

### 1. Configure Variables

Copy the example file and add your Linode API token:

```bash
cd infrastructure/terraform
cp terraform.tfvars.example terraform.tfvars
```

Edit `terraform.tfvars` and set your Linode API token:

```hcl
linode_token = "your-actual-linode-api-token-here"
```

### 2. Initialize Terraform

```bash
terraform init
```

### 3. Review the Plan

```bash
terraform plan
```

Review the resources that will be created:
- Linode instance (g6-standard-2)
- Firewall with rules
- SSH key
- Domain and DNS records
- Random root password

### 4. Apply Configuration

```bash
terraform apply
```

Type `yes` when prompted to confirm.

### 5. Get Outputs

After successful apply, view the outputs:

```bash
# View all outputs
terraform output

# Get specific outputs
terraform output instance_ip
terraform output ssh_connection

# Get sensitive root password
terraform output -raw root_password
```

## Accessing the Instance

After Terraform completes:

```bash
# SSH into the instance
ssh root@<instance_ip>

# Or use the output command
terraform output -raw ssh_connection | bash
```

## DNS Configuration

The Terraform configuration creates:
- Domain: `barra-pesada.online`
- A Record: `barra-pesada.online` → Instance IP
- WWW Record: `www.barra-pesada.online` → Instance IP

**Note**: If your domain is registered elsewhere, you'll need to:
1. Point your domain's nameservers to Linode's nameservers
2. Or manually create the A records in your current DNS provider

## What Gets Installed

The cloud-init script automatically installs:
- Docker Engine (latest stable)
- Docker Compose Plugin (v2)
- Required dependencies (ca-certificates, curl, gnupg)

Verify installation after SSH:

```bash
docker --version
docker compose version
```

## Next Steps

After infrastructure is provisioned:

1. **SSH to the instance**
2. **Clone the repository**:
   ```bash
   git clone https://github.com/vcaldo/beef-briefing.git
   cd beef-briefing
   ```
3. **Set up environment variables** (`.env` file)
4. **Deploy with Docker Compose**:
   ```bash
   docker compose -f infrastructure/docker-compose.prod.yml up -d
   ```

## Managing Infrastructure

### View Current State

```bash
terraform show
```

### Update Infrastructure

Modify `.tf` files and run:

```bash
terraform plan
terraform apply
```

### Destroy Infrastructure

⚠️ **Warning**: This will delete all resources!

```bash
terraform destroy
```

## Troubleshooting

### Cloud-init logs

SSH to instance and check:

```bash
cat /var/log/cloud-init-output.log
```

### Docker not running

```bash
systemctl status docker
systemctl start docker
```

### DNS not resolving

DNS propagation can take up to 24-48 hours. Check status:

```bash
dig barra-pesada.online
nslookup barra-pesada.online
```

## Security Notes

- `terraform.tfvars` contains sensitive data and is gitignored
- Root password is randomly generated and marked sensitive in outputs
- Firewall restricts all ports except 22, 80, 443
- Always use SSH key authentication (password auth disabled by default)

## Cost Estimate

- **Linode g6-standard-2**: ~$24/month
- **Backup (optional)**: ~$5/month
- **Total**: ~$24-29/month

Domain registration and renewal costs are separate.
