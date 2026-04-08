# GoMessenger — Agent Guide (Backend)

Go microservices backend for a real-time chat application. Read this file fully before touching any code.

---

## Project layout

```text
GoMessenger/
├── cmd/
│   ├── dev/                    # Starts all services in one process
│   └── testui/                 # Manual UI for local testing
├── services/
│   ├── auth/                   # HTTP :50051 — register/login, JWT issuance
│   ├── friends/                # HTTP :50052 — friend requests and relationships
│   ├── gateway/                # HTTP :8080  — JWT validation and reverse proxy
│   ├── websocket/              # HTTP :8081  — WebSocket hub, Redis Stream producer, Pub/Sub consumer
│   ├── chat/                   # HTTP :8082 + Redis Stream consumer — history API and MongoDB persistence
│   ├── presence_service/       # HTTP :8083 — presence snapshot API + Redis lifecycle consumer
│   ├── logging/                # HTTP :8084 + Redis Stream consumer — audit log API and persistence
│   └── notification/           # HTTP :8085 + Redis Stream consumer — notification routing
├── internal/platform/          # Shared utilities (audit, config, mongo, redis)
├── tests/
│   ├── integration/            # End-to-end tests (separate Go module in go.work)
│   └── unit/                   # Unit tests mirroring package paths
├── docker-compose.yml
├── go.mod / go.work
└── TODO.md
```

### Per-service internal layout

```text
services/<name>/
├── cmd/                        # Thin entrypoint; main.go calls internal Run/NewServer
└── internal/
    ├── app.go / server.go      # Load config, wire dependencies, start HTTP servers and background consumers
    ├── service.go              # Business rules
    ├── repository.go           # Local interfaces when an abstraction boundary is useful
    ├── mongo_repository.go     # MongoDB persistence for services that need it
    ├── redis_repository.go     # Redis Pub/Sub, Stream, or KV access for services that need it
    ├── http_handler.go         # HTTP or WebSocket handlers
    ├── stream_server.go        # Redis Stream consumers when the service uses them
    ├── model.go                # Request/response and persistence structs
    └── *.go                    # Service-specific helpers such as auth, mapping, or proxy code
```

Not every service uses every file, but this flat `internal/` package pattern is the current standard.

---

## Architecture rules

This project now uses a **service-oriented layered architecture** with a flat package per service, not hexagonal `domain/infra/transport` folders.

- Each service owns its own `internal/` package and keeps closely related code together.
- `cmd/<service>/main.go` is intentionally thin; startup and dependency wiring live in `internal/app.go` or `internal/server.go`.
- HTTP handlers, WebSocket handlers, middleware, and Redis consumers call `Service` types directly.
- `Service` types contain the business logic and use repositories or publishers for MongoDB and Redis access when an abstraction boundary is helpful.
- Shared cross-cutting code belongs in `internal/platform/`.
- Keep dependencies simple and local to the service. Do not reintroduce the old `domain/infra/transport` split unless the codebase changes again.

When adding a new capability:
1. Extend the service models in `model.go` or a nearby helper file.
2. Add methods to `repository.go` only if the service needs a storage or messaging abstraction.
3. Implement MongoDB or Redis changes in the matching `*_repository.go`.
4. Add business logic in `service.go`.
5. Expose the behavior through `http_handler.go`, `server.go`, or `stream_server.go`.
6. Wire new dependencies in `app.go`, `server.go`, or the service `cmd/main.go`.

---

## Infrastructure

```bash
docker-compose up -d
```

| Container     | Port            | Used by               |
|---------------|-----------------|-----------------------|
| redis         | localhost:6379  | all services          |
| mongo_chat    | localhost:27018 | chat service          |
| mongo_user    | localhost:27019 | auth service          |
| mongo_friends | localhost:27020 | friends service       |
| mongo_logging | localhost:27021 | logging service       |

---

## Running services

```bash
# All at once
go run ./cmd/dev

# Individually
go run ./services/auth/cmd
go run ./services/chat/cmd
go run ./services/friends/cmd
go run ./services/websocket/cmd
go run ./services/gateway/cmd
go run ./services/logging/cmd
go run ./services/notification/cmd
go run ./services/presence_service/cmd
```

---

## Environment variables

```bash
GATEWAY_ADDR=:8080
AUTH_ADDR=:50051
FRIENDS_ADDR=:50052
WEBSOCKET_ADDR=:8081
CHAT_ADDR=:8082
PRESENCE_ADDR=:8083
LOGGING_ADDR=:8084
NOTIFICATION_ADDR=:8085
REDIS_ADDR=localhost:6379
REDIS_STREAM_CHAT=chat.message.created
REDIS_STREAM_AUDIT_LOGS=audit.logs
REDIS_STREAM_NOTIFICATION_FRIEND_REQUESTS=notification.friend_request.created
REDIS_STREAM_NOTIFICATION_MESSAGES=notification.message.created
REDIS_CHANNEL_CHAT=chat.message.persisted
REDIS_CHANNEL_CHAT_EVENTS=chat.events
REDIS_CHANNEL_FRIEND_EVENTS=friend.events
REDIS_CHANNEL_NOTIFICATIONS=notifications
REDIS_CHANNEL_PRESENCE_EVENTS=presence.lifecycle
REDIS_CHANNEL_PRESENCE=presence.updated
REDIS_KEY_PREFIX_PRESENCE=presence:user:
NOTIFICATION_FRIEND_CONSUMER_GROUP=notification-friend-service
NOTIFICATION_MESSAGE_CONSUMER_GROUP=notification-message-service
AUTH_UPSTREAM_URL=http://localhost:50051
FRIENDS_UPSTREAM_URL=http://localhost:50052
WEBSOCKET_UPSTREAM_URL=http://localhost:8081
CHAT_UPSTREAM_URL=http://localhost:8082
PRESENCE_UPSTREAM_URL=http://localhost:8083
LOGGING_UPSTREAM_URL=http://localhost:8084
JWT_SECRET=secret-key
```

See `.env_example` for the full default set. Config is loaded via `internal/platform/config/env.go` (`String()`, `MustString()`).

---

## Message flow

```
Client
  │ HTTP POST /auth/register|login
  ▼
Gateway :8080  ──HTTP proxy──▶  Auth :50051  ──▶  MongoDB (userdb)
  │
  │ ws://localhost:8080/ws?token=<jwt>
  │  (gateway validates JWT, proxies to websocket)
  ▼
Websocket :8081
  │ XAdd  "payload": <json>
  ▼
Redis Stream  (REDIS_STREAM_CHAT)
  ▼
Chat service :8082  ──▶  MongoDB (chatdb, collection: messages)
  │ XPublish
  ▼
Redis Pub/Sub  (REDIS_CHANNEL_CHAT)
  ▼
Websocket :8081  ──▶  Client (sender + receiver)
```

Friend events flow separately:

```
Client ──▶ Gateway ──HTTP proxy──▶ Friends :50052 ──▶ MongoDB (friends_db)
                 │ Publish
                 ▼
           Redis Pub/Sub  (REDIS_CHANNEL_FRIEND_EVENTS)
                 ▼
           Websocket :8081 ──▶ target client
```

Presence and audit flow in parallel:

- `websocket` publishes lifecycle events to `REDIS_CHANNEL_PRESENCE_EVENTS`; `presence_service` consumes them, stores the latest snapshot in Redis, and serves `GET /presence/{userId}`.
- `friends` and `chat` publish notification intents to `REDIS_STREAM_NOTIFICATION_FRIEND_REQUESTS` and `REDIS_STREAM_NOTIFICATION_MESSAGES`; `notification` consumes them, checks presence state, and publishes normalized websocket notifications on `REDIS_CHANNEL_NOTIFICATIONS`.
- Services publish audit events to `REDIS_STREAM_AUDIT_LOGS`; `logging` consumes them, persists them in MongoDB, and serves `GET /logs` and `GET /logs/ws`.

---

## Redis data shapes

### Stream entry (REDIS_STREAM_CHAT)

```
XADD chat.message.created * payload <json>
```

The `payload` field is a JSON string:

```json
{
  "sender_id":   "user-a",
  "receiver_id": "user-b",
  "content":     "hello",
  "timestamp":   1712000000
}
```

### Pub/Sub: chat persisted (REDIS_CHANNEL_CHAT)

Published by chat service after MongoDB write:

```json
{
  "id":          "663f...",
  "sender_id":   "user-a",
  "receiver_id": "user-b",
  "content":     "hello",
  "timestamp":   1712000000
}
```

### Pub/Sub: friend events (REDIS_CHANNEL_FRIEND_EVENTS)

```json
{
  "target_user_id": "user-b",
  "type":           "friend_request_received",
  "payload":        { ... }
}
```

---

## MongoDB schemas

### userdb — `users` collection

```
_id       ObjectID
username  string
password  string (bcrypt)
```

### friends_db — `friends` collection

```
_id        ObjectID
user_id    string   (compound unique index with friend_id)
friend_id  string
created_at time.Time
```

Friendships are **bidirectional**: accepting A↔B inserts two documents (A→B and B→A).

### friends_db — `friend_requests` collection

```
_id          ObjectID
sender_id    string  (compound unique index with receiver_id)
receiver_id  string  (indexed with created_at)
created_at   time.Time
```

### chatdb — `messages` collection

```
_id         ObjectID
stream_id   string   (unique, sparse — used for idempotency)
sender_id   string
receiver_id string
content     string
timestamp   int64
```

The Redis stream entry ID is stored as `stream_id`. The upsert uses `$setOnInsert` so replayed messages are safe.

---

## JWT

- Algorithm: HS256
- Secret: `JWT_SECRET` env var (defaults to `"secret-key"` — change in production)
- Claims: `userId` (camelCase), `exp` (24 h)
- `sender_id` is always derived from the JWT inside websocket service; clients cannot spoof it.

---

## Known pitfalls and bugs already fixed

### Redis consumer group NOGROUP after FLUSHALL
`tests/integration/main_test.go:TestMain` calls `FLUSHALL` to reset state, which deletes the consumer group. The chat stream server (`services/chat/internal/stream_server.go`) now detects the `NOGROUP` error in `processClaimedMessages` and calls `ensureConsumerGroup` to recreate it before retrying. **Do not remove this guard.**

Consumer group constants in `services/chat/internal/stream_server.go`:
- `consumerGroupName = "chat-service"`
- `readBatchSize = 10`
- `readBlockTimeout = 5s`
- `claimMinIdle = 30s`

---

## TDD — mandatory

> **Hard rule. No exceptions.**

| Trigger       | Required action                                                                 |
|---------------|---------------------------------------------------------------------------------|
| New feature   | Create or update unit tests and integration tests first or alongside the code.  |
| Bug fix       | Add or fix the unit tests and integration tests that cover the bug.             |
| Refactor      | Existing unit and integration tests must pass; add or update tests for exposed behaviour. |

After every change:

```bash
./scripts/test_all.sh
```

After every significant change—whether creating a new service or modifying an existing one—you MUST create or update integration tests.

Integration tests are the most critical part of this project. They are the primary way to validate that the system works correctly across service boundaries.

Rules:
- Do NOT skip integration tests under any circumstance.
- If existing integration tests are affected, you MUST update them.
- If no integration test exists for the change, you MUST create one.
- Every feature and bug fix must include the needed unit-test coverage and integration-test coverage.
- The implementation is NOT complete until unit tests and integration tests pass.
- Always run integration tests before considering the task finished.

The goal is to ensure real behavior is validated, not just internal logic.

All tests must pass before the task is done.

### Test placement

| Layer                        | Location                                              |
|------------------------------|-------------------------------------------------------|
| Shared platform utilities    | `tests/unit/internal/platform/**/*_test.go`           |
| Service unit tests           | `tests/unit/services/<name>/internal/*_test.go`       |
| Integration                  | `tests/integration/`                                  |

### Test conventions

- Hand-written stubs only. No `gomock`, no `testify/mock`.
- Standard library `testing.T` only. No test frameworks.
- Table-driven tests for multiple input scenarios on the same function.
- Integration tests live under `tests/integration/` and run against the isolated test stack started by `./scripts/test_integration.sh`.

---

## Shared utilities

| Package                          | Purpose                                                    |
|----------------------------------|------------------------------------------------------------|
| `internal/platform/config/`      | `String(key, default)` and `MustString(key)` for env vars  |
| `internal/platform/mongo/`       | `NewDatabase(uri, name)` — adds `directConnection=true` for localhost URIs |
| `internal/platform/redis/`       | `NewClient(addr)` — connects and pings                     |

Always use these instead of building your own clients.

---

## Active work (see TODO.md for full detail)

### High priority
- **Improve message reliability**: keep hardening `services/chat/internal/stream_server.go` around retries, idempotency, and failure recovery.
- **Tighten security**: make production secrets mandatory and restrict permissive WebSocket origin handling.

### Medium priority
- Expand unit tests: `auth`, `chat`, `websocket` — invalid tokens, reconnects, Redis/MongoDB failures.
- Add better health checks and readiness for local and test orchestration.
- Keep reducing leftover hardcoded runtime assumptions and improve payload validation.

### Known gaps
- WebSocket `CheckOrigin` is still permissive and should be tightened before production use.
- Realtime scalability work such as slow-client handling, reconnect sync, and multi-device fan-out is still backlog.
- Local development still depends on external services being booted separately; health checks and readiness are limited.
