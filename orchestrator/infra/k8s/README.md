# Kubernetes Deployment for Omniphi Validator Orchestrator

This directory contains Kubernetes manifests for deploying the Omniphi Validator Orchestrator to a production Kubernetes cluster.

## Prerequisites

- Kubernetes cluster (v1.24+)
- kubectl configured to access your cluster
- NGINX Ingress Controller (or AWS ALB Ingress Controller)
- cert-manager for TLS certificates (optional but recommended)
- Persistent volume provisioner
- Docker registry access for custom images

## Quick Start

### 1. Build and Push Docker Images

```bash
# Build backend image
cd ../../backend
docker build -t your-registry/omniphi-orchestrator-backend:latest .
docker push your-registry/omniphi-orchestrator-backend:latest

# Update image reference in backend-deployment.yaml
```

### 2. Update Configuration

#### Edit secrets.yaml
```bash
# IMPORTANT: Update all passwords and keys
vi secrets.yaml
```

#### Edit configmap.yaml
```bash
# Update chain RPC URLs, domain names, etc.
vi configmap.yaml
```

#### Edit ingress.yaml
```bash
# Update domain name and TLS configuration
vi ingress.yaml
```

### 3. Deploy to Kubernetes

```bash
# Create namespace
kubectl apply -f namespace.yaml

# Create secrets and configmap
kubectl apply -f secrets.yaml
kubectl apply -f configmap.yaml

# Deploy PostgreSQL
kubectl apply -f postgres-statefulset.yaml

# Wait for PostgreSQL to be ready
kubectl wait --for=condition=ready pod -l app=postgres -n omniphi-orchestrator --timeout=300s

# Deploy backend
kubectl apply -f backend-deployment.yaml

# Wait for backend to be ready
kubectl wait --for=condition=ready pod -l app=backend -n omniphi-orchestrator --timeout=300s

# Deploy ingress
kubectl apply -f ingress.yaml
```

### 4. Verify Deployment

```bash
# Check all pods are running
kubectl get pods -n omniphi-orchestrator

# Check services
kubectl get svc -n omniphi-orchestrator

# Check ingress
kubectl get ingress -n omniphi-orchestrator

# View backend logs
kubectl logs -f deployment/backend -n omniphi-orchestrator

# Check database connectivity
kubectl exec -it postgres-0 -n omniphi-orchestrator -- psql -U omniphi -d validator_orchestrator -c "SELECT version();"
```

## Configuration

### Secrets Management

For production, use external secret management instead of plain Kubernetes secrets:

#### Option 1: External Secrets Operator
```bash
helm repo add external-secrets https://charts.external-secrets.io
helm install external-secrets external-secrets/external-secrets -n external-secrets-system --create-namespace
```

Then integrate with AWS Secrets Manager, Azure Key Vault, or HashiCorp Vault.

#### Option 2: Sealed Secrets
```bash
helm repo add sealed-secrets https://bitnami-labs.github.io/sealed-secrets
helm install sealed-secrets sealed-secrets/sealed-secrets -n kube-system
```

### Storage Classes

Update `postgres-statefulset.yaml` to use your cluster's storage class:

```yaml
volumeClaimTemplates:
  - metadata:
      name: postgres-storage
    spec:
      storageClassName: fast-ssd  # Your storage class
      accessModes: ["ReadWriteOnce"]
      resources:
        requests:
          storage: 10Gi
```

Common storage classes:
- **AWS**: `gp3`, `gp2`, `io1`
- **GCP**: `pd-ssd`, `pd-standard`
- **Azure**: `managed-premium`, `managed-standard`

### Resource Limits

Adjust resource requests/limits based on your cluster capacity and workload:

**Backend** (`backend-deployment.yaml`):
```yaml
resources:
  requests:
    memory: "512Mi"
    cpu: "500m"
  limits:
    memory: "2Gi"
    cpu: "2000m"
```

**PostgreSQL** (`postgres-statefulset.yaml`):
```yaml
resources:
  requests:
    memory: "256Mi"
    cpu: "250m"
  limits:
    memory: "1Gi"
    cpu: "1000m"
```

### Horizontal Pod Autoscaling

Create HPA for backend:

```bash
kubectl autoscale deployment backend \
  --cpu-percent=70 \
  --min=2 \
  --max=10 \
  -n omniphi-orchestrator
```

Or use YAML:

```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: backend-hpa
  namespace: omniphi-orchestrator
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: backend
  minReplicas: 2
  maxReplicas: 10
  metrics:
    - type: Resource
      resource:
        name: cpu
        target:
          type: Utilization
          averageUtilization: 70
    - type: Resource
      resource:
        name: memory
        target:
          type: Utilization
          averageUtilization: 80
```

## Monitoring

### Prometheus ServiceMonitor

```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: backend-metrics
  namespace: omniphi-orchestrator
spec:
  selector:
    matchLabels:
      app: backend
  endpoints:
    - port: http
      path: /metrics
      interval: 30s
```

### Grafana Dashboard

Import dashboard from `../monitoring/grafana-dashboard.json`

## Backup and Restore

### Database Backup

```bash
# Create backup
kubectl exec postgres-0 -n omniphi-orchestrator -- \
  pg_dump -U omniphi validator_orchestrator | gzip > backup-$(date +%Y%m%d).sql.gz

# Restore backup
gunzip -c backup-20250119.sql.gz | \
  kubectl exec -i postgres-0 -n omniphi-orchestrator -- \
  psql -U omniphi validator_orchestrator
```

### Automated Backups with CronJob

```yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: postgres-backup
  namespace: omniphi-orchestrator
spec:
  schedule: "0 2 * * *"  # Daily at 2 AM
  jobTemplate:
    spec:
      template:
        spec:
          containers:
            - name: backup
              image: postgres:15-alpine
              command:
                - /bin/sh
                - -c
                - |
                  pg_dump -h postgres -U $POSTGRES_USER $POSTGRES_DB | \
                  gzip > /backups/backup-$(date +%Y%m%d-%H%M%S).sql.gz
              env:
                - name: POSTGRES_USER
                  valueFrom:
                    secretKeyRef:
                      name: orchestrator-secrets
                      key: POSTGRES_USER
                - name: POSTGRES_PASSWORD
                  valueFrom:
                    secretKeyRef:
                      name: orchestrator-secrets
                      key: POSTGRES_PASSWORD
                - name: POSTGRES_DB
                  valueFrom:
                    secretKeyRef:
                      name: orchestrator-secrets
                      key: POSTGRES_DB
              volumeMounts:
                - name: backups
                  mountPath: /backups
          volumes:
            - name: backups
              persistentVolumeClaim:
                claimName: postgres-backups
          restartPolicy: OnFailure
```

## Troubleshooting

### Pod Not Starting

```bash
# Check pod events
kubectl describe pod <pod-name> -n omniphi-orchestrator

# Check logs
kubectl logs <pod-name> -n omniphi-orchestrator

# Check previous logs (if pod crashed)
kubectl logs <pod-name> -n omniphi-orchestrator --previous
```

### Database Connection Issues

```bash
# Test database connectivity
kubectl run -it --rm debug --image=postgres:15-alpine --restart=Never -n omniphi-orchestrator -- \
  psql -h postgres -U omniphi -d validator_orchestrator

# Check database pod logs
kubectl logs postgres-0 -n omniphi-orchestrator
```

### Ingress Not Working

```bash
# Check ingress controller logs
kubectl logs -n ingress-nginx -l app.kubernetes.io/name=ingress-nginx

# Check ingress status
kubectl describe ingress orchestrator-ingress -n omniphi-orchestrator

# Test service directly
kubectl port-forward svc/backend 8000:8000 -n omniphi-orchestrator
curl http://localhost:8000/api/v1/health
```

## Cleanup

```bash
# Delete all resources
kubectl delete -f ingress.yaml
kubectl delete -f backend-deployment.yaml
kubectl delete -f postgres-statefulset.yaml
kubectl delete -f configmap.yaml
kubectl delete -f secrets.yaml
kubectl delete -f namespace.yaml

# Or delete entire namespace (WARNING: deletes all data)
kubectl delete namespace omniphi-orchestrator
```

## Production Checklist

- [ ] Update all secrets in `secrets.yaml`
- [ ] Configure external secret management
- [ ] Update domain names in `ingress.yaml`
- [ ] Configure TLS certificates (cert-manager)
- [ ] Set appropriate resource limits
- [ ] Configure HPA for backend
- [ ] Set up monitoring (Prometheus/Grafana)
- [ ] Configure log aggregation (ELK/Loki)
- [ ] Set up automated backups
- [ ] Configure network policies
- [ ] Enable pod security policies
- [ ] Set up disaster recovery plan
- [ ] Document runbook procedures
- [ ] Test failover scenarios

## Support

For issues or questions:
- GitHub Issues: https://github.com/omniphi/validator-orchestrator/issues
- Documentation: https://docs.omniphi.io
