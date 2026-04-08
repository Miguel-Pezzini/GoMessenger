# GoMessenger

A real-time chat backend written in Go, organized into small services integrated through HTTP, Redis, and MongoDB.

Today the project already covers:

- user registration and login with JWT.
- authenticated WebSocket connection through the gateway with origin checks
- message exchange between two connected users
- typing indicators and chat-open lifecycle events
- delivery and read acknowledgements with persisted `viewed_status`
- message persistence in MongoDB
- presence snapshots backed by Redis
- notification service for friend-request and message alerts
- append-only audit logging with admin live streaming
- Redis Stream for ingestion and Redis Pub/Sub for fan-out
- unit and integration tests covering auth, friends, chat history, websocket, presence, and logging

## Current State

The flow implemented in the repository today is:

1. The client calls the `gateway` using `POST /auth/register` or `POST /auth/login`.
2. The `gateway` proxies authentication to the `auth` service over HTTP.
3. The `auth` service creates or validates the user in MongoDB and returns a JWT.
4. The client opens `ws://localhost:8080/ws` with `Authorization: Bearer <jwt>`.
5. The `gateway` validates the token and proxies the connection to the `websocket` service.
6. The `websocket` service receives the message and writes the payload to a Redis Stream.
7. The `chat` service consumes the stream, persists the message in MongoDB, and publishes the result through Redis Pub/Sub.
8. The `websocket` service listens to the Pub/Sub channel and delivers the message to connected sender and receiver clients.
9. The `websocket` service also publishes presence lifecycle, typing, and receipt events through Redis Pub/Sub.
10. The `presence_service` stores the latest online/offline and `current_chat_id` snapshot per user.
11. The `notification` service consumes notification-intent streams, checks presence state, and publishes normalized in-app notifications.
12. Services publish audit events to a Redis Stream, and the `logging` service persists and streams them for admin consumers.

## Architecture

GoMessenger uses a microservices architecture with a flat package layout inside each service. The repository no longer uses `domain/infra/transport` folders.

Typical service layout:

```text
services/<name>/
├── cmd/                        # Thin main.go entrypoint
└── internal/
    ├── app.go / server.go      # Config loading and dependency wiring
    ├── service.go              # Business rules
    ├── repository.go           # Optional local interfaces
    ├── mongo_repository.go     # MongoDB integration when needed
    ├── redis_repository.go     # Redis integration when needed
    ├── http_handler.go         # HTTP or WebSocket handlers
    ├── stream_server.go        # Redis Stream consumers when needed
    ├── model.go                # Request/response and persistence structs
    └── helpers                 # Mapping, auth, proxy, and other service-specific code
```

`cmd/<service>/main.go` calls `Run()` or `NewServer(...)`, wiring lives in `app.go` or `server.go`, handlers and consumers call the service layer, and services use MongoDB or Redis repositories when they need an abstraction boundary. Shared cross-cutting code lives in `internal/platform/`.

## Services

### `gateway`

- Port: `8080`
- Responsibilities:
  - expose HTTP authentication routes
  - expose HTTP friendship routes
  - validate JWT for WebSocket access and accept previous secrets during rotation
  - forward `/ws` to the `websocket` service

Available routes:

- `POST /auth/register`
- `POST /auth/login`
- `POST /friends/requests`
- `POST /friends/requests/{id}/accept`
- `DELETE /friends/requests/{id}/decline`
- `GET /friends/requests/pending`
- `GET /friends`
- `DELETE /friends/{friendId}`
- `GET /messages/{userId}`
- `GET /presence/{userId}`
- `GET /logs`
- `GET /logs/ws`
- `GET /ws`

### `auth`

- Port: `50051` (HTTP)
- Responsibilities:
  - register users
  - authenticate users
  - generate JWTs with the `userId` claim and configurable expiry
- Database:
  - MongoDB at `mongodb://localhost:27019`, database `userdb`

### `friends`

- Port: `50052` (HTTP)
- Responsibilities:
  - create friend requests
  - accept or decline friend requests
  - list friendships and pending requests
  - remove friendships
- Database:
  - MongoDB at `mongodb://localhost:27020`, database `friends_db`

### `websocket`

- Port: `8081`
- Responsibilities:
  - accept WebSocket connections proxied by the gateway
  - receive client messages
  - publish messages to the Redis Stream
  - publish presence, typing, and receipt events to Redis
  - listen to Redis Pub/Sub and forward messages and realtime events to connected clients

Message format sent by the client:

```json
{
  "type": "chat_message",
  "payload": {
    "receiver_id": "user-b",
    "content": "hello"
  }
}
```

The `websocket` service fills `sender_id` from the authenticated JWT identity. If a client sends `sender_id`, it must match the authenticated user or the message is rejected.

Additional realtime message types supported by the WebSocket service:

- `typing_started`
- `typing_stopped`
- `chat_opened`
- `chat_closed`
- `message_delivered`
- `message_seen`

### `chat`

- Port: `8082` (HTTP + Redis Stream consumer)
- Uses Redis to consume and publish events
- Responsibilities:
  - expose `GET /messages/{userId}` for conversation history
  - consume messages from the Redis Stream
  - persist messages in MongoDB
  - update persisted `viewed_status` from delivery and read receipt events
  - publish the resulting payload through Redis Pub/Sub
  - enqueue message notification intents after persistence
- Database:
  - MongoDB at `mongodb://localhost:27018`, database `chatdb`

### `notification`

- Port: `8085`
- Responsibilities:
  - consume friend-request and message notification intents from Redis Streams
  - load receiver presence snapshots from Redis
  - suppress message notifications when the receiver is actively viewing the sender chat
  - publish normalized `notification` websocket events through Redis Pub/Sub

### `logging`

- Port: `8084`
- Responsibilities:
  - consume structured audit and error events from Redis
  - persist immutable log events in MongoDB
  - expose `GET /logs` for admin history
  - expose `GET /logs/ws` for admin real-time log streaming
- Database:
  - MongoDB at `mongodb://localhost:27021`, database `logging_db`

### `presence_service`

- Port: `8083`
- Responsibilities:
  - consume presence lifecycle events from Redis
  - store the latest presence snapshot per user in Redis
  - expose `GET /presence/{userId}` through the gateway
- Presence snapshot fields:
  - `status`
  - `last_seen`
  - `current_chat_id`

## Structure

```text
.
|-- cmd/
|   |-- dev/
|   `-- testui/
|-- internal/
|   `-- platform/
|-- services/
|   |-- auth/
|   |-- chat/
|   |-- friends/
|   |-- gateway/
|   |-- logging/
|   |-- notification/
|   |-- presence_service/
|   `-- websocket/
|-- tests/
|   |-- integration/
|   `-- unit/
|-- docker-compose.yml
|-- go.mod
`-- go.work
```

## Requirements

- Go `1.23+`
- Docker
- Docker Compose

## Local Dependencies

Start Redis and the databases:

```bash
docker-compose up -d
```

Expected containers:

- `redis` at `localhost:6379`
- `mongo_chat` at `localhost:27018`
- `mongo_user` at `localhost:27019`
- `mongo_friends` at `localhost:27020`
- `mongo_logging` at `localhost:27021`

## Environment Variables

Current defaults:

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
JWT_SECRET=replace-with-a-strong-secret
JWT_SECRET_PREVIOUS=
JWT_EXPIRY=24h
WEBSOCKET_ALLOWED_ORIGINS=http://localhost:5173
LOGGING_ALLOWED_ORIGINS=http://localhost:5173
```

See [`.env_example`](.env_example) for the full default environment.

Security notes:

- `JWT_SECRET` is required at startup for both `auth` and `gateway`.
- `JWT_EXPIRY` accepts Go duration strings such as `15m` or `24h`.
- `JWT_SECRET_PREVIOUS` can contain a comma-separated list of old signing secrets that the gateway should still accept during a rotation window.
- Refresh tokens are not implemented yet. Today the system issues access tokens only, so shorter `JWT_EXPIRY` values plus rotation windows are the current mitigation.

## Running

### Option 1: start the active services

```bash
go run ./cmd/dev
```

`cmd/dev` starts:

- `auth`
- `chat`
- `friends`
- `gateway`
- `logging`
- `notification`
- `presence`
- `websocket`

### Option 2: start manually

In separate terminals:

```bash
go run ./services/auth/cmd
go run ./services/chat/cmd
go run ./services/friends/cmd
go run ./services/websocket/cmd
go run ./services/gateway/cmd
go run ./services/logging/cmd
go run ./services/notification/cmd
go run ./services/presence_service/cmd
```

## Tests

Current coverage:

- auth register/login
- friend request flows
- notification delivery and suppression rules
- websocket message delivery, typing, presence, and receipts
- chat history and pagination
- audit logging and admin access control

To run the full suite:

```bash
./scripts/test_all.sh
```

Or run them separately:

```bash
./scripts/test_unit.sh
./scripts/test_integration.sh
```

The integration script starts isolated test infrastructure, boots all services with `.env.test`, and runs `tests/integration`.

The backlog and planned improvements are listed in [TODO.md](TODO.md).

The realtime event contract for presence, typing, and future delivery/read feedback is documented in [docs/realtime-chat-feedback.md](docs/realtime-chat-feedback.md).
