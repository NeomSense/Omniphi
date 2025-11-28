# Security Guide

This document outlines the security features, best practices, and configuration for the Omniphi Validator Orchestrator system.

## Table of Contents

- [Authentication & Authorization](#authentication--authorization)
- [Rate Limiting](#rate-limiting)
- [API Security](#api-security)
- [Database Security](#database-security)
- [Network Security](#network-security)
- [Key Management](#key-management)
- [Production Deployment](#production-deployment)
- [Security Checklist](#security-checklist)

## Authentication & Authorization

### JWT (JSON Web Token) Authentication

The orchestrator uses JWT tokens for API authentication. Tokens are issued for validators and contain:

- **Subject**: Validator wallet address
- **Node ID**: Optional validator node identifier
- **Expiration**: Configurable (default: 30 minutes)
- **Type**: `access` or `refresh`

#### Getting a Token

```bash
curl -X POST http://localhost:8000/api/v1/auth/token \
  -H "Content-Type: application/json" \
  -d '{
    "wallet_address": "omni1...",
    "node_id": "node123"
  }'
```

Response:
```json
{
  "access_token": "eyJ...",
  "refresh_token": "eyJ...",
  "token_type": "bearer",
  "expires_in": 1800
}
```

#### Using a Token

Include the token in the `Authorization` header:

```bash
curl http://localhost:8000/api/v1/validators/setup \
  -H "Authorization: Bearer eyJ..." \
  -H "Content-Type: application/json" \
  -d '{"wallet_address": "omni1..."}'
```

#### Refreshing a Token

```bash
curl -X POST http://localhost:8000/api/v1/auth/token/refresh \
  -H "Content-Type: application/json" \
  -d '{"refresh_token": "eyJ..."}'
```

### API Key Authentication

For external integrations and service-to-service communication, use API keys.

#### Generating an API Key

```bash
curl -X POST http://localhost:8000/api/v1/auth/api-key/generate \
  -H "X-API-Key: <master-api-key>"
```

**IMPORTANT**: Store the generated API key securely. It will only be shown once.

#### Using an API Key

Include the API key in the `X-API-Key` header:

```bash
curl http://localhost:8000/api/v1/validators/status \
  -H "X-API-Key: <your-api-key>"
```

### Security Configuration

In `.env` file:

```bash
# Generate with: python -c "import secrets; print(secrets.token_urlsafe(32))"
SECRET_KEY=<random-32-char-string>

# JWT expiration in minutes
ACCESS_TOKEN_EXPIRE_MINUTES=30

# Generate with: python -c "import secrets; print(secrets.token_hex(32))"
MASTER_API_KEY=<random-64-char-hex>
```

## Rate Limiting

The orchestrator implements rate limiting to prevent abuse and ensure fair usage.

### Configuration

```bash
# Enable/disable rate limiting
RATE_LIMIT_ENABLED=true

# Requests per minute per IP
RATE_LIMIT_PER_MINUTE=60

# Requests per hour per IP
RATE_LIMIT_PER_HOUR=1000
```

### Rate Limit Headers

Responses include rate limit information:

```
X-RateLimit-Limit: 60
X-RateLimit-Remaining: 59
X-RateLimit-Reset: 1634567890
```

### Exceeded Rate Limit

When rate limit is exceeded, you'll receive:

```json
{
  "error": "Rate limit exceeded",
  "retry_after": 42
}
```

HTTP Status: `429 Too Many Requests`

## API Security

### CORS (Cross-Origin Resource Sharing)

Configure allowed origins in `.env`:

```bash
BACKEND_CORS_ORIGINS=["https://validators.omniphi.xyz","https://app.omniphi.xyz"]
```

**Development**:
```bash
BACKEND_CORS_ORIGINS=["http://localhost:3000","http://localhost:5173"]
```

### Input Validation

All API endpoints validate input using Pydantic models:

- Type checking
- Format validation
- Required field enforcement
- Length constraints

### Error Handling

Errors are returned with appropriate HTTP status codes:

- `400 Bad Request`: Invalid input
- `401 Unauthorized`: Missing/invalid authentication
- `403 Forbidden`: Insufficient permissions
- `404 Not Found`: Resource not found
- `429 Too Many Requests`: Rate limit exceeded
- `500 Internal Server Error`: Server error

Sensitive information is never leaked in error messages.

## Database Security

### Connection Security

**Development (SQLite)**:
```bash
DATABASE_URL=sqlite:///./dev.db
```

**Production (PostgreSQL with SSL)**:
```bash
DATABASE_URL=postgresql://user:password@host:5432/db?sslmode=require
```

### Credentials

- Use strong passwords (16+ characters, mixed case, numbers, symbols)
- Rotate credentials regularly
- Never commit `.env` file to version control
- Use environment-specific credentials

### SQL Injection Prevention

- All queries use SQLAlchemy ORM
- Parameterized queries prevent SQL injection
- Input validation at API layer

### Encryption at Rest

For sensitive data in database:

1. Enable PostgreSQL encryption
2. Use encrypted volumes (AWS EBS encryption, GCP persistent disk encryption)
3. Implement application-level encryption for highly sensitive fields

## Network Security

### HTTPS/TLS

**Production**: Always use HTTPS with valid TLS certificates.

Configure reverse proxy (nginx example):

```nginx
server {
    listen 443 ssl http2;
    server_name api.validators.omniphi.xyz;

    ssl_certificate /path/to/cert.pem;
    ssl_certificate_key /path/to/key.pem;
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers HIGH:!aNULL:!MD5;

    location / {
        proxy_pass http://localhost:8000;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

### Firewall Rules

Recommended firewall configuration:

```bash
# Allow HTTPS from anywhere
allow 443/tcp from any

# Allow HTTP (for redirect to HTTPS)
allow 80/tcp from any

# Allow SSH from trusted IPs only
allow 22/tcp from <your-ip>

# Block all other incoming traffic
deny all incoming
```

### Docker Network Isolation

Validator containers run in isolated Docker network:

```yaml
networks:
  omniphi-validator-network:
    driver: bridge
    internal: false  # Set to true for complete isolation
```

## Key Management

### Consensus Keys

**Local Validator Desktop App**:

- Private keys stored in OS keychain (Windows Credential Manager, macOS Keychain, Linux Secret Service)
- Never transmitted over network
- Encrypted backups with password protection

**Cloud Validators**:

- Keys generated inside Docker containers
- Stored in encrypted volumes
- Access restricted to container process

### API Keys

**Storage**:

- Hashed before storing in database (bcrypt)
- Original key shown only once during generation
- Cannot be retrieved after initial generation

**Best Practices**:

1. Rotate API keys every 90 days
2. Use different keys for different environments
3. Revoke compromised keys immediately
4. Log API key usage for audit trails

## Production Deployment

### Environment Variables

**Required**:
- `SECRET_KEY`: JWT signing key (32+ random characters)
- `MASTER_API_KEY`: Admin API key (64 hex characters)
- `POSTGRES_PASSWORD`: Strong database password
- `DATABASE_URL`: Production database connection string

**Recommended**:
- `RATE_LIMIT_ENABLED=true`
- `DEBUG=false`
- `BACKEND_CORS_ORIGINS`: Restrict to production domains

### Docker Security

**Dockerfile best practices**:

```dockerfile
# Use non-root user
RUN useradd -m -u 1000 appuser
USER appuser

# Don't expose unnecessary ports
EXPOSE 8000

# Use secrets for sensitive data
RUN --mount=type=secret,id=db_password \
    echo "Password is in /run/secrets/db_password"
```

**Docker Compose secrets**:

```yaml
services:
  backend:
    environment:
      DATABASE_PASSWORD_FILE: /run/secrets/db_password
    secrets:
      - db_password

secrets:
  db_password:
    external: true
```

### Kubernetes Security

**Use secrets**:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: orchestrator-secrets
type: Opaque
stringData:
  secret-key: <base64-encoded-secret>
  api-key: <base64-encoded-api-key>
```

**Network policies**:

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: backend-network-policy
spec:
  podSelector:
    matchLabels:
      app: orchestrator-backend
  policyTypes:
    - Ingress
    - Egress
  ingress:
    - from:
        - podSelector:
            matchLabels:
              app: frontend
      ports:
        - protocol: TCP
          port: 8000
```

### Monitoring & Logging

**Security Events to Monitor**:

- Failed authentication attempts
- Rate limit violations
- Invalid API key usage
- Database connection errors
- Unusual request patterns

**Recommended Tools**:

- Application monitoring: Sentry, DataDog
- Log aggregation: ELK Stack, Splunk
- Security scanning: Snyk, OWASP Dependency-Check
- Penetration testing: OWASP ZAP, Burp Suite

## Security Checklist

### Pre-Production

- [ ] Change `SECRET_KEY` to random 32+ character string
- [ ] Change `MASTER_API_KEY` to random 64 character hex
- [ ] Use strong PostgreSQL password
- [ ] Set `DEBUG=false`
- [ ] Configure CORS to allow only production domains
- [ ] Enable rate limiting (`RATE_LIMIT_ENABLED=true`)
- [ ] Enable PostgreSQL SSL/TLS
- [ ] Set up HTTPS with valid certificates
- [ ] Configure firewall rules
- [ ] Set up monitoring and alerting
- [ ] Implement log aggregation
- [ ] Review and test backup/restore procedures
- [ ] Scan dependencies for vulnerabilities
- [ ] Run security penetration tests

### Post-Deployment

- [ ] Monitor authentication logs for suspicious activity
- [ ] Review rate limit violations
- [ ] Check database connection logs
- [ ] Verify HTTPS is enforced
- [ ] Audit API key usage
- [ ] Test backup restoration
- [ ] Review security alerts
- [ ] Update dependencies regularly

### Monthly

- [ ] Rotate API keys
- [ ] Review access logs
- [ ] Update dependencies
- [ ] Security audit
- [ ] Test disaster recovery

### Quarterly

- [ ] Rotate database credentials
- [ ] Rotate JWT secret key
- [ ] Security penetration testing
- [ ] Update security documentation

## Reporting Security Issues

If you discover a security vulnerability, please email security@omniphi.io with:

- Description of the vulnerability
- Steps to reproduce
- Potential impact
- Suggested fix (if available)

**Do NOT** open a public GitHub issue for security vulnerabilities.

## Additional Resources

- [OWASP Top 10](https://owasp.org/www-project-top-ten/)
- [FastAPI Security](https://fastapi.tiangolo.com/tutorial/security/)
- [Docker Security Best Practices](https://docs.docker.com/engine/security/)
- [Kubernetes Security](https://kubernetes.io/docs/concepts/security/)
- [PostgreSQL Security](https://www.postgresql.org/docs/current/security.html)
