# Deployment Guide

Complete guide for deploying the Omniphi Validator Orchestrator system to production.

## Table of Contents

- [Overview](#overview)
- [Prerequisites](#prerequisites)
- [Local Development](#local-development)
- [Docker Deployment](#docker-deployment)
- [Kubernetes Deployment](#kubernetes-deployment)
- [AWS Deployment (Terraform)](#aws-deployment-terraform)
- [Production Checklist](#production-checklist)
- [Monitoring & Maintenance](#monitoring--maintenance)

## Overview

The Omniphi Validator Orchestrator consists of:

1. **Backend API** (FastAPI) - Orchestration server
2. **PostgreSQL Database** - Persistent storage
3. **Local Validator App** (Electron) - Desktop client
4. **Validator Nodes** (Docker containers) - Blockchain validators

## Prerequisites

### Required

- Docker & Docker Compose (20.10+)
- Python 3.11+
- Node.js 18+
- PostgreSQL 15+ (or use Docker)
- Git

### Optional (for cloud deployment)

- Kubernetes cluster (1.25+)
- kubectl configured
- Terraform (1.5+)
- AWS CLI (for AWS deployment)
- Domain name with DNS access

## Local Development

### 1. Clone Repository

```bash
git clone https://github.com/omniphi/validator-orchestrator.git
cd validator-orchestrator
```

### 2. Backend Setup

```bash
cd backend

# Create virtual environment
python -m venv venv
source venv/bin/activate  # On Windows: venv\Scripts\activate

# Install dependencies
pip install -r requirements.txt

# Configure environment
cp .env.example .env
# Edit .env with your settings

# Run database migrations
alembic upgrade head

# Start development server
uvicorn app.main:app --reload --port 8000
```

Backend will be available at: `http://localhost:8000`

API Documentation: `http://localhost:8000/docs`

### 3. Desktop App Setup

```bash
cd local-validator-app

# Install dependencies
npm install

# Add posd binary
# Place posd (or posd.exe on Windows) in bin/ directory

# Start development
npm run dev
```

### 4. Database (Development)

**Option 1: SQLite (Quick Start)**

In `.env`:
```bash
DATABASE_URL=sqlite:///./dev.db
```

**Option 2: PostgreSQL (Recommended)**

```bash
# Start PostgreSQL with Docker
docker run -d \
  --name postgres \
  -e POSTGRES_PASSWORD=omniphi_dev \
  -e POSTGRES_USER=omniphi \
  -e POSTGRES_DB=validator_orchestrator \
  -p 5432:5432 \
  postgres:15-alpine

# Or install PostgreSQL locally
# Ubuntu: sudo apt-get install postgresql
# macOS: brew install postgresql
```

In `.env`:
```bash
POSTGRES_USER=omniphi
POSTGRES_PASSWORD=omniphi_dev
POSTGRES_SERVER=localhost
POSTGRES_PORT=5432
POSTGRES_DB=validator_orchestrator
```

## Docker Deployment

### Quick Start

```bash
# Clone repository
git clone https://github.com/omniphi/validator-orchestrator.git
cd validator-orchestrator

# Configure environment
cp backend/.env.example backend/.env
# Edit backend/.env with production values

# Start all services
docker-compose up -d

# Check logs
docker-compose logs -f

# Stop services
docker-compose down
```

Services:
- Backend API: `http://localhost:8000`
- PostgreSQL: `localhost:5432`
- API Docs: `http://localhost:8000/docs`

### Production Docker Compose

For production, update `docker-compose.yml`:

```yaml
services:
  postgres:
    restart: always
    environment:
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD}  # Use strong password
    volumes:
      - postgres_data:/var/lib/postgresql/data
      - ./backups:/backups  # Backup location

  backend:
    restart: always
    environment:
      DEBUG: "false"
      SECRET_KEY: ${SECRET_KEY}  # Generate random key
      MASTER_API_KEY: ${MASTER_API_KEY}  # Generate random key
      DATABASE_URL: postgresql://omniphi:${POSTGRES_PASSWORD}@postgres:5432/validator_orchestrator
    depends_on:
      postgres:
        condition: service_healthy

volumes:
  postgres_data:
    driver: local
```

### Build Custom Images

```bash
# Build backend image
cd backend
docker build -t omniphi/orchestrator-backend:v1.0.0 .

# Tag for registry
docker tag omniphi/orchestrator-backend:v1.0.0 your-registry.com/omniphi/orchestrator-backend:v1.0.0

# Push to registry
docker push your-registry.com/omniphi/orchestrator-backend:v1.0.0
```

## Kubernetes Deployment

### Prerequisites

- Kubernetes cluster (1.25+)
- kubectl configured
- Helm (optional, for PostgreSQL)

### 1. Create Namespace

```bash
kubectl apply -f infra/k8s/namespace.yaml
```

### 2. Configure Secrets

```bash
# Generate secrets
SECRET_KEY=$(python -c "import secrets; print(secrets.token_urlsafe(32))")
API_KEY=$(python -c "import secrets; print(secrets.token_hex(32))")
DB_PASSWORD=$(python -c "import secrets; print(secrets.token_urlsafe(16))")

# Create secret
kubectl create secret generic orchestrator-secrets \
  --from-literal=secret-key=$SECRET_KEY \
  --from-literal=api-key=$API_KEY \
  --from-literal=postgres-password=$DB_PASSWORD \
  -n omniphi-orchestrator
```

### 3. Deploy PostgreSQL

```bash
kubectl apply -f infra/k8s/postgres-statefulset.yaml
```

Wait for PostgreSQL to be ready:
```bash
kubectl wait --for=condition=ready pod -l app=postgres -n omniphi-orchestrator --timeout=300s
```

### 4. Configure ConfigMap

Edit `infra/k8s/configmap.yaml` with your chain configuration, then:

```bash
kubectl apply -f infra/k8s/configmap.yaml
```

### 5. Deploy Backend

```bash
kubectl apply -f infra/k8s/backend-deployment.yaml
```

### 6. Configure Ingress

**Update Ingress for your domain:**

Edit `infra/k8s/ingress.yaml`:

```yaml
spec:
  tls:
    - hosts:
        - api.validators.yourdomain.com  # Update this
      secretName: orchestrator-tls
  rules:
    - host: api.validators.yourdomain.com  # Update this
```

**Create TLS certificate:**

```bash
# Option 1: cert-manager (recommended)
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.13.0/cert-manager.yaml

# Option 2: Manual certificate
kubectl create secret tls orchestrator-tls \
  --cert=path/to/tls.crt \
  --key=path/to/tls.key \
  -n omniphi-orchestrator
```

**Deploy Ingress:**

```bash
kubectl apply -f infra/k8s/ingress.yaml
```

### 7. Verify Deployment

```bash
# Check all pods are running
kubectl get pods -n omniphi-orchestrator

# Check services
kubectl get svc -n omniphi-orchestrator

# Check ingress
kubectl get ingress -n omniphi-orchestrator

# View logs
kubectl logs -f deployment/orchestrator-backend -n omniphi-orchestrator
```

### 8. Test API

```bash
# Get ingress IP/hostname
INGRESS_HOST=$(kubectl get ingress orchestrator-ingress -n omniphi-orchestrator -o jsonpath='{.status.loadBalancer.ingress[0].hostname}')

# Test health endpoint
curl https://$INGRESS_HOST/api/v1/health
```

## AWS Deployment (Terraform)

### Prerequisites

- AWS Account with credentials configured
- Terraform installed (1.5+)
- AWS CLI installed

### 1. Configure AWS Credentials

```bash
aws configure
# Enter Access Key ID, Secret Access Key, Region
```

### 2. Initialize Terraform

```bash
cd infra/terraform

# Initialize
terraform init
```

### 3. Configure Variables

Create `terraform.tfvars`:

```hcl
# General
project_name = "omniphi-orchestrator"
environment  = "production"
aws_region   = "us-east-1"

# Networking
vpc_cidr            = "10.0.0.0/16"
availability_zones  = ["us-east-1a", "us-east-1b"]

# Database
db_instance_class   = "db.t3.medium"
db_allocated_storage = 100
db_username         = "omniphi"
db_name             = "validator_orchestrator"

# Backend
backend_instance_type = "t3.medium"
backend_desired_count = 2

# Domain (optional)
domain_name = "validators.yourdomain.com"
```

### 4. Review Plan

```bash
terraform plan
```

### 5. Deploy Infrastructure

```bash
terraform apply

# Confirm with 'yes'
```

This will create:
- VPC with public/private subnets across 2 AZs
- Application Load Balancer with HTTPS
- RDS PostgreSQL Multi-AZ
- Security Groups
- Backend infrastructure

### 6. Get Outputs

```bash
# Get deployment information
terraform output
```

Important outputs:
- `alb_dns_name`: Load balancer DNS
- `rds_endpoint`: Database endpoint
- `vpc_id`: VPC identifier

### 7. Configure DNS

Point your domain to the ALB DNS name:

```bash
# Get ALB DNS
ALB_DNS=$(terraform output -raw alb_dns_name)

# Create CNAME record
# validators.yourdomain.com -> $ALB_DNS
```

### 8. Deploy Application

After infrastructure is ready, deploy the application:

```bash
# SSH into backend instances (if using EC2)
# Or deploy to ECS/EKS as configured

# Build and push Docker image
docker build -t your-registry/orchestrator-backend:v1.0.0 backend/
docker push your-registry/orchestrator-backend:v1.0.0

# Deploy to EC2/ECS/EKS
# (Specific steps depend on your deployment method)
```

### 9. Cleanup (when needed)

```bash
terraform destroy
# Confirm with 'yes'
```

## Production Checklist

### Security

- [ ] Generate strong `SECRET_KEY` (32+ characters)
- [ ] Generate strong `MASTER_API_KEY` (64 hex characters)
- [ ] Use strong database password
- [ ] Set `DEBUG=false`
- [ ] Configure CORS for production domains only
- [ ] Enable rate limiting
- [ ] Use HTTPS/TLS everywhere
- [ ] Configure firewall rules
- [ ] Review [SECURITY.md](SECURITY.md)

### Database

- [ ] Use PostgreSQL (not SQLite)
- [ ] Enable SSL/TLS connections
- [ ] Configure backups (daily recommended)
- [ ] Set up monitoring
- [ ] Configure connection pooling
- [ ] Enable query logging (for debugging)

### Application

- [ ] Set appropriate `ACCESS_TOKEN_EXPIRE_MINUTES`
- [ ] Configure correct `OMNIPHI_CHAIN_ID`
- [ ] Set correct RPC/REST/gRPC URLs
- [ ] Configure Docker network
- [ ] Set resource limits (CPU/memory)
- [ ] Configure health checks

### Infrastructure

- [ ] Use reverse proxy (nginx, Traefik)
- [ ] Configure load balancing (if multi-instance)
- [ ] Set up auto-scaling (optional)
- [ ] Configure persistent volumes
- [ ] Set restart policies

### Monitoring

- [ ] Application monitoring (Sentry, DataDog)
- [ ] Log aggregation (ELK, Splunk)
- [ ] Uptime monitoring
- [ ] Performance monitoring
- [ ] Error tracking
- [ ] Security monitoring

### Backup & Recovery

- [ ] Database backups configured
- [ ] Backup retention policy defined
- [ ] Restore procedure tested
- [ ] Disaster recovery plan documented

## Monitoring & Maintenance

### Health Checks

```bash
# Application health
curl https://api.validators.yourdomain.com/api/v1/health

# Database connection
docker exec postgres pg_isready

# Kubernetes health
kubectl get pods -n omniphi-orchestrator
```

### Logs

**Docker:**
```bash
docker-compose logs -f backend
docker-compose logs -f postgres
```

**Kubernetes:**
```bash
kubectl logs -f deployment/orchestrator-backend -n omniphi-orchestrator
kubectl logs -f statefulset/postgres -n omniphi-orchestrator
```

### Database Backup

**Manual Backup:**
```bash
# Docker
docker exec postgres pg_dump -U omniphi validator_orchestrator > backup_$(date +%Y%m%d).sql

# Kubernetes
kubectl exec statefulset/postgres -n omniphi-orchestrator -- pg_dump -U omniphi validator_orchestrator > backup_$(date +%Y%m%d).sql
```

**Automated Backups:**

Add to cron:
```bash
0 2 * * * /path/to/backup-script.sh
```

### Database Restore

```bash
# Docker
docker exec -i postgres psql -U omniphi validator_orchestrator < backup_20241120.sql

# Kubernetes
kubectl exec -i statefulset/postgres -n omniphi-orchestrator -- psql -U omniphi validator_orchestrator < backup_20241120.sql
```

### Scaling

**Docker Compose:**
```bash
docker-compose up -d --scale backend=3
```

**Kubernetes:**
```bash
kubectl scale deployment orchestrator-backend --replicas=3 -n omniphi-orchestrator
```

### Updates

**Backend Update:**

```bash
# Build new version
docker build -t omniphi/orchestrator-backend:v1.1.0 backend/

# Update docker-compose.yml
# Change image to v1.1.0

# Restart
docker-compose up -d

# Kubernetes
kubectl set image deployment/orchestrator-backend \
  backend=omniphi/orchestrator-backend:v1.1.0 \
  -n omniphi-orchestrator
```

**Database Migrations:**

```bash
# Docker
docker-compose exec backend alembic upgrade head

# Kubernetes
kubectl exec deployment/orchestrator-backend -n omniphi-orchestrator -- alembic upgrade head
```

### Troubleshooting

**Backend won't start:**
1. Check logs: `docker-compose logs backend`
2. Verify database connection
3. Check environment variables
4. Verify port availability

**Database connection errors:**
1. Check PostgreSQL is running
2. Verify credentials in `.env`
3. Check network connectivity
4. Review PostgreSQL logs

**Rate limiting issues:**
1. Adjust `RATE_LIMIT_PER_MINUTE` in `.env`
2. Check client IP identification
3. Review rate limit logs

**SSL/TLS issues:**
1. Verify certificate validity
2. Check certificate chain
3. Verify domain DNS
4. Review reverse proxy configuration

## Additional Resources

- [API Documentation](http://localhost:8000/docs)
- [Security Guide](SECURITY.md)
- [Architecture Overview](README.md)
- [Local Validator App](local-validator-app/README.md)

## Support

For deployment issues:
- GitHub Issues: https://github.com/omniphi/validator-orchestrator/issues
- Discord: https://discord.gg/omniphi
- Email: support@omniphi.io
