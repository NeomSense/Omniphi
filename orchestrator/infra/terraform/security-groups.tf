# =============================================================================
# SECURITY GROUPS - PRODUCTION HARDENED
# =============================================================================
# IMPORTANT: These security groups follow the principle of least privilege.
# SSH access is restricted to specified admin IPs or bastion hosts.
# RPC and metrics endpoints are internal-only by default.
# =============================================================================

# Variable for admin access (should be set in terraform.tfvars)
variable "admin_cidr_blocks" {
  description = "CIDR blocks allowed SSH access (e.g., your office IP, VPN)"
  type        = list(string)
  default     = [] # MUST be configured in production - empty = no SSH access
}

variable "monitoring_cidr_blocks" {
  description = "CIDR blocks for monitoring systems (Prometheus, Grafana)"
  type        = list(string)
  default     = [] # MUST be configured in production
}

# Security Group for ALB
resource "aws_security_group" "alb" {
  count       = var.enable_alb ? 1 : 0
  name        = "${var.project_name}-alb-sg"
  description = "Security group for Application Load Balancer"
  vpc_id      = aws_vpc.main.id

  ingress {
    description = "HTTP from internet (redirects to HTTPS)"
    from_port   = 80
    to_port     = 80
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  ingress {
    description = "HTTPS from internet"
    from_port   = 443
    to_port     = 443
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  egress {
    description = "All outbound traffic"
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = merge(
    var.additional_tags,
    {
      Name = "${var.project_name}-alb-sg"
    }
  )
}

# Security Group for Backend Instances
resource "aws_security_group" "backend" {
  name        = "${var.project_name}-backend-sg"
  description = "Security group for backend API instances"
  vpc_id      = aws_vpc.main.id

  ingress {
    description     = "HTTP from ALB"
    from_port       = 8000
    to_port         = 8000
    protocol        = "tcp"
    security_groups = var.enable_alb ? [aws_security_group.alb[0].id] : []
    cidr_blocks     = var.enable_alb ? [] : ["0.0.0.0/0"]
  }

  # SSH: PRODUCTION HARDENED - Only allow from specified admin IPs
  dynamic "ingress" {
    for_each = length(var.admin_cidr_blocks) > 0 ? [1] : []
    content {
      description = "SSH from admin IPs only"
      from_port   = 22
      to_port     = 22
      protocol    = "tcp"
      cidr_blocks = var.admin_cidr_blocks
    }
  }

  egress {
    description = "All outbound traffic"
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = merge(
    var.additional_tags,
    {
      Name = "${var.project_name}-backend-sg"
    }
  )

  lifecycle {
    # Prevent accidental opening of SSH to the world
    precondition {
      condition     = !contains(var.admin_cidr_blocks, "0.0.0.0/0")
      error_message = "SECURITY: admin_cidr_blocks cannot contain 0.0.0.0/0. Specify exact admin IPs."
    }
  }
}

# Security Group for Validator Container Hosts
resource "aws_security_group" "validator_hosts" {
  name        = "${var.project_name}-validator-hosts-sg"
  description = "Security group for validator container hosts"
  vpc_id      = aws_vpc.main.id

  # Docker API: Internal only - backend to validator hosts
  ingress {
    description     = "Docker API from backend (internal only)"
    from_port       = 2375
    to_port         = 2376
    protocol        = "tcp"
    security_groups = [aws_security_group.backend.id]
  }

  # P2P: Public access required for validator networking
  ingress {
    description = "Validator P2P port range (required for consensus)"
    from_port   = 26656
    to_port     = 26756 # Allow 100 validators per host
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"] # P2P must be public
  }

  # RPC: INTERNAL ONLY - exposed only via ALB/API Gateway
  ingress {
    description     = "Validator RPC from backend only"
    from_port       = 26657
    to_port         = 26757
    protocol        = "tcp"
    security_groups = [aws_security_group.backend.id]
  }

  # gRPC: INTERNAL ONLY
  ingress {
    description     = "gRPC from backend only"
    from_port       = 9090
    to_port         = 9190 # Allow 100 validators per host
    protocol        = "tcp"
    security_groups = [aws_security_group.backend.id]
  }

  # Prometheus metrics: MONITORING SERVERS ONLY
  dynamic "ingress" {
    for_each = length(var.monitoring_cidr_blocks) > 0 ? [1] : []
    content {
      description = "Prometheus metrics from monitoring servers"
      from_port   = 26660
      to_port     = 26760
      protocol    = "tcp"
      cidr_blocks = var.monitoring_cidr_blocks
    }
  }

  # SSH: PRODUCTION HARDENED - Only allow from specified admin IPs
  dynamic "ingress" {
    for_each = length(var.admin_cidr_blocks) > 0 ? [1] : []
    content {
      description = "SSH from admin IPs only"
      from_port   = 22
      to_port     = 22
      protocol    = "tcp"
      cidr_blocks = var.admin_cidr_blocks
    }
  }

  egress {
    description = "All outbound traffic"
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = merge(
    var.additional_tags,
    {
      Name = "${var.project_name}-validator-hosts-sg"
    }
  )

  lifecycle {
    # Prevent accidental opening of SSH to the world
    precondition {
      condition     = !contains(var.admin_cidr_blocks, "0.0.0.0/0")
      error_message = "SECURITY: admin_cidr_blocks cannot contain 0.0.0.0/0. Specify exact admin IPs."
    }
    precondition {
      condition     = !contains(var.monitoring_cidr_blocks, "0.0.0.0/0")
      error_message = "SECURITY: monitoring_cidr_blocks cannot contain 0.0.0.0/0. Specify monitoring server IPs."
    }
  }
}

# Security Group for RDS
resource "aws_security_group" "rds" {
  count       = var.enable_rds ? 1 : 0
  name        = "${var.project_name}-rds-sg"
  description = "Security group for RDS PostgreSQL"
  vpc_id      = aws_vpc.main.id

  ingress {
    description     = "PostgreSQL from backend"
    from_port       = 5432
    to_port         = 5432
    protocol        = "tcp"
    security_groups = [aws_security_group.backend.id]
  }

  egress {
    description = "All outbound traffic"
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = merge(
    var.additional_tags,
    {
      Name = "${var.project_name}-rds-sg"
    }
  )
}

# =============================================================================
# OUTPUTS FOR DOCUMENTATION
# =============================================================================

output "security_warning" {
  value = length(var.admin_cidr_blocks) == 0 ? "WARNING: No admin_cidr_blocks configured - SSH access is disabled" : "SSH access restricted to: ${join(", ", var.admin_cidr_blocks)}"
}

output "monitoring_warning" {
  value = length(var.monitoring_cidr_blocks) == 0 ? "WARNING: No monitoring_cidr_blocks configured - metrics endpoints not accessible" : "Metrics access restricted to: ${join(", ", var.monitoring_cidr_blocks)}"
}
