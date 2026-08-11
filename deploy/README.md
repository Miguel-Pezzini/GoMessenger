# GoMessenger — Kubernetes Deployment (Cloud Edition)

Deploy GoMessenger on a local **kind** cluster using Helm. Milestone **M1**: cluster + Helm → gateway responds to `GET /healthz`.

## Prerequisites

- [Docker](https://docs.docker.com/get-docker/)
- [kind](https://kind.sigs.k8s.io/docs/user/quick-start/)
- [kubectl](https://kubernetes.io/docs/tasks/tools/)
- [Helm 3](https://helm.sh/docs/intro/install/)

## Quick start (one-liner flow)

From the repository root:

```bash
# 1. Build all images
cd backend && ./scripts/docker-build-all.sh && cd ..
docker build -t gomessenger/frontend:local \
  --build-arg VITE_GATEWAY_URL=http://localhost:8080 \
  --build-arg VITE_WS_URL=ws://localhost:8080/ws \
  -f frontend/Dockerfile frontend/

# 2. Create kind cluster and load images
./deploy/scripts/kind-up.sh

# 3. Install metrics-server + Helm chart
chmod +x deploy/scripts/helm-install.sh
./deploy/scripts/helm-install.sh

# 4. Smoke test
curl http://localhost:8080/healthz
```

Open the UI at **http://localhost:30517**.

## Architecture

```text
Host (localhost)
  ├── :8080  → kind NodePort 30080 → gateway Service
  └── :30517 → kind NodePort 30517 → frontend Service

Cluster (namespace: default)
  ├── Infra: redis, mongo (single pod, multi-db), minio
  └── Apps: auth, friends, gateway, websocket, chat, presence,
            logging, notification, media, frontend
```

### MongoDB

A **single MongoDB pod** hosts all databases (`userdb`, `friends_db`, `chatdb`, `logging_db`, `media_db`). This matches the Cloud Edition trade-off for simpler local ops (see `docs/cloud-edition/TODO.md`).

### Service discovery

Microservices use Kubernetes DNS names (`http://auth:50051`, `redis:6379`, etc.) via ConfigMap `gomessenger-config`.

### Secrets

Default dev secrets are in `values.yaml`. Override for non-local use:

```bash
helm upgrade --install gomessenger ./deploy/helm/gomessenger \
  --set secrets.jwtSecret='your-production-secret' \
  --set secrets.internalServiceToken='your-internal-token'
```

## Helm chart

| Path | Purpose |
|------|---------|
| `deploy/helm/gomessenger/Chart.yaml` | Chart metadata |
| `deploy/helm/gomessenger/values.yaml` | Defaults (images, replicas, HPA, probes) |
| `deploy/helm/gomessenger/templates/` | K8s manifests |

### Useful commands

```bash
# Watch pods
kubectl get pods -w

# Gateway logs
kubectl logs deploy/gateway -f

# HPA status (after metrics-server is running)
kubectl get hpa

# Upgrade after image rebuild
cd backend && ./scripts/docker-build-all.sh && cd ..
kind load docker-image gomessenger/gateway:local --name gomessenger
helm upgrade gomessenger ./deploy/helm/gomessenger
kubectl rollout restart deployment/gateway
```

## Probes

All Go microservices expose:

| Probe | Path | Purpose |
|-------|------|---------|
| Liveness | `GET /healthz` | Restart unhealthy pods |
| Readiness | `GET /readyz` | Traffic only when deps are OK |
| Startup | `GET /readyz` | `chat` and `media` (slow init) |

## HPA

Horizontal Pod Autoscalers are enabled for **gateway**, **websocket**, and **chat**:

| Service | Min | Max | CPU target |
|---------|-----|-----|------------|
| gateway | 2 | 5 | 70% |
| websocket | 2 | 5 | 70% |
| chat | 1 | 3 | 70% |

Requires **metrics-server** (`deploy/kind/metrics-server.yaml`), installed automatically by `helm-install.sh`.

Load tests (k6) in Fase 4 will validate scaling under load (Milestone M2).

## kind port mappings

`deploy/kind/cluster-config.yaml` maps:

| Host | NodePort | Service |
|------|----------|---------|
| `localhost:8080` | 30080 | gateway |
| `localhost:30517` | 30517 | frontend |

Recreate the cluster after changing port mappings:

```bash
kind delete cluster --name gomessenger
./deploy/scripts/kind-up.sh
./deploy/scripts/helm-install.sh
```

## Troubleshooting

**Pods stuck in `Pending` (PVC)** — kind uses a default StorageClass. If PVCs do not bind, check `kubectl get pvc` and ensure the local-path provisioner is available.

**`ImagePullBackOff`** — Images are loaded locally via `kind load docker-image`. Rebuild and reload:

```bash
backend/scripts/docker-build-all.sh
./deploy/scripts/kind-up.sh
```

**Gateway `readyz` fails** — An upstream service is not ready. Check `kubectl get pods` and logs for auth, websocket, chat, etc.

**CORS errors in browser** — Frontend must be opened at `http://localhost:30517` (matches `gatewayAllowedOrigin` in values). Rebuild the frontend image if gateway URL changed.

## Related docs

- [Cloud Edition plan](../docs/cloud-edition/TODO.md)
- [Compose smoke test](./docs/smoke-test.md)
- [Backend AGENTS.md](../backend/AGENTS.md)
