# 💬 GoMessenger, a Real-Time Chat Backend in Go

The GoMessenger is a **real-time chat platform** built with **Go**, designed to explore advanced backend engineering concepts — including caching, messaging, rate limiting, observability, end-to-end testing, and NoSQL databases.

---

## 🚀 Technologies & Concepts

| Category           | Technology / Concept                                     |
| ------------------ | -------------------------------------------------------- |
| Language           | Go (Golang)                                              |
| Communication      | WebSocket (`gorilla/websocket` or `nhooyr.io/websocket`) |
| Cache / Sessions   | Redis                                                    |
| Messaging          | RabbitMQ | Redis                                         |
| Database           | MongoDB                                                  |
| Observability      | Prometheus, Grafana                                      |
| Authentication     | JWT                                                      |
| End-to-End Testing | testcontainers-go + testify                              |

---

## 🧠 Core Services

### 🔹 **Gateway Service**

- Client endpoint service.
- Handles WebSocket connections.
- Applies **rate limiting** per user.
- Publishes messages to the message queue.

### 🔹 **Authentication Service**

- Authenticates users via JWT and stores sessions in Redis.
- Persist all users in NOSQL Database (Mongo)

### 🔹 **Chat Service**

- Consumes messages from the queue.
- Persists messages in the NoSQL database (Mongo)
- Publishes new message events through Redis Pub/Sub.
- Ensures idempotent delivery.

### 🔹 **Presence Service** WIP

- Tracks online/offline user status using Redis.
- Publishes presence updates to gateways.

### 🔹 **Notification Service** WIP

- Processes asynchronous events from the queue.
- Sends external notifications (push, email, or simulated logs).

---

## ⚙️ Message Flow

1. A user connects via WebSocket → authenticated via JWT.
2. Session stored in Redis.
3. User sends a message → published to the message queue (`chat.message.created`).
4. Chat Service consumes, stores in MongoDB, and publishes via Redis Pub/Sub.
5. Presence Service updates online/offline status.
6. Observability tools track message latency and throughput.

---Miguel-Pezzini
GoMessengerg Started

### Prerequisites

- Go 1.23+
- Docker & Docker Compose

### Commands

```bash
# Clone the repository
git clone https://github.com/Miguel-Pezzini/GoMessenger.git

# Start dependencies
docker-compose up -d

# Run the gateway service
go run ./gateway/cmd

# Run other services
go run ./chat_service/cmd
go run ./auth_service/cmd
```

---

## 📚 Key Learning Outcomes

✅ Real-time communication with WebSocket
✅ Distributed cache and Pub/Sub (Redis)
✅ Asynchronous messaging (Redis Streams/RabbitMQ/NATS/SQS)
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
