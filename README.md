# GoMessenger

Backend de chat em tempo real escrito em Go, organizado em serviços pequenos e integrados por HTTP/gRPC, Redis e MongoDB.

Hoje o projeto já cobre:

- cadastro e login de usuários com JWT
- conexão WebSocket autenticada via gateway
- envio de mensagens entre dois usuários conectados
- persistência das mensagens no MongoDB
- uso de Redis Stream para ingestão e Redis Pub/Sub para fan-out
- testes E2E básicos para autenticação e troca de mensagens por WebSocket

## Estado Atual

O fluxo implementado no repositório hoje é este:

1. O cliente chama o `gateway` em `POST /auth/register` ou `POST /auth/login`.
2. O `gateway` encaminha a autenticação para o `auth_service` via gRPC.
3. O `auth_service` cria ou valida o usuário no MongoDB e devolve um JWT.
4. O cliente abre `ws://localhost:8080/ws?token=<jwt>`.
5. O `gateway` valida o token e faz proxy da conexão para o `websocket_service`.
6. O `websocket_service` recebe a mensagem e grava o payload em um Redis Stream.
7. O `chat_service` consome o stream, persiste a mensagem no MongoDB e publica o resultado em Redis Pub/Sub.
8. O `websocket_service` escuta o canal Pub/Sub e entrega a mensagem para remetente e destinatário conectados.

## Serviços

### `gateway`

- Porta: `8080`
- Responsabilidades:
  - expor as rotas HTTP de autenticação
  - validar JWT para acesso ao WebSocket
  - encaminhar `/ws` para o `websocket_service`

Rotas disponíveis:

- `POST /auth/register`
- `POST /auth/login`
- `GET /ws?token=<jwt>`

### `auth_service`

- Porta: `50051` (gRPC)
- Responsabilidades:
  - registrar usuários
  - autenticar usuários
  - gerar JWT com claim `userId`
- Banco usado:
  - MongoDB em `mongodb://localhost:27019`, database `userdb`

### `websocket_service`

- Porta: `8081`
- Responsabilidades:
  - aceitar conexões WebSocket proxied pelo gateway
  - receber mensagens do cliente
  - publicar mensagens no Redis Stream
  - escutar Redis Pub/Sub e reenviar para os clientes conectados

Formato da mensagem enviada pelo cliente:

```json
{
  "type": "chat_message",
  "payload": {
    "sender_id": "user-a",
    "receiver_id": "user-b",
    "content": "ola"
  }
}
```

### `chat_service`

- Não expõe API HTTP no estado atual
- Responsabilidades:
  - consumir mensagens do Redis Stream
  - persistir no MongoDB
  - publicar o resultado em Redis Pub/Sub
- Banco usado:
  - MongoDB em `mongodb://localhost:27018`, database `chatdb`

### `presence_service`

Existe apenas como estrutura inicial e ainda não faz parte do fluxo principal.

Hoje ele:

- tem `go.mod` próprio
- possui código inicial de conexão com Redis
- não está integrado ao `go.work`
- não está sendo iniciado por `run.go`
- não está concluído e não deve ser tratado como funcional

## Estrutura

```text
.
├── services/
│   ├── auth_service/
│   ├── chat_service/
│   ├── gateway/
│   ├── presence_service/
│   └── websocket_service/
├── proto/
├── tests_e2e/
├── docker-compose.yml
└── run.go
```

## Requisitos

- Go `1.25.1`
- Docker
- Docker Compose

## Dependências Locais

Suba Redis e os dois MongoDBs:

```bash
docker-compose up -d
```

Containers esperados:

- `redis` em `localhost:6379`
- `mongo_chat` em `localhost:27018`
- `mongo_user` em `localhost:27019`

## Variáveis de Ambiente

Para o fluxo de mensagens funcionar, defina:

```bash
export REDIS_STREAM_CHAT=chat.message.created
export REDIS_CHANNEL_CHAT=chat.message.persisted
```

Opcional:

```bash
export REDIS_URL=localhost:6379
```

Observações:

- `auth_service` e `chat_service` usam hosts fixos para MongoDB no código atual.
- `gateway` usa `localhost:50051` para gRPC e `http://localhost:8081` para o WebSocket service.
- o `presence_service` lê `REDIS_CHANNEL_PRESENCE`, mas ainda não está finalizado.

## Como Rodar

### Opção 1: iniciar tudo que já participa do fluxo principal

```bash
go run run.go
```

O `run.go` sobe:

- `auth_service`
- `chat_service`
- `gateway`
- `websocket_service`

### Opção 2: iniciar manualmente

Em terminais separados:

```bash
go run ./services/auth_service/cmd
go run ./services/chat_service/cmd
go run ./services/websocket_service/cmd
go run ./services/gateway/cmd
```

## Testes

Os testes E2E ficam em [`tests_e2e`](/home/user/GoMessenger/tests_e2e).

Cobertura atual:

- registro de usuário
- login via fallback quando o usuário já existe
- troca de mensagem entre dois usuários conectados por WebSocket

Para rodar:

```bash
cd tests_e2e
go test ./...
```

Os testes dependem dos serviços estarem ativos localmente.

## Limitações Conhecidas

- `presence_service` está incompleto
- o README antigo citava observabilidade, rate limiting e notification service, mas isso não está implementado neste repositório hoje
- o `chat_service` funciona como worker e não expõe endpoint HTTP, apesar de o nome do método `Start` sugerir servidor
- há configuração hardcoded em vários pontos
- o JWT usa chave fixa em código
- o WebSocket recebe `sender_id` no payload do cliente, sem sobrescrever pelo usuário autenticado

## Próximos Passos

O backlog sugerido está em [`TODO.md`](/home/user/GoMessenger/TODO.md).
