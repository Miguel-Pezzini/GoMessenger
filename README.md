# 💬 GoMessenger, a Real-Time Chat Backend in Go

The GoMessenger is a **real-time chat platform** built with **Go**, designed to explore advanced backend engineering concepts — including caching, messaging, rate limiting, observability, end-to-end testing, and NoSQL databases.

---

## 🚀 Technologies & Concepts

| Category           | Technology / Concept                                     |
| ------------------ | -------------------------------------------------------- |
| Language           | Go (Golang)                                              |
| Communication      | WebSocket (`gorilla/websocket` or `nhooyr.io/websocket`) |
| Cache / Sessions   | Redis                                                    |
| Messaging          | RabbitMQ / NATS / AWS SQS                                |
| Database           | MongoDB or DynamoDB                                      |
| Observability      | Prometheus, Grafana, OpenTelemetry, Jaeger               |
| Authentication     | JWT                                                      |
| End-to-End Testing | testcontainers-go + testify                              |

---

## 🧩 Architecture Overview

```
           +----------------------+
           |      API Gateway     |
           | (HTTP + WebSocket)   |
           +----------+-----------+
                      |
        +-------------+--------------+
        |                            |
+---------------+          +------------------+
| Message Bus   |          |    Redis Cache   |
| (RabbitMQ/NATS|          | (Sessions, PubSub)|
| /SQS)         |          +------------------+
+---------------+                     |
        |                              |
  +------------+             +-----------------+
  | Chat Svc   |             | Presence Svc     |
  | (Messages, |             | (User Status)    |
  | History)   |             +-----------------+
  +------------+
        |
 +------------------+
 | MongoDB/DynamoDB |
 +------------------+
```

---

## 🧠 Core Services

### 🔹 **Gateway Service**

- Handles WebSocket connections.
- Authenticates users via JWT and stores sessions in Redis.
- Applies **rate limiting** per user.
- Publishes messages to the message queue.

### 🔹 **Chat Service**

- Consumes messages from the queue.
- Persists messages in the NoSQL database.
- Publishes new message events through Redis Pub/Sub.
- Ensures idempotent delivery.

### 🔹 **Presence Service**

- Tracks online/offline user status using Redis.
- Publishes presence updates to gateways.

### 🔹 **Notification Service**

- Processes asynchronous events from the queue.
- Sends external notifications (push, email, or simulated logs).

---

## 🧾 Suggested Directory Structure

```
/chat-backend
├── cmd/
│   ├── gateway/
│   ├── chat/
│   ├── presence/
│   └── notification/
├── internal/
│   ├── websocket/
│   ├── redis/
│   ├── messaging/
│   ├── repository/
│   ├── auth/
│   ├── limiter/
│   └── observability/
├── pkg/
│   └── models/
├── tests/
│   └── e2e/
├── docker-compose.yml
└── README.md
```

---

## ⚙️ Message Flow

1. A user connects via WebSocket → authenticated via JWT.
2. Session stored in Redis.
3. User sends a message → published to the message queue (`chat.message.created`).
4. Chat Service consumes, stores in MongoDB, and publishes via Redis Pub/Sub.
5. Gateways receive and broadcast to connected clients in the same room.
6. Presence Service updates online/offline status.
7. Observability tools track message latency and throughput.

---

## 🔍 Observability

- **Structured Logging:** `zerolog` or `logrus`
- **Metrics (Prometheus):**

  - `messages_sent_total`
  - `active_connections_total`
  - `avg_message_latency_ms`

- **Distributed Tracing:** OpenTelemetry + Jaeger

---

## 🧪 End-to-End Testing

Using **testcontainers-go**, the E2E tests:

- Start Redis, RabbitMQ, and MongoDB containers.
- Simulate multiple WebSocket clients.
- Send and receive messages through the full stack.
- Validate message persistence and broadcast.
- Measure end-to-end latency (<100ms locally).

---

## 🧰 Getting Started

### Prerequisites

- Go 1.23+
- Docker & Docker Compose

### Commands

```bash
# Clone the repository
git clone https://github.com/your-username/chat-backend.git
cd chat-backend

# Start dependencies
docker-compose up -d

# Run the gateway service
go run ./cmd/gateway

# Run other services
go run ./cmd/chat
go run ./cmd/presence
```

---

## 📚 Key Learning Outcomes

✅ Real-time communication with WebSocket
✅ Distributed cache and Pub/Sub (Redis)
✅ Asynchronous messaging (RabbitMQ/NATS/SQS)
WIP: Rate limiting and connection control
WIP: Full observability (logs, metrics, tracing)
WIP: End-to-end integration testing 
✅ Event-driven microservice architecture

---

## 🧑‍💻 Author

**Miguel P.**
Backend developer focused on performance, scalability, and distributed systems using Go.

---

## 🏗️ License

This project is licensed under the MIT License — feel free to study, adapt, and improve it.
