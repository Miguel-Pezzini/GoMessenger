# GoMessenger — Agent Guide (repository root)

Real-time chat stack: **Go microservices** plus **Vue 3 + Vite** frontend. Read this file first, then the guide for the area you are changing.

---

## Where to look

| Area | Guide |
|------|--------|
| Backend (services, Redis, MongoDB, tests) | [backend/AGENTS.md](backend/AGENTS.md) |
| Frontend (Vue, Vite, gateway URLs) | [frontend/AGENTS.md](frontend/AGENTS.md) |
| Backend overview and API surface | [backend/README.md](backend/README.md) |

---

## Repository layout

```text
GoMessenger/
├── backend/          # Go workspace: gateway, auth, chat, websocket, friends, presence, logging, notification
├── frontend/         # Vue 3 app (Vite)
└── AGENTS.md         # This file
```

---

## Local development (typical)

1. **Infrastructure** — From `backend/`, start Redis and MongoDB (see [backend/AGENTS.md](backend/AGENTS.md) — `docker compose up -d`).

2. **Backend** — From `backend/`, run all services in one process:

   ```bash
   go run ./cmd/dev
   ```

   Gateway listens on **`:8080`** (REST + proxied WebSocket).

3. **Frontend** — From `frontend/`:

   ```bash
   npm install
   npm run dev
   ```

   Default UI: `http://localhost:5173`. It talks to the gateway at `http://localhost:8080` and `ws://localhost:8080/ws?token=<jwt>` (details in [frontend/AGENTS.md](frontend/AGENTS.md)).

Environment variables and ports for each service are documented in **backend/AGENTS.md** and `backend/.env_example`.

---

## Cross-cutting expectations

- **Backend changes**: Follow [backend/AGENTS.md](backend/AGENTS.md) — layered services, shared `internal/platform/` helpers, and **mandatory unit + integration tests** (`backend/scripts/test_all.sh` from `backend/` on Unix-like shells, or the equivalent `test_unit.sh` / `test_integration.sh` steps).

- **Frontend changes**: Match existing Vue/Tailwind patterns; keep gateway and WebSocket URLs consistent with the backend defaults unless configuration is explicitly introduced.

- **End-to-end behavior**: Chat flows through the **gateway** → **websocket** → Redis → **chat** and Pub/Sub back to clients; do not assume a single monolithic server. High-level message flow diagrams live in **backend/AGENTS.md**.

- **Product backlog / gaps**: See `backend/TODO.md` when relevant.

---

## Cursor rules

Project-specific rules may live under `.cursor/rules/`. Subfolder **AGENTS.md** files remain the source of truth for stack-specific conventions.
