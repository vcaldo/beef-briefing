# Beef Briefing - Linode Infrastructure

This directory contains Terraform configuration to provision the complete Linode infrastructure for the beef-briefing application.

## Infrastructure Overview

### Compute Resources
- **Linode Instance**: g6-standard-2 (2 vCPUs, 4GB RAM)
- **Region**: eu-central (Madrid, Spain)
- **OS**: Ubuntu 24.04 LTS
- **User Access**: `admin` user with passwordless sudo + docker group

### Storage Resources
- **PostgreSQL Volume**: 10GB block storage at `/mnt/postgres-data`
  - Persistent across instance rebuilds
  - Automatically mounted via fstab
  - Pre-formatted with ext4
- **Object Storage**: S3-compatible bucket in es-mad region
  - Bucket: `telegram-media`
  - Versioning enabled
  - CORS enabled
  - Lifecycle: 365 days retention, 5 days for old versions

### Networking & Security
- **Firewall**: Custom inbound rules
  - Port 22 (SSH) - Worldwide
  - Port 80 (HTTP) - Worldwide
  - Port 443 (HTTPS) - Worldwide
  - Default policy: DROP all other inbound, ACCEPT all outbound
- **DNS**: Managed domain with Linode DNS
  - A record: `barra-pesada.online` → instance IP
  - A record: `www.barra-pesada.online` → instance IP
  - TTL: 300 seconds

### Monitoring (Optional)
- **New Relic Infrastructure Agent**: Auto-installed if license key provided
- **Region**: EU data center

## Prerequisites

1. **Linode API Token**: Create at https://cloud.linode.com/profile/tokens
2. **SSH Key**: Ensure `~/.ssh/id_rsa.pub` exists
3. **Terraform**: Install from https://www.terraform.io/downloads
4. **Domain**: `barra-pesada.online` must be registered and ready for DNS management

## Setup Instructions

### 1. Configure Variables

Copy the example file and configure your deployment:

```bash
cd infrastructure/terraform
cp terraform.tfvars.example terraform.tfvars
```

Edit `terraform.tfvars` with your values:

```hcl
# Required: Authentication
linode_token = "your-actual-linode-api-token-here"

# Required: Domain Configuration
domain_name  = "barra-pesada.online"
domain_email = "admin@barra-pesada.online"

# Required: SSH Access
ssh_public_key_path = "~/.ssh/id_rsa.pub"

# Optional: Instance Configuration
region        = "eu-central"     # Madrid, Spain
instance_type = "g6-standard-2"  # 2 vCPUs, 4GB RAM
hostname      = "beef-briefing"

# Optional: PostgreSQL Volume
postgres_volume_size       = 10                            # GB
postgres_volume_label      = "beef-briefing-postgres-data"
postgres_volume_mount_path = "/mnt/postgres-data"

# Optional: Object Storage (Telegram media files)
object_storage_region                           = "es-mad"         # Madrid
object_storage_bucket_label                     = "telegram-media"
object_storage_acl                              = "private"
object_storage_versioning                       = true
object_storage_lifecycle_expiration_days        = 365  # 1 year
object_storage_noncurrent_version_expiration_days = 5    # Old versions

# Optional: New Relic Monitoring
new_relic_license_key = ""  # Leave empty to skip installation
new_relic_account_id  = ""
new_relic_region      = "EU"
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
- **linode_instance.beef_briefing**: Ubuntu 24.04 server with cloud-init
- **linode_firewall.beef_briefing_firewall**: Inbound rules for ports 22, 80, 443
- **linode_sshkey.beef_briefing_key**: SSH public key upload
- **linode_domain.beef_briefing_domain**: DNS zone for your domain
- **linode_domain_record.beef_briefing_a_record**: Root domain A record
- **linode_domain_record.beef_briefing_www_record**: WWW subdomain A record
- **linode_volume.beef_briefing_postgres_volume**: 10GB block storage for PostgreSQL
- **linode_object_storage_bucket.telegram_media_bucket**: S3-compatible media storage
- **linode_object_storage_key.telegram_media_key**: Access credentials for bucket
- **random_password.root_password**: Emergency root password (Lish console access)

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

# Instance access
terraform output instance_ip
terraform output ssh_connection

# Object Storage credentials (for .env file)
terraform output minio_endpoint
terraform output minio_access_key
terraform output -raw minio_secret_key

# Emergency access
terraform output -raw root_password
```

**Important**: Save the Object Storage credentials to your `.env` file:
```bash
MINIO_ENDPOINT=$(terraform output -raw minio_endpoint)
MINIO_ACCESS_KEY=$(terraform output -raw minio_access_key)
MINIO_SECRET_KEY=$(terraform output -raw minio_secret_key)
```

## Accessing the Instance

After Terraform completes:

```bash
# SSH into the instance (using admin user)
ssh admin@<instance_ip>

# Or use the output command
terraform output -raw ssh_connection | bash
```

**Note**: The `admin` user has passwordless sudo access and is a member of the `docker` group.

### Verify Installation

After SSH, verify all components are installed:

```bash
# Check versions
docker --version
docker compose version
go version

# Check services
systemctl status docker
systemctl status unattended-upgrades

# Check security hardening
sudo sysctl net.ipv4.tcp_syncookies     # Should be 1
sudo sysctl kernel.randomize_va_space   # Should be 2

# Check PostgreSQL volume
df -h /mnt/postgres-data
ls -la /mnt/postgres-data/pgdata

# Check cloud-init completion
cat /var/log/cloud-init-output.log
```

## DNS Configuration

The Terraform configuration creates:
- Domain: `barra-pesada.online`
- A Record: `barra-pesada.online` → Instance IP
- WWW Record: `www.barra-pesada.online` → Instance IP

**Note**: If your domain is registered elsewhere, you'll need to:
1. Point your domain's nameservers to Linode's nameservers
2. Or manually create the A records in your current DNS provider

## Cloud-Init Configuration

The `cloud-init.yaml` file automates the complete server setup with security hardening:

### User Management
- **Admin User**: `admin`
  - Groups: `sudo`, `docker`
  - Passwordless sudo: `NOPASSWD:ALL`
  - SSH key authentication only
  - Shell: `/bin/bash`

### SSH Hardening
Applied via `/etc/ssh/sshd_config.d/99-hardening.conf`:
- ✅ Root login disabled (`PermitRootLogin no`)
- ✅ Password authentication disabled
- ✅ Public key authentication only
- ✅ Only `admin` user allowed to connect
- ✅ X11 forwarding disabled
- ✅ Max authentication attempts: 3
- ✅ Login grace time: 20 seconds
- ✅ Client keep-alive: 300 seconds (5 min)
- ✅ Max client keep-alive failures: 2

### System Hardening
Applied via `/etc/sysctl.d/99-security.conf`:

**Network Security**:
- ✅ Reverse path filtering (anti-spoofing)
- ✅ ICMP broadcast echo ignored
- ✅ ICMP/IPv6 redirects disabled
- ✅ Source routing disabled
- ✅ SYN cookies enabled (SYN flood protection)

**Kernel Hardening**:
- ✅ ASLR enabled (`kernel.randomize_va_space = 2`)
- ✅ Kernel pointer hiding (`kernel.kptr_restrict = 2`)
- ✅ Dmesg restricted to root (`kernel.dmesg_restrict = 1`)

### Automatic Security Updates
Configured via `/etc/apt/apt.conf.d/20auto-upgrades`:
- ✅ Daily package list updates
- ✅ Automatic security patch installation
- ✅ Weekly cache cleanup

### Software Installation

**Packages Installed**:
- `docker-ce`, `docker-ce-cli`, `containerd.io` (Docker Engine)
- `docker-buildx-plugin`, `docker-compose-plugin` (Docker Compose v2)
- `unattended-upgrades`, `apt-listchanges` (Auto-updates)
- `ca-certificates`, `curl`, `gnupg`, `wget` (Core tools)
- `make`, `vim`, `gzip` (Utilities)

**Custom Installations**:
- **Go 1.25.4**: Installed to `/usr/local/go` (⚠️ version may not exist, update to 1.23.x)
- **LazyDocker**: Interactive Docker TUI at `/usr/local/bin/lazydocker`
- **New Relic Agent**: Infrastructure monitoring (optional, if license key provided)

### Storage Automation

**PostgreSQL Volume Mount**:
1. Waits up to 5 minutes for block storage device to appear
2. Detects device at `/dev/disk/by-id/scsi-0Linode_Volume_beef-briefing-postgres-data`
3. Formats with ext4 (if not already formatted)
4. Mounts to `/mnt/postgres-data`
5. Adds to `/etc/fstab` for persistence across reboots
6. Creates `/mnt/postgres-data/pgdata` with PostgreSQL ownership (UID 999)
7. Sets proper permissions (700)

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

### Access Control
- ✅ Root password randomly generated (32 chars) for emergency Lish console access only
- ✅ Root SSH login **disabled** - only `admin` user can SSH
- ✅ Password authentication **disabled** - SSH keys only
- ✅ `admin` user has passwordless sudo and docker group access
- ✅ SSH max auth attempts: 3 (brute force protection)
- ✅ SSH login grace time: 20 seconds

### Network Security
- ✅ Firewall restricts all ports except 22, 80, 443 (default DROP)
- ✅ PostgreSQL port (5432) **not exposed externally** - internal Docker network only
- ✅ SYN flood protection enabled (tcp_syncookies)
- ✅ ICMP redirects disabled
- ✅ Source routing disabled

### System Hardening
- ✅ ASLR (Address Space Layout Randomization) enabled
- ✅ Kernel pointer hiding enabled
- ✅ Dmesg restricted to root only
- ✅ Automatic security updates enabled (unattended-upgrades)
- ✅ Daily package list updates

### Data Protection
- ✅ PostgreSQL data on persistent block storage (survives instance rebuilds)
- ✅ Object Storage with versioning enabled
- ✅ Object lifecycle policies (365 day retention)
- ✅ S3 bucket ACL: private

### Secrets Management
- ⚠️ `terraform.tfvars` contains sensitive data and is gitignored
- ⚠️ Store Object Storage credentials securely in `.env` file
- ⚠️ New Relic license key stored in Terraform state (use remote backend with encryption)

## Cost Estimate

### Monthly Costs (Linode Pricing)
- **Linode g6-standard-2** (2 vCPUs, 4GB RAM): ~$24/month
- **Block Storage Volume** (10GB): ~$1/month
- **Object Storage**: $5/month (250GB included)
- **Backups** (optional): ~$5/month
- **Data Transfer**: 4TB/month included (overages: $0.01/GB)

**Total**: ~$30-35/month (without backups)

### Additional Costs
- Domain registration/renewal: Varies by registrar
- New Relic (if used): Free tier available, paid plans start at $0

### Cost Optimization Tips
- Delete old object storage versions (currently 5-day retention)
- Monitor data transfer usage via Linode dashboard
- Use Linode's free tier New Relic integration
- Scale down instance type if resource usage is low
