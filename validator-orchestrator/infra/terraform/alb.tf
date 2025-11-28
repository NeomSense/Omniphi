# Application Load Balancer
resource "aws_lb" "main" {
  count              = var.enable_alb ? 1 : 0
  name               = "${var.project_name}-alb"
  internal           = false
  load_balancer_type = "application"
  security_groups    = [aws_security_group.alb[0].id]
  subnets            = aws_subnet.public[*].id

  enable_deletion_protection = var.environment == "production" ? true : false
  enable_http2              = true
  enable_cross_zone_load_balancing = true

  tags = merge(
    var.additional_tags,
    {
      Name = "${var.project_name}-alb"
    }
  )
}

# Target Group for Backend
resource "aws_lb_target_group" "backend" {
  count       = var.enable_alb ? 1 : 0
  name        = "${var.project_name}-backend-tg"
  port        = 8000
  protocol    = "HTTP"
  vpc_id      = aws_vpc.main.id
  target_type = "instance"

  health_check {
    enabled             = true
    healthy_threshold   = 2
    unhealthy_threshold = 3
    timeout             = 5
    interval            = 30
    path                = "/api/v1/health"
    protocol            = "HTTP"
    matcher             = "200"
  }

  deregistration_delay = 30

  stickiness {
    type            = "lb_cookie"
    cookie_duration = 86400
    enabled         = true
  }

  tags = merge(
    var.additional_tags,
    {
      Name = "${var.project_name}-backend-tg"
    }
  )
}

# HTTPS Listener
resource "aws_lb_listener" "https" {
  count             = var.enable_alb && var.certificate_arn != "" ? 1 : 0
  load_balancer_arn = aws_lb.main[0].arn
  port              = "443"
  protocol          = "HTTPS"
  ssl_policy        = "ELBSecurityPolicy-TLS-1-2-2017-01"
  certificate_arn   = var.certificate_arn

  default_action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.backend[0].arn
  }
}

# HTTP Listener (redirects to HTTPS)
resource "aws_lb_listener" "http" {
  count             = var.enable_alb ? 1 : 0
  load_balancer_arn = aws_lb.main[0].arn
  port              = "80"
  protocol          = "HTTP"

  default_action {
    type = var.certificate_arn != "" ? "redirect" : "forward"

    # Redirect to HTTPS if certificate is configured
    dynamic "redirect" {
      for_each = var.certificate_arn != "" ? [1] : []
      content {
        port        = "443"
        protocol    = "HTTPS"
        status_code = "HTTP_301"
      }
    }

    # Forward to backend if no certificate
    target_group_arn = var.certificate_arn == "" ? aws_lb_target_group.backend[0].arn : null
  }
}

# WAF (Web Application Firewall) - Optional
# resource "aws_wafv2_web_acl" "main" {
#   count       = var.enable_alb && var.environment == "production" ? 1 : 0
#   name        = "${var.project_name}-waf"
#   description = "WAF for Omniphi Orchestrator"
#   scope       = "REGIONAL"
#
#   default_action {
#     allow {}
#   }
#
#   # Rate limiting rule
#   rule {
#     name     = "RateLimitRule"
#     priority = 1
#
#     override_action {
#       none {}
#     }
#
#     statement {
#       rate_based_statement {
#         limit              = 2000
#         aggregate_key_type = "IP"
#       }
#     }
#
#     visibility_config {
#       cloudwatch_metrics_enabled = true
#       metric_name               = "RateLimitRule"
#       sampled_requests_enabled  = true
#     }
#   }
#
#   # AWS Managed Rules - Common Rule Set
#   rule {
#     name     = "AWSManagedRulesCommonRuleSet"
#     priority = 2
#
#     override_action {
#       none {}
#     }
#
#     statement {
#       managed_rule_group_statement {
#         name        = "AWSManagedRulesCommonRuleSet"
#         vendor_name = "AWS"
#       }
#     }
#
#     visibility_config {
#       cloudwatch_metrics_enabled = true
#       metric_name               = "AWSManagedRulesCommonRuleSetMetric"
#       sampled_requests_enabled  = true
#     }
#   }
#
#   visibility_config {
#     cloudwatch_metrics_enabled = true
#     metric_name               = "${var.project_name}-waf"
#     sampled_requests_enabled  = true
#   }
#
#   tags = var.additional_tags
# }
#
# # Associate WAF with ALB
# resource "aws_wafv2_web_acl_association" "main" {
#   count        = var.enable_alb && var.environment == "production" ? 1 : 0
#   resource_arn = aws_lb.main[0].arn
#   web_acl_arn  = aws_wafv2_web_acl.main[0].arn
# }
