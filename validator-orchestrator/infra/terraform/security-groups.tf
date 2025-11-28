# Security Group for ALB
resource "aws_security_group" "alb" {
  count       = var.enable_alb ? 1 : 0
  name        = "${var.project_name}-alb-sg"
  description = "Security group for Application Load Balancer"
  vpc_id      = aws_vpc.main.id

  ingress {
    description = "HTTP from internet"
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

  ingress {
    description = "SSH from anywhere (restrict in production)"
    from_port   = 22
    to_port     = 22
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"] # TODO: Restrict to bastion or VPN
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
}

# Security Group for Validator Container Hosts
resource "aws_security_group" "validator_hosts" {
  name        = "${var.project_name}-validator-hosts-sg"
  description = "Security group for validator container hosts"
  vpc_id      = aws_vpc.main.id

  ingress {
    description     = "Docker API from backend"
    from_port       = 2375
    to_port         = 2376
    protocol        = "tcp"
    security_groups = [aws_security_group.backend.id]
  }

  ingress {
    description = "Validator P2P port range"
    from_port   = 26656
    to_port     = 26756 # Allow 100 validators per host
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  ingress {
    description = "Validator RPC port range"
    from_port   = 26657
    to_port     = 26757 # Allow 100 validators per host
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  ingress {
    description = "SSH from anywhere (restrict in production)"
    from_port   = 22
    to_port     = 22
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"] # TODO: Restrict to bastion or VPN
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
