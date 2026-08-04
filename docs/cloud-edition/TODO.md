# GoMessenger Cloud Edition — Plano de Evolução (Opção A)

Plano para evoluir o GoMessenger para uma versão **production-grade** em Kubernetes local (kind), **100% gratuita e local**.

**Estimativa total:** 8–12 semanas part-time.

---

## Estado atual vs. objetivo (gap analysis)

| Área | Estado atual | Objetivo Cloud Edition | Gap |
|------|--------------|------------------------|-----|
| **Microserviços** | 9 serviços Go HTTP (`auth`, `friends`, `gateway`, `websocket`, `chat`, `presence`, `logging`, `notification`, `media`) + Vue 3 frontend | Mesmo stack, production-grade em K8s | Sem containerização dos apps; `cmd/dev` roda tudo com `go run` |
| **Mensageria** | **Redis Streams + Pub/Sub** (não RabbitMQ) | DLQ + resiliência | Sem DLQ explícito; falhas de persistência ficam em pending e são reclaimadas via `XAutoClaim` (~30s), sem stream de dead-letter |
| **gRPC** | Não usado (HTTP REST + WebSocket) | — | Item do objetivo não se aplica ao código atual |
| **Docker** | `docker-compose.yml` só para **infra** (Redis, 5× MongoDB, MinIO) + profile `observability` | Dockerização completa | **Zero Dockerfile** no repo |
| **Kubernetes / Helm** | Não existe | Deploy com Helm, HPA, probes | Nada em `helm/`, `k8s/`, `.github/` |
| **Health / readiness** | `GET /healthz`, `GET /readyz`, `GET /metrics` via `internal/platform/observability` em todos os serviços | Probes K8s | Endpoints existem, mas **não há liveness separado de readiness**; gateway checa upstreams via HTTP |
| **Observabilidade** | Prometheus client + dashboard Grafana (`backend/observability/`) | Prometheus + Grafana (+ Cloud opcional) | Scrape config aponta a `host.docker.internal` (dev local); sem ServiceMonitor/Helm; sem OpenTelemetry |
| **WebSocket multi-réplica** | Hub local `map[userID]`; cada pod subscreve Pub/Sub e entrega só se o user está local | Múltiplas réplicas | **Funciona sem sticky session** para fan-out (Pub/Sub broadcast + entrega local), mas **1 conexão por userID** (sem multi-device) |
| **HPA / scaling consumers** | `consumerName()` usa `addr` (`:8082`) ou nome fixo (`logging-consumer`) | HPA 2→5 pods | Nomes de consumer duplicados entre réplicas — **pré-requisito para escalar chat/notification/logging** |
| **CI/CD** | Scripts `backend/scripts/test_*.sh`; 9 integration tests | GitHub Actions → ghcr.io → kind | Pipeline inexistente |
| **Load test** | Não existe | k6 + métricas documentadas | Sem `k6/`, sem cenários de carga |
| **Testes de resiliência** | Integration tests com stack isolada (`docker-compose.test.yml`) | Kill pod + recovery time | Sem automação de chaos/resiliência |
| **Segurança prod** | `JWT_SECRET` default, origins configuráveis mas backlog de hardening | Production-grade | Ver `backend/TODO.md` e `AGENTS.md` |
| **Frontend** | Vite dev server; URLs hardcoded `localhost:8080` | Deploy no cluster | Sem Dockerfile/nginx; sem config de gateway via env |

**Decisão crítica — RabbitMQ vs Redis:** o projeto **não usa RabbitMQ**. Para DLQ, recomendo **Redis Streams DLQ** (stream `*.dlq`, mover após N retries) em vez de migrar a RabbitMQ — menor risco, zero novo componente. Migrar a RabbitMQ = **Fase 0** (refactor arquitetural grande).

---

## Riscos e decisões pendentes

| Decisão | Recomendação | Justificativa |
|---------|--------------|---------------|
| **kind vs minikube** | **kind** | Leve, multi-node fácil, padrão em CI (GitHub Actions + `kind load docker-image`). Minikube é útil para addons, mas mais pesado para pipeline automatizado. |
| **Sticky sessions WebSocket** | **Não obrigatório** para fan-out | Pub/Sub chega em todos os pods websocket; entrega é local. Sticky session só ajuda se quiser otimizar (menos mensagens "vazias" por pod). |
| **Sticky session Gateway→WebSocket** | **Não necessário** | Gateway faz reverse proxy HTTP; conexão WS fica no pod websocket escolhido pelo Service K8s. |
| **DLQ** | **Redis Streams DLQ** (não RabbitMQ) | Alinhado ao código; RabbitMQ = nova infra + rewrite dos producers/consumers. |
| **6 instâncias MongoDB** | **Manter separadas no Helm** (dev realism) ou **1 MongoDB multi-db** (simplifica ops) | Trade-off: fidelidade vs complexidade. Para Cloud Edition local, 1 deployment Mongo com múltiplos databases é aceitável. |
| **Load test no cluster local** | k6 no **host** apontando a NodePort/Ingress do gateway | Evita rodar k6 dentro do cluster (métricas de rede mais limpas). |
| **Grafana Cloud** | **Opcional** na Fase 4 | `remote_write` do Prometheus local → Grafana Cloud free tier para retenção; Grafana local basta para milestones iniciais. |
| **Ordem crítica** | Containerização → K8s base → probes/HPA → DLQ/consumers → observability scrape K8s → k6 → CI | CI precisa de imagens + Helm; HPA precisa de métricas + consumer names únicos. |

---

## Milestones verificáveis

| # | Milestone | Critério de sucesso |
|---|-----------|-------------------|
| **M1** | Cluster + Helm sobe tudo | `kind create cluster` + `helm install gomessenger ./deploy/helm/gomessenger` → gateway responde `GET /healthz` |
| **M2** | HPA escala sob carga | k6 em gateway/websocket → HPA escala gateway ou websocket de 2→5 pods; métricas visíveis |
| **M3** | Recovery < 3s | `kubectl delete pod` em websocket ou chat → cliente reconecta; mensagem entregue em < 3s (medido) |
| **M4** | CI verde | GitHub Actions: test → build → push ghcr.io → deploy kind → smoke test |

---

## Fase 1 — Containerização e cluster local (2–3 semanas)

### Pré-requisitos (antes de Dockerizar)

- [ ] **Auditar e corrigir consumer names para scaling** — usar `HOSTNAME` ou `POD_NAME` em chat, notification e logging
  - **Arquivos:** `backend/services/chat/internal/stream_server.go`, `backend/services/notification/internal/app.go`, `backend/services/logging/internal/stream_server.go`
  - **Dependências:** nenhuma
  - **Esforço:** S
  - **Skills:** Redis Streams, distributed consumers
  - **Done:** cada réplica registra consumer único; integration tests passam

- [ ] **Validar suite de testes como gate** — `./scripts/test_all.sh` verde antes de cada fase
  - **Arquivos:** `backend/scripts/test_all.sh`
  - **Dependências:** Docker para integration
  - **Esforço:** S
  - **Done:** unit + integration passam no CI local

### Containerização backend

- [ ] **Criar Dockerfile multi-stage para serviços Go (template compartilhado)**
  - **Arquivos:** `backend/Dockerfile` (ou `backend/docker/Dockerfile.service`) + build args `SERVICE=auth|chat|...`
  - **Dependências:** pré-requisito consumer names
  - **Esforço:** M
  - **Skills:** Docker multi-stage, Go cross-compile
  - **Done:** `docker build` produz imagem < 50MB; container inicia e responde `/healthz`

- [ ] **Dockerfile para cada serviço (ou matrix build com SERVICE arg)** — 9 imagens: auth, friends, gateway, websocket, chat, presence, logging, notification, media
  - **Arquivos:** `backend/services/*/cmd/` (entrypoints já existem)
  - **Dependências:** Dockerfile template
  - **Esforço:** M
  - **Done:** 9 imagens buildam localmente

- [ ] **Variáveis de ambiente para URLs internas K8s-ready** — garantir que upstream URLs usam hostnames de Service (não `localhost`)
  - **Arquivos:** `backend/.env_example`, `backend/internal/platform/config/`
  - **Dependências:** nenhuma
  - **Esforço:** S
  - **Done:** serviços iniciam com `REDIS_ADDR=redis:6379`, `AUTH_UPSTREAM_URL=http://auth:50051`, etc.

- [ ] **Estender `docker-compose.yml` com profile `apps`** — rodar todos os microserviços + infra em Compose (espelho do K8s)
  - **Arquivos:** `backend/docker-compose.yml`, opcional `backend/docker-compose.apps.yml`
  - **Dependências:** Dockerfiles
  - **Esforço:** M
  - **Done:** `docker compose --profile apps up` → chat E2E funciona via gateway `:8080`

- [ ] **Script `scripts/docker-build-all.sh`** — build paralelo de todas as imagens com tag `gomessenger/<service>:local`
  - **Arquivos:** `backend/scripts/docker-build-all.sh`
  - **Dependências:** Dockerfiles
  - **Esforço:** S
  - **Done:** um comando builda tudo

### Containerização frontend

- [ ] **Configurar gateway URL via env no frontend** — `VITE_GATEWAY_URL` / `VITE_WS_URL`
  - **Arquivos:** `frontend/src/app/chat/api.ts`, `frontend/vite.config.ts`, `frontend/.env.example`
  - **Dependências:** nenhuma
  - **Esforço:** S
  - **Done:** frontend aponta a gateway configurável sem editar código

- [ ] **Dockerfile frontend (nginx + static build)**
  - **Arquivos:** `frontend/Dockerfile`, `frontend/nginx.conf`
  - **Dependências:** env configurável
  - **Esforço:** S
  - **Done:** container serve UI e proxy opcional ao gateway

### Cluster local (kind)

- [ ] **Instalar e documentar kind** — cluster single-node + registry local (`kind create cluster`, `kind load docker-image`)
  - **Arquivos:** `deploy/kind/cluster-config.yaml`, `deploy/scripts/kind-up.sh`
  - **Dependências:** Docker
  - **Esforço:** S
  - **Skills:** kind, local K8s
  - **Done:** cluster criado; imagens locais carregadas

- [ ] **Smoke test manual no Compose** — registrar/login, WS, enviar mensagem, attachment
  - **Arquivos:** `deploy/docs/smoke-test.md`
  - **Dependências:** compose apps profile
  - **Esforço:** S
  - **Done:** fluxo completo documentado e reproduzível

---

## Fase 2 — Kubernetes e Helm (2–3 semanas)

### Infra Helm (subcharts ou manifests)

- [ ] **Estrutura Helm chart `deploy/helm/gomessenger/`** — Chart.yaml, values.yaml, templates
  - **Arquivos:** `deploy/helm/gomessenger/**`
  - **Dependências:** Fase 1 Dockerfiles
  - **Esforço:** L
  - **Skills:** Helm, K8s manifests

- [ ] **Deploy Redis** — Deployment + Service (+ PVC se persistência desejada)
  - **Dependências:** chart base
  - **Esforço:** S
  - **Done:** pods conectam via `redis:6379`

- [ ] **Deploy MongoDB** — decisão: 1 pod multi-db **ou** 5 Deployments (chat, user, friends, logging, media)
  - **Dependências:** chart base
  - **Esforço:** M
  - **Done:** cada serviço conecta ao URI correto

- [ ] **Deploy MinIO** — Deployment + Service + bucket init Job
  - **Dependências:** chart base
  - **Esforço:** M
  - **Done:** media service faz upload/download

- [ ] **ConfigMaps + Secrets** — JWT, internal token, MinIO keys via Secret; URLs via ConfigMap
  - **Arquivos:** `values.yaml`, templates `secret.yaml`, `configmap.yaml`
  - **Dependências:** infra
  - **Esforço:** M
  - **Skills:** K8s secrets management

### App deployments

- [ ] **Deployment + Service para cada microserviço** (9 serviços)
  - **Arquivos:** `templates/deployment-auth.yaml`, etc.
  - **Dependências:** infra + imagens
  - **Esforço:** L
  - **Done:** todos os pods `Ready`; `readyz` passa

- [ ] **Gateway Ingress ou NodePort** — expor `:8080` para host
  - **Arquivos:** `templates/ingress.yaml` ou `nodeport-gateway.yaml`
  - **Dependências:** gateway deployment
  - **Esforço:** S
  - **Done:** `curl http://localhost:<port>/healthz` OK

- [ ] **Frontend Deployment** — nginx servindo build estático
  - **Dependências:** frontend Dockerfile
  - **Esforço:** S

### Probes

- [ ] **Liveness: `GET /healthz`** em todos os Deployments
  - **Arquivos:** templates deployments
  - **Dependências:** deployments
  - **Esforço:** S
  - **Done:** probe configurada; pod unhealthy reinicia

- [ ] **Readiness: `GET /readyz`** — já implementado com checks de Redis/Mongo/upstream
  - **Arquivos:** templates deployments
  - **Dependências:** deployments
  - **Esforço:** S
  - **Done:** pod não recebe tráfego até deps OK

- [ ] **Startup probe para serviços com init lento** (media + MinIO, chat consumer group)
  - **Dependências:** deployments
  - **Esforço:** S

### HPA

- [ ] **Instalar metrics-server no kind** — `kubectl apply` manifest oficial
  - **Arquivos:** `deploy/kind/metrics-server.yaml`
  - **Esforço:** S

- [ ] **HPA para gateway** — CPU target 70%, min 2, max 5
  - **Arquivos:** `templates/hpa-gateway.yaml`
  - **Dependências:** metrics-server
  - **Esforço:** S
  - **Skills:** HPA, resource limits

- [ ] **HPA para websocket** — CPU + opcional custom metric `gomessenger_websocket_active_connections`
  - **Arquivos:** `templates/hpa-websocket.yaml`
  - **Dependências:** metrics-server, Prometheus adapter (opcional)
  - **Esforço:** M
  - **Done:** Milestone M2 parcial — escala sob carga WS

- [ ] **Resource requests/limits** em todos os pods — necessário para HPA
  - **Esforço:** M
  - **Done:** `kubectl describe hpa` mostra métricas

- [ ] **Decidir HPA para chat** — escalar consumers exige consumer names únicos (Fase 1)
  - **Esforço:** M

### Milestone M1

- [ ] **Documentar `helm install` one-liner** em `deploy/README.md`
  - **Done:** `kind create cluster` + `helm install` → sistema operacional

---

## Fase 3 — Resiliência e mensageria (1–2 semanas)

### Redis Streams DLQ (substitui RabbitMQ do objetivo original)

- [ ] **Definir política DLQ** — max retries (ex. 5), `claimMinIdle`, streams `chat.message.created.dlq`, `audit.logs.dlq`, etc.
  - **Arquivos:** `backend/internal/platform/redis/dlq.go` (novo), config env
  - **Dependências:** Fase 2 deploy funcional
  - **Esforço:** M
  - **Skills:** Redis Streams, error handling

- [ ] **Implementar DLQ no chat stream consumer** — após N falhas de persistência, XADD na DLQ + XACK original
  - **Arquivos:** `backend/services/chat/internal/stream_server.go`
  - **Dependências:** política DLQ
  - **Esforço:** M
  - **Done:** mensagem inválida vai à DLQ; não fica em pending infinito

- [ ] **DLQ para notification e logging consumers**
  - **Arquivos:** `backend/services/notification/internal/stream_server.go`, `backend/services/logging/internal/stream_server.go`
  - **Esforço:** M

- [ ] **Métricas DLQ** — `gomessenger_stream_dlq_messages_total`
  - **Arquivos:** `backend/internal/platform/observability/observability.go`
  - **Esforço:** S

- [ ] **Admin/replay endpoint ou script** — reprocessar DLQ manualmente
  - **Esforço:** M

### WebSocket multi-réplica

- [ ] **Validar fan-out com 2+ réplicas websocket** — teste integration ou script
  - **Arquivos:** `backend/tests/integration/` ou `deploy/scripts/ws-multi-replica-test.sh`
  - **Dependências:** Helm com `websocket.replicas: 2`
  - **Esforço:** M
  - **Done:** user A e B em pods diferentes recebem mensagens

- [ ] **(Opcional) Per-connection writer queues** — backlog em `backend/TODO.md`; evita slow client bloquear hub
  - **Arquivos:** `backend/services/websocket/internal/http_handler.go`
  - **Esforço:** L

### Testes de falha

- [ ] **Script resiliência: kill pod websocket** — medir tempo até reconexão + entrega
  - **Arquivos:** `deploy/scripts/resilience-kill-pod.sh`
  - **Dependências:** cluster + frontend/k6 client
  - **Esforço:** M
  - **Done:** Milestone M3 — recovery < 3s documentado

- [ ] **Script: kill pod chat** — mensagens em stream são reclaimadas e entregues
  - **Esforço:** M

- [ ] **Teste: Redis down** — pods ficam NotReady; recovery quando Redis volta
  - **Esforço:** M

- [ ] **Integration test: consumer group após pod restart**
  - **Arquivos:** `backend/tests/integration/`
  - **Esforço:** M

---

## Fase 4 — Observabilidade e load test (1–2 semanas)

### Prometheus no cluster

- [ ] **Helm subchart ou manifests Prometheus** — scrape via K8s service discovery (não `host.docker.internal`)
  - **Arquivos:** `deploy/helm/gomessenger/templates/prometheus-*.yaml`, `backend/observability/prometheus/prometheus-k8s.yml`
  - **Dependências:** Fase 2
  - **Esforço:** M
  - **Skills:** Prometheus, K8s SD

- [ ] **Service annotations ou PodMonitor** — `prometheus.io/scrape: "true"` em cada Service
  - **Esforço:** S

- [ ] **Grafana no cluster** — provisionar dashboard existente `gomessenger-observability.json`
  - **Arquivos:** `backend/observability/grafana/**`, Helm templates
  - **Dependências:** Prometheus
  - **Esforço:** M
  - **Done:** dashboard mostra WS connections, chat stream lag, HTTP latency

- [ ] **(Opcional) Grafana Cloud remote_write** — free tier para retenção
  - **Esforço:** S

### Métricas faltantes

- [ ] **Métricas notification + logging stream consumers** — espelhar chat (lag, pending, processing)
  - **Arquivos:** observability package + stream servers
  - **Dependências:** `backend/TODO.md` item parcialmente feito para gateway/ws/chat
  - **Esforço:** M

### k6 load tests

- [ ] **Estrutura `load/k6/`** — scripts, thresholds, README
  - **Arquivos:** `load/k6/auth.js`, `chat-ws.js`, `smoke.js`
  - **Dependências:** gateway exposto
  - **Esforço:** M
  - **Skills:** k6, load testing

- [ ] **Cenário: registro + login + WS connect** — ramping VUs
  - **Done:** relatório com p95 latency, error rate

- [ ] **Cenário: mensagens chat sustentadas** — 100–500 VUs, medir throughput
  - **Dependências:** cenário auth
  - **Esforço:** M

- [ ] **Cenário: attachments upload** — carga em media + gateway
  - **Esforço:** M

- [ ] **Correlacionar k6 com Grafana** — screenshots/métricas em `docs/cloud-edition/benchmarks.md`
  - **Done:** números reais documentados (RPS, p95, CPU por pod)

- [ ] **Rodar k6 durante HPA test** — alimenta Milestone M2
  - **Esforço:** S

---

## Fase 5 — CI/CD (1 semana)

- [ ] **Workflow `.github/workflows/ci.yml`** — lint, `test_unit.sh`, `test_integration.sh`
  - **Arquivos:** `.github/workflows/ci.yml`
  - **Dependências:** testes estáveis
  - **Esforço:** M
  - **Skills:** GitHub Actions

- [ ] **Workflow build + push ghcr.io** — matrix 9 serviços + frontend; tags `ghcr.io/<user>/gomessenger-<service>:<sha>`
  - **Arquivos:** `.github/workflows/build-push.yml`
  - **Dependências:** Dockerfiles
  - **Esforço:** M
  - **Done:** imagens públicas no ghcr.io (free)

- [ ] **Workflow deploy kind** — kind no runner, `helm upgrade --install`, smoke test
  - **Arquivos:** `.github/workflows/deploy-kind.yml`
  - **Dependências:** Helm chart, build workflow
  - **Esforço:** L
  - **Skills:** kind em CI, Helm deploy

- [ ] **Smoke test no CI** — curl healthz + integration smoke (register, send message)
  - **Esforço:** M
  - **Done:** Milestone M4 — pipeline verde

- [ ] **Cache Go modules + Docker layers** — reduzir tempo de CI
  - **Esforço:** S

- [ ] **Branch protection** — exigir CI verde antes de merge
  - **Esforço:** S (config GitHub UI)

---

## Fase 6 — Documentação e polish (3–5 dias)

- [ ] **README Cloud Edition** — `docs/cloud-edition/README.md` com arquitetura K8s
  - **Esforço:** M

- [ ] **Diagrama de arquitetura atualizado** — K8s: gateway, services, Redis, Mongo, MinIO, observability
  - **Arquivos:** `docs/cloud-edition/architecture.md` + diagrama (mermaid ou PNG)
  - **Esforço:** M
  - **Skills:** technical writing, system design

- [ ] **Trade-offs documentados** — kind vs minikube, Redis DLQ vs RabbitMQ, Mongo split vs unified, sticky sessions
  - **Esforço:** S

- [ ] **Métricas e benchmarks** — resultados k6, HPA, recovery time
  - **Arquivos:** `docs/cloud-edition/benchmarks.md`
  - **Esforço:** S

- [ ] **Lições aprendidas** — `docs/cloud-edition/lessons.md`
  - **Esforço:** S

- [ ] **Atualizar root README.md** — link à Cloud Edition; não quebrar fluxo dev local (`go run ./cmd/dev`)
  - **Esforço:** S

- [ ] **Atualizar AGENTS.md** — paths `deploy/`, convenções Helm, CI
  - **Esforço:** S

- [ ] **Hardening segurança (mínimo para "production-grade")** — JWT/INTERNAL_SERVICE_TOKEN obrigatórios em prod values; origins restritos
  - **Arquivos:** `values-prod.yaml`, gateway/websocket security
  - **Dependências:** backlog existente
  - **Esforço:** M

---

## Tabela resumo

| Fase | Tarefas (aprox.) | Esforço total | Skills principais |
|------|------------------|---------------|-------------------|
| **1 — Containerização + kind** | ~12 | **L** (2–3 sem) | Docker, Compose, kind, Go build |
| **2 — K8s + Helm + HPA** | ~15 | **L** (2–3 sem) | Kubernetes, Helm, HPA, Ingress, probes |
| **3 — Resiliência + DLQ** | ~10 | **M** (1–2 sem) | Redis Streams, chaos testing, WS scaling |
| **4 — Observability + k6** | ~10 | **M** (1–2 sem) | Prometheus, Grafana, k6, benchmarking |
| **5 — CI/CD** | ~6 | **M** (1 sem) | GitHub Actions, ghcr.io, kind CI |
| **6 — Docs + polish** | ~8 | **S** (3–5 dias) | Technical writing, security hardening |

---

## Próximos 3 passos

1. **Corrigir consumer names** (`HOSTNAME`/`POD_NAME`) em chat, notification e logging — bloqueia HPA de consumers sem conflito no Redis consumer group.

2. **Criar Dockerfile multi-stage** para um serviço piloto (ex. `gateway`) + estender `docker-compose.yml` com profile `apps` só para gateway + infra — valida o pipeline de imagem antes de replicar para os 9 serviços.

3. **Subir kind local** com `deploy/kind/cluster-config.yaml` e script `kind-up.sh` — prepara o destino do Helm mesmo antes do chart completo.
