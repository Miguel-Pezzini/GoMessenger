# GoMessenger-Frontend

Vue 3 + Vite frontend for the GoMessenger real-time chat application.

## Layout

```
GoMessenger-Frontend/
├── src/
│   ├── app/
│   │   ├── App.vue
│   │   └── components/
│   │       ├── AuthCard.vue
│   │       ├── ChatHeader.vue
│   │       ├── ChatMessages.vue
│   │       ├── ChatSidebar.vue
│   │       ├── MessageBubble.vue
│   │       └── MessageInput.vue
│   ├── imports/
│   ├── styles/
│   └── main.ts
├── index.html
├── vite.config.ts
└── package.json
```

## Requirements

- Node.js (LTS)
- npm

## Development

```bash
npm install
npm run dev
```

App runs at `http://localhost:5173` (Vite default).

## Build

```bash
npm run build
```

Output goes to `dist/`.

## Key dependencies

| Package              | Purpose                          |
|----------------------|----------------------------------|
| vue 3                | UI framework                     |
| vite                 | Build tool / dev server          |
| tailwindcss 4        | Utility-first CSS                |
| @mui/material        | Material UI component library    |
| @radix-ui/*          | Headless UI primitives           |
| lucide-vue-next      | Icon set                         |
| motion               | Animations                       |

## Backend connection

The frontend connects to the Go backend at:

- REST: `http://localhost:8080` (gateway)
- WebSocket: `ws://localhost:8080/ws?token=<jwt>`

Authentication flow: POST `/auth/register` or `/auth/login` → receive JWT → open WebSocket with token in query string.

WebSocket message format (client → server):

```json
{
  "type": "chat_message",
  "payload": {
    "receiver_id": "user-b",
    "content": "hello"
  }
}
```

## Active Work

- Presence indicators (online/offline/last_seen) — blocked on backend `presence_service` completion.
- Conversation history, message pagination, delivery/read receipts — tracked in `../GoMessenger/TODO.md`.
