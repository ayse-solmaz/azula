# Kubernetes (Scale / Tier C)

This is a **single-region** starter chart-in-a-file, not multi-region production.

```bash
docker build -t azula-api:local .
docker build -t azula-web:local -f web/Dockerfile .
kubectl apply -f deploy/k8s/azula.yaml
```

## Topology

- `azula-web` (nginx) → GraphQL `/graphql`, OIDC `/auth`, Stripe `/billing/webhook`
- Ingress also routes those prefixes to `azula-api` (so SSO/Stripe work even if nginx is skipped)
- `azula-api` ×2 with HPA 2–8
- `azula-mongo` (emptyDir — replace with a managed replica set before production)
- Shared uploads volume is emptyDir per pod — attach RWX (NFS/EFS) if API replicas must share MCP files

## Secrets

Edit `azula-secrets` before apply:

| Key | Purpose |
|-----|---------|
| `JWT_SECRET` | Session tokens |
| `MONGODB_URI` | Mongo connection |
| `STRIPE_*` | Pro checkout + webhook |
| `OIDC_*` | SSO (issuer + client) |

## Not included (still Scale-phase)

- Multi-region failover
- Mongo replica set / backups
- Dedicated inference GPU workers
- Redis job queue
