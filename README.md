# 💬 GoMessenger, a Real-Time Chat Backend in Go

The GoMessenger is a **real-time chat platform** built with **Go**, designed to explore advanced backend engineering concepts — including caching, messaging, rate limiting, observability, end-to-end testing, and NoSQL databases.

---

## 🚀 Technologies & Concepts

| Category           | Technology / Concept                                     |
| ------------------ | -------------------------------------------------------- |
| Language           | Go (Golang)                                              |
| Communication      | WebSocket (`gorilla/websocket` or `nhooyr.io/websocket`) |
| Cache / Sessions   | Redis                                                    |
| Messaging          | Redis Streams                                            |
| Database           | MongoDB                                                  |
| Observability      | Prometheus, Grafana                                      |
| Authentication     | JWT                                                      |
| End-to-End Testing | testcontainers-go + testify                              |

---

## Core Services

### 🔹 **Gateway Service**

- Central entry point for all clients.
- Manages WebSocket connections and session authentication (via JWT).
- Applies **rate limiting** per user
- Publishes messages to Redis Streams.
- Forwards chat events received from the ChatService to connected users.

### 🔹 **Authentication Service**

- Handles user registration and login via gRPC.
- Issues JWT tokens and manages sessions in Redis.
- Persists user data in MongoDB.
- HTTP clients should use the Gateway service for `/auth/register` and `/auth/login`.
- Friendship is modeled with separate `friend_requests` and `friends` collections, with acceptance creating both friendship directions transactionally.

### 🔹 **Chat Service**

- Consumes messages from the queue.
- Persists messages in the NoSQL database (Mongo)
- Publishes new message events through Redis Pub/Sub.
- Ensures idempotent delivery.

### 🔹 **Presence Service** WIP

- Tracks real-time user presence (online/offline, current chat ID) with redis.
- Stores connection state in Redis.
- Publishes status changes to interested services (e.g., NotificationService).

### 🔹 **Notification Service** WIP

- Subscribes to chat and presence events.
- Decides whether to send notifications based on user preferences and active status.
- Handles asynchronous notification delivery (push, email, or simulated logs).
  
---

## Message Flow

1. User connects via WebSocket → Authenticated through JWT (AuthService).
2. Gateway stores session in Redis and registers presence.
3. User sends a message → Gateway publishes to Redis Stream (`chat.message.created`).
4. ChatService consumes, persists message in MongoDB, and publish via Redis Pub/Sub.
5. PresenceService updates user activity and publishes online/offline changes.
6. NotificationService receives events and sends external notifications if recipient is offline or inactive.
7. Observability stack (Prometheus + Grafana) tracks latency, throughput, and errors across services.


### Prerequisites

- Go 1.23+
- Docker & Docker Compose

### Commands

```bash
# Clone the repository
git clone https://github.com/Miguel-Pezzini/GoMessenger.git

# Start dependencies
docker-compose up -d

# Run all active services
go run ./cmd/dev

# Or run them individually
go run ./services/gateway/cmd
go run ./services/auth/cmd
go run ./services/friends/cmd
go run ./services/chat/cmd
go run ./services/websocket/cmd
```

---

## Key Learning Outcomes

✅ Real-time communication with WebSocket
✅ Distributed cache and Pub/Sub (Redis)
✅ Asynchronous messaging (Redis Streams/RabbitMQ/NATS/SQS)
WIP: Rate limiting and connection control
WIP: Full observability (logs, metrics, tracing)
WIP: End-to-end integration testing 
✅ Event-driven microservice architecture

---

## Author

**Miguel P.**
Backend developer focused on performance, scalability, and distributed systems using Go.

---

## License

This project is licensed under the MIT License — feel free to study, adapt, and improve it.
