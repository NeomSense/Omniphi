# RDS Subnet Group
resource "aws_db_subnet_group" "main" {
  count      = var.enable_rds ? 1 : 0
  name       = "${var.project_name}-db-subnet-group"
  subnet_ids = aws_subnet.private[*].id

  tags = merge(
    var.additional_tags,
    {
      Name = "${var.project_name}-db-subnet-group"
    }
  )
}

# RDS PostgreSQL Instance
resource "aws_db_instance" "main" {
  count                  = var.enable_rds ? 1 : 0
  identifier             = "${var.project_name}-db"
  engine                 = "postgres"
  engine_version         = "15.4"
  instance_class         = var.db_instance_class
  allocated_storage      = var.db_allocated_storage
  storage_type           = "gp3"
  storage_encrypted      = true

  db_name  = var.db_name
  username = var.db_username
  password = var.db_password
  port     = 5432

  db_subnet_group_name   = aws_db_subnet_group.main[0].name
  vpc_security_group_ids = [aws_security_group.rds[0].id]
  publicly_accessible    = false

  # Backup configuration
  backup_retention_period = 7
  backup_window          = "03:00-04:00"
  maintenance_window     = "mon:04:00-mon:05:00"

  # High availability
  multi_az = var.environment == "production" ? true : false

  # Monitoring
  enabled_cloudwatch_logs_exports = ["postgresql", "upgrade"]
  monitoring_interval             = 60
  monitoring_role_arn            = var.enable_cloudwatch ? aws_iam_role.rds_monitoring[0].arn : null

  # Deletion protection
  deletion_protection = var.environment == "production" ? true : false
  skip_final_snapshot = var.environment != "production"
  final_snapshot_identifier = var.environment == "production" ? "${var.project_name}-final-snapshot-${formatdate("YYYY-MM-DD-hhmm", timestamp())}" : null

  # Performance Insights
  performance_insights_enabled = true
  performance_insights_retention_period = 7

  # Auto minor version upgrade
  auto_minor_version_upgrade = true

  tags = merge(
    var.additional_tags,
    {
      Name = "${var.project_name}-rds"
    }
  )
}

# IAM Role for RDS Enhanced Monitoring
resource "aws_iam_role" "rds_monitoring" {
  count = var.enable_rds && var.enable_cloudwatch ? 1 : 0
  name  = "${var.project_name}-rds-monitoring-role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action = "sts:AssumeRole"
        Effect = "Allow"
        Principal = {
          Service = "monitoring.rds.amazonaws.com"
        }
      }
    ]
  })

  managed_policy_arns = ["arn:aws:iam::aws:policy/service-role/AmazonRDSEnhancedMonitoringRole"]

  tags = var.additional_tags
}

# RDS Read Replica (optional, for high-traffic scenarios)
# resource "aws_db_instance" "read_replica" {
#   count                  = var.enable_rds && var.environment == "production" ? 1 : 0
#   identifier             = "${var.project_name}-db-read-replica"
#   replicate_source_db    = aws_db_instance.main[0].identifier
#   instance_class         = var.db_instance_class
#   publicly_accessible    = false
#   skip_final_snapshot    = true
#   vpc_security_group_ids = [aws_security_group.rds[0].id]
#
#   tags = merge(
#     var.additional_tags,
#     {
#       Name = "${var.project_name}-rds-read-replica"
#     }
#   )
# }
