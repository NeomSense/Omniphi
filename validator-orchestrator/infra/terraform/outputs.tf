# VPC Outputs
output "vpc_id" {
  description = "ID of the VPC"
  value       = aws_vpc.main.id
}

output "public_subnet_ids" {
  description = "IDs of public subnets"
  value       = aws_subnet.public[*].id
}

output "private_subnet_ids" {
  description = "IDs of private subnets"
  value       = aws_subnet.private[*].id
}

# ALB Outputs
output "alb_dns_name" {
  description = "DNS name of the Application Load Balancer"
  value       = var.enable_alb ? aws_lb.main[0].dns_name : null
}

output "alb_zone_id" {
  description = "Zone ID of the Application Load Balancer"
  value       = var.enable_alb ? aws_lb.main[0].zone_id : null
}

# RDS Outputs
output "rds_endpoint" {
  description = "RDS instance endpoint"
  value       = var.enable_rds ? aws_db_instance.main[0].endpoint : null
  sensitive   = true
}

output "rds_address" {
  description = "RDS instance address"
  value       = var.enable_rds ? aws_db_instance.main[0].address : null
}

# Database Connection String
output "database_url" {
  description = "Database connection URL"
  value       = var.enable_rds ? "postgresql://${var.db_username}:${var.db_password}@${aws_db_instance.main[0].endpoint}/${var.db_name}" : null
  sensitive   = true
}

# Security Group IDs
output "backend_security_group_id" {
  description = "Security group ID for backend instances"
  value       = aws_security_group.backend.id
}

output "validator_hosts_security_group_id" {
  description = "Security group ID for validator hosts"
  value       = aws_security_group.validator_hosts.id
}

output "rds_security_group_id" {
  description = "Security group ID for RDS"
  value       = var.enable_rds ? aws_security_group.rds[0].id : null
}

# NAT Gateway IPs (for whitelisting)
output "nat_gateway_ips" {
  description = "Elastic IPs of NAT Gateways (for whitelisting outbound traffic)"
  value       = aws_eip.nat[*].public_ip
}

# Deployment Information
output "deployment_instructions" {
  description = "Next steps for deployment"
  value = <<-EOT
    Infrastructure provisioned successfully!

    Next steps:
    1. Update DNS records to point to ALB:
       - ALB DNS: ${var.enable_alb ? aws_lb.main[0].dns_name : "N/A"}
       - Create CNAME: api.omniphi.io -> ${var.enable_alb ? aws_lb.main[0].dns_name : "N/A"}

    2. Database connection details:
       - Endpoint: ${var.enable_rds ? aws_db_instance.main[0].endpoint : "N/A"}
       - Database: ${var.db_name}
       - Username: ${var.db_username}

    3. Deploy backend application:
       - Build Docker image
       - Push to registry
       - Deploy via docker-compose or Kubernetes

    4. Configure environment variables on backend instances:
       - DATABASE_URL=<use 'database_url' output>
       - OMNIPHI_CHAIN_ID=${var.omniphi_chain_id}
       - OMNIPHI_RPC_URL=${var.omniphi_rpc_url}

    5. Security:
       - Update SSH access in security groups (currently open to 0.0.0.0/0)
       - Rotate database password
       - Configure SSL/TLS certificates
       - Set up CloudWatch alarms

    For detailed setup instructions, see infra/terraform/README.md
  EOT
}
