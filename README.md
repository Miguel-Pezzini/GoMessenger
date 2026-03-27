# GoMessenger

A real-time chat backend written in Go, organized into small services integrated through HTTP/gRPC, Redis, and MongoDB.

Today the project already covers:

- user registration and login with JWT
- authenticated WebSocket connection through the gateway
- message exchange between two connected users
- message persistence in MongoDB
- Redis Stream for ingestion and Redis Pub/Sub for fan-out
- basic end-to-end tests for authentication and WebSocket messaging

## Current State

The flow implemented in the repository today is:

1. The client calls the `gateway` using `POST /auth/register` or `POST /auth/login`.
2. The `gateway` forwards authentication to the `auth` service via gRPC.
3. The `auth` service creates or validates the user in MongoDB and returns a JWT.
4. The client opens `ws://localhost:8080/ws?token=<jwt>`.
5. The `gateway` validates the token and proxies the connection to the `websocket` service.
6. The `websocket` service receives the message and writes the payload to a Redis Stream.
7. The `chat` service consumes the stream, persists the message in MongoDB, and publishes the result through Redis Pub/Sub.
8. The `websocket` service listens to the Pub/Sub channel and delivers the message to connected sender and receiver clients.

## Services

### `gateway`

- Port: `8080`
- Responsibilities:
  - expose HTTP authentication routes
  - expose HTTP friendship routes
  - validate JWT for WebSocket access
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
- `GET /ws?token=<jwt>`

### `auth`

- Port: `50051` (gRPC)
- Responsibilities:
  - register users
  - authenticate users
  - generate JWTs with the `userId` claim
- Database:
  - MongoDB at `mongodb://localhost:27019`, database `userdb`

### `friends`

- Port: `50052` (gRPC)
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
  - listen to Redis Pub/Sub and forward messages to connected clients

Message format sent by the client:

```json
{
  "type": "chat_message",
  "payload": {
    "sender_id": "user-a",
    "receiver_id": "user-b",
    "content": "hello"
  }
}
```

### `chat`

- Uses Redis to consume and publish events
- Responsibilities:
  - consume messages from the Redis Stream
  - persist messages in MongoDB
  - publish the resulting payload through Redis Pub/Sub
- Database:
  - MongoDB at `mongodb://localhost:27018`, database `chatdb`

### `presence_service`

It currently exists only as an initial structure and is not part of the main flow.

Today it:

- has its own `go.mod`
- contains initial Redis connection code
- is not integrated into the main flow

## Structure

```text
.
|-- cmd/
|   `-- dev/
|-- internal/
|-- pkg/
|-- proto/
|-- services/
|   |-- auth/
|   |-- chat/
|   |-- friends/
|   |-- gateway/
|   |-- presence_service/
|   `-- websocket/
|-- tests_e2e/
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

## Environment Variables

Current defaults:

```bash
REDIS_ADDR=localhost:6379
REDIS_STREAM_CHAT=chat.message.created
REDIS_CHANNEL_CHAT=chat.message.persisted
AUTH_GRPC_ADDR=localhost:50051
FRIENDS_GRPC_ADDR=localhost:50052
WEBSOCKET_UPSTREAM_URL=http://localhost:8081
JWT_SECRET=secret-key
```

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
- `websocket`

### Option 2: start manually

In separate terminals:

```bash
go run ./services/auth/cmd
go run ./services/chat/cmd
go run ./services/friends/cmd
go run ./services/websocket/cmd
go run ./services/gateway/cmd
```

## Tests

Current coverage:

- user registration
- login fallback when the user already exists
- message exchange between two users connected through WebSocket

To run:

```bash
go test ./...
```

End-to-end tests depend on the services being active locally.

The backlog and planned improvements are listed in [TODO.md](/c:/Users/migue/programming/GoMessenger/TODO.md).
