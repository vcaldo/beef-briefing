# Beef Briefing - Linode Infrastructure

This directory contains Terraform configuration to provision the complete Linode infrastructure for the beef-briefing application.

## Infrastructure Overview

### Compute Resources
- **Linode Instance**: g6-standard-2 (2 vCPUs, 4GB RAM) - configurable
- **Region**: eu-central (default) - configurable
- **OS**: Ubuntu 24.04 LTS
- **User Access**: `admin` user with passwordless sudo + docker group

### Application Services
Deployed via Docker Compose (`docker-compose.prod.yml`):
- **postgres**: PostGIS 17-3.4 (PostgreSQL with geographic extensions)
- **api-service**: Go REST API for data access and Telegram integration
- **telegram-bot**: Go Telegram bot for group interaction
- **dashboard**: Python Flask dashboard with analytics
- **newrelic-infra**: New Relic infrastructure monitoring (optional)

### Storage Resources
- **PostgreSQL Volume**: Block storage for persistent database data
  - Default: 10GB (configurable)
  - Mount path: `/mnt/postgres-data` (configurable)
  - Persistent across instance rebuilds
  - Automatically mounted via fstab
  - Pre-formatted with ext4
  - PostgreSQL data directory: `/mnt/postgres-data/pgdata`
- **Object Storage**: S3-compatible Linode Object Storage
  - Region: es-mad (Madrid) by default
  - Bucket name: `{sanitized-domain}-{suffix}` (e.g., `barra-pesada-online-telegram-media`)
  - Purpose: Telegram media files (photos, videos, documents)
  - Versioning: Enabled (configurable)
  - CORS: Enabled
  - Lifecycle: 365 days retention, 5 days for old versions (configurable)
  - ACL: Private

### Networking & Security
- **Firewall**: Stateful firewall with custom inbound rules
  - Port 22 (SSH) - Worldwide access
  - Port 80 (HTTP) - Worldwide access
  - Port 443 (HTTPS) - Worldwide access
  - Default policy: DROP all other inbound, ACCEPT all outbound
  - PostgreSQL (5432) NOT exposed - internal Docker network only
- **DNS**: Managed domain with Linode DNS
  - A record: `{domain_name}` → instance IP
  - A record: `www.{domain_name}` → instance IP
  - TTL: 300 seconds (5 minutes)

### Monitoring (Optional)
- **New Relic Infrastructure Agent**: Auto-installed if license key provided
- **Region**: Configurable (EU or US)
- **Integration**: Docker container monitoring enabled

## Prerequisites

1. **Linode API Token**: Create at https://cloud.linode.com/profile/tokens
   - Required scopes: Read/Write for Domains, Linodes, Object Storage, Volumes, Firewalls
2. **SSH Key**: Ensure you have an SSH public key (default: `~/.ssh/id_rsa.pub`)
3. **Terraform**: Install from https://www.terraform.io/downloads (tested with v1.0+)
4. **Domain**: Domain name must be registered (can be from any registrar)
   - You'll need to point nameservers to Linode DNS after deployment

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
domain_name  = "yourdomain.example"
domain_email = "admin@yourdomain.example"

# Required: SSH Access
ssh_public_key_path = "~/.ssh/id_rsa.pub"

# Optional: Instance Configuration
region        = "eu-central"     # Available: eu-central, es-mad, us-east, us-central, etc.
instance_type = "g6-standard-2"  # 2 vCPUs, 4GB RAM (other options: g6-nanode-1, g6-standard-4)
hostname      = "beef-briefing"

# Optional: PostgreSQL Volume
postgres_volume_size       = 10                            # GB
postgres_volume_label      = "beef-briefing-postgres-data"
postgres_volume_mount_path = "/mnt/postgres-data"

# Optional: Object Storage (Telegram media files)
# Note: Bucket name is auto-generated as {sanitized-domain}-{suffix}
# e.g., "yourdomain-example-telegram-media" for domain "yourdomain.example"
object_storage_region                             = "es-mad"         # Madrid
object_storage_bucket_suffix                      = "telegram-media" # Suffix only
object_storage_acl                                = "private"
object_storage_versioning                         = true
object_storage_lifecycle_expiration_days          = 365  # 1 year
object_storage_noncurrent_version_expiration_days = 5    # Old versions

# Optional: New Relic Monitoring
new_relic_license_key = ""  # Leave empty to skip installation
new_relic_account_id  = ""
new_relic_region      = "EU"  # or "US"
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
# View all outputs including deployment notes
terraform output

# View deployment instructions
terraform output -raw deployment_notes

# Instance access
terraform output instance_ip
terraform output ssh_connection

# Object Storage credentials (for .env file)
terraform output object_storage_endpoint
terraform output object_storage_access_key_id
terraform output -raw object_storage_secret_access_key
terraform output object_storage_bucket_name

# Emergency access (Lish console only)
terraform output -raw root_password
```

**Important**: Save the Object Storage credentials - you'll need them for your `.env` file.

### 6. Configure DNS Nameservers

After Terraform creates the domain, configure your domain registrar:

1. Log into Linode Cloud Manager
2. Navigate to Domains → Your Domain
3. Note the nameservers (e.g., `ns1.linode.com`, `ns2.linode.com`, etc.)
4. Log into your domain registrar
5. Update nameservers to point to Linode's nameservers
6. Wait for DNS propagation (can take up to 24-48 hours)

Verify DNS propagation:
```bash
dig @8.8.8.8 yourdomain.example
```

## Accessing the Instance

After Terraform completes and DNS is configured:

```bash
# SSH into the instance (using admin user)
ssh admin@<instance_ip>

# Or use the domain name (after DNS propagates)
ssh admin@yourdomain.example
```

**Note**: The `admin` user has passwordless sudo access and is a member of the `docker` group.

### Verify Installation

After SSH, verify all components are installed:

```bash
# Check versions
docker --version
docker compose version

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
sudo cat /var/log/cloud-init-output.log
```

## DNS Configuration

The Terraform configuration creates a Linode DNS zone with:
- Domain: Your configured domain name (e.g., `yourdomain.example`)
- A Record: `yourdomain.example` → Instance IP
- WWW Record: `www.yourdomain.example` → Instance IP

### Pointing Your Domain to Linode DNS

**If your domain is registered elsewhere**, you need to:

1. **After Terraform apply**, get Linode's nameservers:
   - Log into Linode Cloud Manager
   - Navigate to **Domains** → Select your domain
   - Note the nameservers listed (typically 5 nameservers: `ns1.linode.com` through `ns5.linode.com`)

2. **Update your domain registrar**:
   - Log into your domain registrar (GoDaddy, Namecheap, Google Domains, etc.)
   - Find DNS/Nameserver settings
   - Replace existing nameservers with Linode's nameservers
   - Save changes

3. **Wait for propagation**:
   - DNS changes can take 24-48 hours to propagate globally
   - Check status: `dig @8.8.8.8 yourdomain.example`
   - Or use online tools: https://www.whatsmydns.net/

**If your domain is already at Linode**, the DNS records will be automatically configured.

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
- `unattended-upgrades`, `apt-listchanges` (Automatic security updates)
- `ca-certificates`, `curl`, `gnupg`, `wget` (Core tools)
- `vim`, `gzip`, `bash-completion` (Utilities)

**Custom Installations**:
- **LazyDocker**: Interactive Docker TUI at `/usr/local/bin/lazydocker` (optional)
- **New Relic Agent**: Infrastructure monitoring (optional, if license key provided)

**Note**: Go is NOT installed on the server - all services run in Docker containers.

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

After infrastructure is provisioned, follow these steps to deploy the application:

### 1. SSH to the Instance

```bash
ssh admin@<instance_ip>
```

### 2. Clone the Repository

```bash
cd /home/admin/beef-briefing
git clone https://github.com/vcaldo/beef-briefing.git .
```

### 3. Set Up Environment Variables

Create `.env` file in `/home/admin/beef-briefing/infrastructure/`:

```bash
cd ~/beef-briefing/infrastructure
nano .env
```

Add the following (use Terraform outputs for Object Storage credentials):

```bash
# Database Configuration
DB_USER=beef_briefing_user
DB_PASSWORD=<generate-strong-password>
DB_NAME=beef_briefing

# API Service
API_PORT=8080
TELEGRAM_BOT_TOKEN=<your-telegram-bot-token>

# Dashboard
DASHBOARD_PORT=8050

# Object Storage (Linode S3-compatible)
# Use: terraform output object_storage_endpoint
MINIO_ENDPOINT=<from-terraform-output>
# Use: terraform output object_storage_access_key_id
MINIO_ACCESS_KEY=<from-terraform-output>
# Use: terraform output -raw object_storage_secret_access_key
MINIO_SECRET_KEY=<from-terraform-output>
# Use: terraform output object_storage_bucket_name
MINIO_BUCKET=<from-terraform-output>
MINIO_USE_SSL=true

# Deployment
IMAGE_TAG=latest
POSTGRES_DATA_PATH=/mnt/postgres-data
```

### 4. Set Up Dashboard Secrets

```bash
cd ~/beef-briefing/infrastructure/secrets/apps/dashboard

# Generate Flask secret key
openssl rand -base64 32 > flask_secret_key
chmod 600 flask_secret_key

# Verify files
ls -la
```

### 5. Deploy with Docker Compose

```bash
cd ~/beef-briefing/infrastructure
docker compose -f docker-compose.prod.yml pull
docker compose -f docker-compose.prod.yml up -d
```

### 6. Verify Deployment

```bash
# Check containers
docker ps

# Check logs
docker compose -f docker-compose.prod.yml logs -f

# Test API
curl http://localhost:8080/health

# Test admin panel
curl http://localhost:8081
```

### 7. Configure SSL/TLS (Recommended)

For production use, set up HTTPS with Let's Encrypt:

```bash
# Install Caddy (automatic HTTPS)
sudo apt install -y debian-keyring debian-archive-keyring apt-transport-https
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' | sudo gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' | sudo tee /etc/apt/sources.list.d/caddy-stable.list
sudo apt update
sudo apt install caddy

# Configure Caddyfile
sudo nano /etc/caddy/Caddyfile
```

Example Caddyfile:
```
yourdomain.example {
    reverse_proxy localhost:8080
}

www.yourdomain.example {
    reverse_proxy localhost:8080
}

admin.yourdomain.example {
    reverse_proxy localhost:8081
}
```

```bash
# Restart Caddy
sudo systemctl restart caddy
```

## Managing Infrastructure

### View Current State

```bash
# View all resources
terraform show

# View specific output
terraform output instance_ip

# View state list
terraform state list
```

### Update Infrastructure

Modify `.tf` or `terraform.tfvars` files and run:

```bash
terraform plan
terraform apply
```

### Common Updates

**Resize PostgreSQL volume**:
```hcl
# In terraform.tfvars
postgres_volume_size = 20  # Increase from 10GB to 20GB
```

**Change instance type**:
```hcl
# In terraform.tfvars
instance_type = "g6-standard-4"  # Upgrade to 4 vCPUs, 8GB RAM
```

**Update Object Storage lifecycle**:
```hcl
# In terraform.tfvars
object_storage_lifecycle_expiration_days = 180  # Change from 365 to 180 days
```

### Destroy Infrastructure

⚠️ **Warning**: This will permanently delete all resources including data!

Before destroying:
1. Backup your PostgreSQL database
2. Download any important files from Object Storage
3. Save any configuration you may need

```bash
# View what will be destroyed
terraform plan -destroy

# Destroy all resources
terraform destroy
```

**Note**: Object Storage buckets must be empty before they can be destroyed. Delete all objects first or use the Linode Cloud Manager to force delete.

## Troubleshooting

### Cloud-init logs

SSH to instance and check initialization logs:

```bash
sudo cat /var/log/cloud-init-output.log
sudo journalctl -u cloud-final
```

### PostgreSQL volume not mounted

```bash
# Check if volume is attached
lsblk

# Check mount status
df -h | grep postgres

# Check fstab entry
cat /etc/fstab | grep postgres

# Manual mount (if needed)
sudo mount /mnt/postgres-data
```

### Docker not running

```bash
systemctl status docker
sudo systemctl start docker
sudo systemctl enable docker

# Check if admin user is in docker group
groups admin
```

### DNS not resolving

DNS propagation can take up to 24-48 hours. Check status:

```bash
# Check nameservers
dig NS yourdomain.example

# Check A record
dig A yourdomain.example

# Use specific nameserver
dig @8.8.8.8 yourdomain.example

# Online tool
# Visit: https://www.whatsmydns.net/
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

## Terraform Resources Created

This configuration creates the following Linode resources:

### Core Resources
- `random_password.root_password` - Random 32-character root password for emergency access
- `linode_sshkey.beef_briefing_key` - SSH public key for admin user authentication
- `linode_instance.beef_briefing` - Ubuntu 24.04 compute instance with cloud-init
- `linode_firewall.beef_briefing_firewall` - Firewall with SSH/HTTP/HTTPS rules

### Storage Resources
- `linode_volume.beef_briefing_postgres_volume` - Block storage volume for PostgreSQL data
- `linode_object_storage_bucket.telegram_media_bucket` - S3-compatible object storage bucket
- `linode_object_storage_key.telegram_media_key` - Access credentials for object storage

### Networking Resources
- `linode_domain.beef_briefing_domain` - DNS zone for your domain
- `linode_domain_record.beef_briefing_a_record` - A record for root domain
- `linode_domain_record.beef_briefing_www_record` - A record for www subdomain

## Cost Estimate

### Monthly Costs (Linode Pricing - December 2025)

| Resource | Configuration | Estimated Cost |
|----------|---------------|----------------|
| Linode Instance | g6-standard-2 (2 vCPUs, 4GB RAM) | ~$24/month |
| Block Storage | 10GB volume | ~$1/month |
| Object Storage | Up to 250GB storage + 1TB egress | $5/month (base tier) |
| Data Transfer | 4TB/month included with instance | Included |
| DNS Hosting | Linode DNS zone + records | Free |
| Firewall | Stateful firewall rules | Free |
| **Subtotal** | | **~$30/month** |
| Backups (optional) | Weekly automated backups | +$5/month |

### Additional Potential Costs
- **Domain Registration**: $10-20/year (external to Linode)
- **New Relic**: Free tier available (100GB/month data ingest)
- **Object Storage Overages**: $0.02/GB over 250GB
- **Data Transfer Overages**: $0.005-0.01/GB over 4TB
- **Additional Block Storage**: $0.10/GB/month

### Cost Optimization Tips
1. **Object Storage Lifecycle**: Configure shorter retention (currently 365 days)
2. **Monitor Data Transfer**: Use Linode dashboard to track bandwidth usage
3. **Instance Right-Sizing**: Start with g6-standard-2, scale up only if needed
4. **Block Storage**: Only expand volume when necessary ($0.10/GB/month)
5. **Backups**: Use `pg_dump` to backup to Object Storage instead of paid Linode Backups
6. **Object Storage Versioning**: Disable if not needed to reduce storage costs

### Monitoring Costs
Check your usage in Linode Cloud Manager:
- **Account** → **Billing Info** → View current month's charges
- **Linodes** → Your instance → **Network** tab → Data transfer graph
- **Object Storage** → Your bucket → Usage statistics

## Architecture Diagram

```
┌─────────────────────────────────────────────────────────────────────┐
│                         Linode Cloud                                │
├─────────────────────────────────────────────────────────────────────┤
│                                                                       │
│  ┌──────────────────────────────────────────────────────────────┐  │
│  │  Linode DNS (ns1-5.linode.com)                               │  │
│  │  • A record: yourdomain.example → Instance IP                │  │
│  │  • A record: www.yourdomain.example → Instance IP            │  │
│  └──────────────────────────────────────────────────────────────┘  │
│                            ↓ DNS Resolution                          │
│  ┌──────────────────────────────────────────────────────────────┐  │
│  │  Firewall Rules (linode_firewall)                            │  │
│  │  ✓ Port 22 (SSH) - Admin access                              │  │
│  │  ✓ Port 80 (HTTP) - Web traffic                              │  │
│  │  ✓ Port 443 (HTTPS) - Secure web traffic                     │  │
│  │  ✗ All other ports - DROPPED                                 │  │
│  └──────────────────────────────────────────────────────────────┘  │
│                            ↓                                         │
│  ┌──────────────────────────────────────────────────────────────┐  │
│  │  Linode Instance (g6-standard-2)                             │  │
│  │  Ubuntu 24.04 LTS • 2 vCPUs • 4GB RAM • admin user          │  │
│  │                                                                │  │
│  │  ┌────────────────────────────────────────────────────────┐ │  │
│  │  │         Docker Compose Environment                      │ │  │
│  │  │                                                          │ │  │
│  │  │  ┌──────────────┐  ┌──────────────┐  ┌─────────────┐  │ │  │
│  │  │  │  postgres    │  │  api-service │  │ telegram-bot│  │ │  │
│  │  │  │  (PostGIS)   │◄─┤  (Go/REST)   │◄─┤   (Go)      │  │ │  │
│  │  │  │  :5432       │  │  :8080       │  │             │  │ │  │
│  │  │  └──────┬───────┘  └──────┬───────┘  └─────────────┘  │ │  │
│  │  │         │                   │                           │ │  │
│  │  │         │                   │   ┌──────────────────┐   │ │  │
│  │  │         │                   └──►│  dashboard       │   │ │  │
│  │  │         │                       │  (Python/Flask)  │   │ │  │
│  │  │         │                       │  :8050           │   │ │  │
│  │  │         │                       └──────────────────┘   │ │  │
│  │  │         │                                               │ │  │
│  │  │  Internal Network: beef-prod-network (bridge)          │ │  │
│  │  └────────────────────────────────────────────────────────┘ │  │
│  │         │                                                    │  │
│  │         ↓ Persistent Storage                                │  │
│  └─────────┼────────────────────────────────────────────────────┘  │
│            │                                                        │
│  ┌─────────▼─────────────────────────────────────────────────┐    │
│  │  Block Storage Volume (linode_volume)                      │    │
│  │  • Size: 10GB (expandable)                                 │    │
│  │  • Mount: /mnt/postgres-data                               │    │
│  │  • Format: ext4                                            │    │
│  │  • PostgreSQL data: /mnt/postgres-data/pgdata              │    │
│  │  • Persistent across instance rebuilds                     │    │
│  └─────────────────────────────────────────────────────────────┘    │
│                                                                      │
│  ┌──────────────────────────────────────────────────────────────┐  │
│  │  Object Storage (Linode S3-compatible)                       │  │
│  │  Region: es-mad (Madrid)                                     │  │
│  │  • Bucket: {domain}-telegram-media                           │  │
│  │  • Access: MinIO client (S3 API)                             │  │
│  │  • Content: Telegram photos, videos, documents              │  │
│  │  • Versioning: Enabled                                       │  │
│  │  • Lifecycle: 365 days (configurable)                        │  │
│  │  • CORS: Enabled                                             │  │
│  └──────────────────────────────────────────────────────────────┘  │
│                                                                      │
│  ┌──────────────────────────────────────────────────────────────┐  │
│  │  New Relic Infrastructure Monitoring (optional)              │  │
│  │  • Container: newrelic-infra                                 │  │
│  │  • Monitors: Host metrics, Docker containers, logs           │  │
│  │  • Region: EU or US (configurable)                           │  │
│  └──────────────────────────────────────────────────────────────┘  │
│                                                                      │
└──────────────────────────────────────────────────────────────────────┘

External Connections:
  ↕ Telegram Bot API (telegram-bot receives updates)
  ↕ User browsers (dashboard web interface)
  ↕ API clients (api-service REST endpoints)
```

### Data Flow

1. **Telegram Messages**:
   - Telegram Bot API → `telegram-bot` → `api-service` → `postgres`
   - Media files → `api-service` → Linode Object Storage (S3)

2. **Dashboard**:
   - User browser → `dashboard` → `postgres` (read-only queries)
   - Authentication via Telegram OAuth

3. **Storage**:
   - Database: PostGIS on persistent block storage volume
   - Media: Content-addressable storage in Object Storage bucket
   - Secrets: Mounted as read-only volumes in containers

4. **Security**:
   - PostgreSQL not exposed externally (internal Docker network only)
   - Firewall blocks all ports except 22, 80, 443
   - SSH key-only authentication, no password auth
   - Object Storage bucket: private ACL

## Related Documentation

- [Docker Compose Configuration](../docker-compose.prod.yml)
- [Secrets Setup](../secrets/README.md)
- [Database Migrations](../../apps/postgres/migrations/)
- [API Service Documentation](../../apps/api-service/README.md)
- [Dashboard Documentation](../../apps/dashboard/README.md)
- [Deployment Scripts](../../scripts/)

## Support

For issues or questions:
- Check [Troubleshooting](#troubleshooting) section above
- Review cloud-init logs: `sudo cat /var/log/cloud-init-output.log`
- Check container logs: `docker compose -f docker-compose.prod.yml logs`
- Linode documentation: https://www.linode.com/docs/
