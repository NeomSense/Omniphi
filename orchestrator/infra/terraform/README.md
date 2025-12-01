# Terraform Infrastructure for Omniphi Validator Orchestrator

Provisions AWS infrastructure for the Omniphi Validator Orchestrator.

## Architecture

- **VPC**: Multi-AZ with public/private subnets
- **RDS PostgreSQL**: Managed database with backups and monitoring
- **Application Load Balancer**: HTTPS termination and traffic distribution
- **Security Groups**: Network isolation and access control
- **NAT Gateways**: Outbound internet for private subnets
- **CloudWatch**: Monitoring and logging

## Prerequisites

- AWS CLI configured (`aws configure`)
- Terraform >= 1.0
- EC2 key pair created
- ACM certificate (for HTTPS)

## Quick Start

### 1. Configure Variables

Create `terraform.tfvars`:

```hcl
# Required
key_pair_name = "your-ec2-keypair"
db_password   = "your-secure-db-password-min-16-chars"

# Optional
environment         = "production"
aws_region         = "us-east-1"
certificate_arn    = "arn:aws:acm:region:account:certificate/xxx"
omniphi_chain_id   = "omniphi-mainnet-1"
omniphi_rpc_url    = "https://rpc.omniphi.io:26657"
```

### 2. Initialize Terraform

```bash
terraform init
```

### 3. Plan Deployment

```bash
terraform plan
```

### 4. Apply Infrastructure

```bash
terraform apply
```

### 5. Get Outputs

```bash
terraform output
terraform output -json > outputs.json
```

## Configuration

### State Backend (Recommended for Production)

Update `main.tf` to use S3 backend:

```hcl
terraform {
  backend "s3" {
    bucket         = "omniphi-terraform-state"
    key            = "validator-orchestrator/terraform.tfstate"
    region         = "us-east-1"
    dynamodb_table = "omniphi-terraform-locks"
    encrypt        = true
  }
}
```

Create S3 bucket and DynamoDB table:

```bash
aws s3 mb s3://omniphi-terraform-state --region us-east-1
aws s3api put-bucket-versioning \
  --bucket omniphi-terraform-state \
  --versioning-configuration Status=Enabled

aws dynamodb create-table \
  --table-name omniphi-terraform-locks \
  --attribute-definitions AttributeName=LockID,AttributeType=S \
  --key-schema AttributeName=LockID,KeyType=HASH \
  --billing-mode PAY_PER_REQUEST \
  --region us-east-1
```

### Key Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `environment` | Environment name | `production` |
| `aws_region` | AWS region | `us-east-1` |
| `vpc_cidr` | VPC CIDR block | `10.0.0.0/16` |
| `db_instance_class` | RDS instance type | `db.t3.medium` |
| `backend_instance_type` | Backend EC2 type | `t3.medium` |
| `certificate_arn` | ACM certificate | `""` |
| `enable_rds` | Enable RDS PostgreSQL | `true` |
| `enable_alb` | Enable ALB | `true` |

## Outputs

Key outputs:

- `alb_dns_name`: Load balancer DNS
- `database_url`: Database connection string (sensitive)
- `vpc_id`: VPC identifier
- `nat_gateway_ips`: NAT gateway public IPs

## Post-Deployment

### 1. Update DNS

Point your domain to the ALB:

```bash
# Get ALB DNS
terraform output alb_dns_name

# Create CNAME record
api.omniphi.io -> <alb_dns_name>
```

### 2. Deploy Backend

Use docker-compose or Kubernetes with the infrastructure outputs.

### 3. Security Hardening

- Restrict SSH access in security groups
- Rotate database password
- Enable AWS GuardDuty
- Configure CloudWatch alarms

## Costs

Estimated monthly costs (us-east-1):

- RDS db.t3.medium: ~$60
- 2x t3.medium (backend): ~$60
- 3x t3.large (validators): ~$190
- ALB: ~$25
- NAT Gateways (2): ~$65
- Data transfer: Variable

**Total: ~$400-500/month**

## Cleanup

```bash
# Destroy all resources
terraform destroy

# WARNING: This deletes all data!
```

## Support

- Issues: https://github.com/omniphi/validator-orchestrator/issues
- Docs: https://docs.omniphi.io
