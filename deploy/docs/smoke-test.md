# Smoke Test — Docker Compose (profile apps)

Manual end-to-end validation after containerizing all services.

## Prerequisites

- Docker and Docker Compose
- `curl` and `jq` (optional, for JSON parsing)
- Copy Compose env: `cp .env.compose.example .env.compose` (from `backend/`)

## 1. Start infrastructure

From `backend/`:

```bash
docker compose up -d
```

Starts Redis, MongoDB (5 instances), and MinIO.

## 2. Start application services

```bash
docker compose --profile apps up -d --build
```

Builds and starts all 9 Go microservices, gateway (`:8080`), and frontend (`:5173`).

## 3. Health check

```bash
curl -sf http://localhost:8080/healthz
```

Expected: HTTP 200 with body `ok` (or empty success).

Readiness (checks upstreams):

```bash
curl -sf http://localhost:8080/readyz
```

## 4. Register users

```bash
curl -s -X POST http://localhost:8080/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"username":"alice","password":"password123"}'

curl -s -X POST http://localhost:8080/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"username":"bob","password":"password123"}'
```

## 5. Login and obtain JWT

```bash
TOKEN=$(curl -s -X POST http://localhost:8080/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"alice","password":"password123"}' | jq -r '.token')

echo "$TOKEN"
```

## 6. WebSocket connect

Use a WebSocket client or browser at `http://localhost:5173` (frontend container).

CLI example with `websocat` (if installed):

```bash
websocat "ws://localhost:8080/ws?token=${TOKEN}"
```

## 7. Send chat message (HTTP history after WS delivery)

After alice and bob are friends (add via UI or friends API), send via WebSocket JSON:

```json
{
  "type": "chat_message",
  "payload": {
    "receiver_id": "<bob-user-id>",
    "content": "hello from smoke test"
  }
}
```

Verify history:

```bash
curl -s "http://localhost:8080/messages/<bob-user-id>" \
  -H "Authorization: Bearer ${TOKEN}"
```

## 8. Upload attachment

```bash
curl -s -X POST http://localhost:8080/attachments \
  -H "Authorization: Bearer ${TOKEN}" \
  -F "file=@/path/to/small-file.txt"
```

Use returned `attachment_ids` in a WebSocket `chat_message` payload.

## 9. Frontend UI

Open `http://localhost:5173`, register/login, send messages, and confirm realtime delivery.

## Troubleshooting

| Issue | Check |
|-------|--------|
| Gateway 502/503 | `docker compose ps`; wait for Mongo/Redis; check `curl localhost:8080/readyz` |
| CORS errors | `GATEWAY_ALLOWED_ORIGIN` in `.env.compose` must match browser origin (`http://localhost:5173`) |
| WS disconnect | JWT expiry; gateway and websocket pods/containers running |
| Media upload fails | MinIO up; `MEDIA_STORAGE_*` in `.env.compose` |

## Cleanup

```bash
docker compose --profile apps down
docker compose down
```
