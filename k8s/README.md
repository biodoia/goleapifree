# Kubernetes Deployment

Deploy GoLeapAI su Kubernetes.

## Prerequisiti

- Kubernetes cluster (minikube, GKE, EKS, AKS, etc.)
- kubectl configurato
- Docker image di GoLeapAI

## Quick Start

```bash
# 1. Build Docker image
docker build -t goleapai:latest .

# 2. Tag per registry (opzionale)
docker tag goleapai:latest your-registry/goleapai:latest
docker push your-registry/goleapai:latest

# 3. Deploy tutto
kubectl apply -f k8s/

# 4. Verifica status
kubectl get pods -n goleapai
kubectl get services -n goleapai
```

## Deploy Componenti Singoli

```bash
# Namespace
kubectl apply -f k8s/deployment.yaml

# PostgreSQL
kubectl apply -f k8s/postgres.yaml

# Redis
kubectl apply -f k8s/redis.yaml
```

## Configurazione

### Secrets

Modifica i secrets prima del deploy:

```bash
# Crea secret per PostgreSQL
kubectl create secret generic goleapai-secrets \
  --from-literal=postgres-password=your-secure-password \
  --from-literal=jwt-secret=your-jwt-secret \
  -n goleapai
```

### ConfigMap

Modifica `deployment.yaml` per configurare:
- Database connection
- Redis settings
- Routing strategy
- Monitoring options

### Ingress

Modifica `deployment.yaml` per configurare:
- Domain name
- TLS certificates
- Rate limiting

## Scaling

### Manuale

```bash
# Scale replicas
kubectl scale deployment goleapai --replicas=5 -n goleapai
```

### Auto-scaling

L'HPA è già configurato in `deployment.yaml`:
- Min replicas: 3
- Max replicas: 10
- Target CPU: 70%
- Target Memory: 80%

## Monitoring

### Logs

```bash
# Logs di tutti i pod
kubectl logs -f -l app=goleapai -n goleapai

# Logs di un pod specifico
kubectl logs -f <pod-name> -n goleapai
```

### Metrics

```bash
# Metrics via Prometheus
kubectl port-forward svc/goleapai 9090:9090 -n goleapai
# Accedi a http://localhost:9090/metrics
```

### Status

```bash
# Status generale
kubectl get all -n goleapai

# Describe deployment
kubectl describe deployment goleapai -n goleapai

# Events
kubectl get events -n goleapai
```

## Troubleshooting

### Pod non parte

```bash
# Check pod status
kubectl get pods -n goleapai
kubectl describe pod <pod-name> -n goleapai
kubectl logs <pod-name> -n goleapai
```

### Database connection issues

```bash
# Check PostgreSQL
kubectl get pods -l app=postgres -n goleapai
kubectl logs -l app=postgres -n goleapai

# Test connection
kubectl run psql-test --rm -it --image=postgres:16-alpine -n goleapai -- \
  psql postgresql://goleapai:goleapai@postgres:5432/goleapai
```

### Redis connection issues

```bash
# Check Redis
kubectl get pods -l app=redis -n goleapai
kubectl logs -l app=redis -n goleapai

# Test connection
kubectl run redis-test --rm -it --image=redis:7-alpine -n goleapai -- \
  redis-cli -h redis ping
```

## Upgrade

```bash
# Update image
kubectl set image deployment/goleapai goleapai=goleapai:new-version -n goleapai

# Rollback
kubectl rollout undo deployment/goleapai -n goleapai

# History
kubectl rollout history deployment/goleapai -n goleapai
```

## Cleanup

```bash
# Delete tutto
kubectl delete namespace goleapai

# Oppure file per file
kubectl delete -f k8s/
```

## Production Checklist

- [ ] Secrets configurati correttamente
- [ ] Persistent volumes per PostgreSQL e Redis
- [ ] Ingress con TLS configurato
- [ ] Resource limits impostati
- [ ] HPA configurato
- [ ] Monitoring attivo
- [ ] Backup database configurato
- [ ] High availability (min 3 replicas)
- [ ] Network policies (opzionale)
- [ ] Pod security policies (opzionale)
