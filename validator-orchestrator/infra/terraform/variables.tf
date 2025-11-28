# General Configuration
variable "environment" {
  description = "Environment name (dev, staging, production)"
  type        = string
  default     = "production"
}

variable "project_name" {
  description = "Project name for resource naming"
  type        = string
  default     = "omniphi-orchestrator"
}

variable "aws_region" {
  description = "AWS region"
  type        = string
  default     = "us-east-1"
}

# VPC Configuration
variable "vpc_cidr" {
  description = "CIDR block for VPC"
  type        = string
  default     = "10.0.0.0/16"
}

variable "availability_zones_count" {
  description = "Number of availability zones to use"
  type        = number
  default     = 2
}

# EC2 Configuration
variable "backend_instance_type" {
  description = "EC2 instance type for backend"
  type        = string
  default     = "t3.medium"
}

variable "backend_instance_count" {
  description = "Number of backend instances"
  type        = number
  default     = 2
}

variable "validator_host_instance_type" {
  description = "EC2 instance type for validator container hosts"
  type        = string
  default     = "t3.large"
}

variable "validator_host_count" {
  description = "Number of validator container hosts"
  type        = number
  default     = 3
}

variable "key_pair_name" {
  description = "EC2 key pair name for SSH access"
  type        = string
}

# RDS Configuration
variable "enable_rds" {
  description = "Enable RDS PostgreSQL (alternative to EC2-hosted PostgreSQL)"
  type        = bool
  default     = true
}

variable "db_instance_class" {
  description = "RDS instance class"
  type        = string
  default     = "db.t3.medium"
}

variable "db_allocated_storage" {
  description = "Allocated storage for RDS (GB)"
  type        = number
  default     = 100
}

variable "db_name" {
  description = "Database name"
  type        = string
  default     = "validator_orchestrator"
}

variable "db_username" {
  description = "Database master username"
  type        = string
  default     = "omniphi"
}

variable "db_password" {
  description = "Database master password"
  type        = string
  sensitive   = true
}

# Load Balancer Configuration
variable "enable_alb" {
  description = "Enable Application Load Balancer"
  type        = bool
  default     = true
}

variable "certificate_arn" {
  description = "ACM certificate ARN for HTTPS"
  type        = string
  default     = ""
}

# Docker Configuration
variable "docker_registry" {
  description = "Docker registry for images"
  type        = string
  default     = "docker.io"
}

variable "backend_image" {
  description = "Backend Docker image"
  type        = string
  default     = "omniphi/validator-orchestrator-backend:latest"
}

variable "validator_node_image" {
  description = "Validator node Docker image"
  type        = string
  default     = "omniphi/validator-node:latest"
}

# Chain Configuration
variable "omniphi_chain_id" {
  description = "Omniphi chain ID"
  type        = string
  default     = "omniphi-mainnet-1"
}

variable "omniphi_rpc_url" {
  description = "Omniphi RPC URL"
  type        = string
  default     = "http://localhost:26657"
}

# Monitoring
variable "enable_cloudwatch" {
  description = "Enable CloudWatch monitoring"
  type        = bool
  default     = true
}

# Tags
variable "additional_tags" {
  description = "Additional tags for all resources"
  type        = map(string)
  default     = {}
}
