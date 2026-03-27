# TODO

Backlog based on the current codebase state.

## High Priority

### 1. Finish `presence_service`

- define the presence contract: `online`, `offline`, `last_seen`, `current_chat_id`
- fix `services/presence_service/cmd/server.go`, which is currently incomplete
- decide whether presence should be exposed through HTTP, gRPC, or only Redis
- publish presence events to a dedicated Redis channel
- register connect and disconnect events from the `websocket` service

### 2. Improve message processing reliability

- review Redis Stream consumption in the `chat` service
- evaluate consumer groups instead of plain `XRead`
- avoid message loss or incorrect reprocessing if a failure happens mid-flow
- add idempotency for message persistence

## Medium Priority

### 5. Expand tests

- add unit tests for `auth`, `chat`, and `websocket`
- cover invalid token and unauthenticated connection scenarios
- test WebSocket reconnect and disconnect flows
- cover Redis and MongoDB failure scenarios
- validate the future presence flow with end-to-end tests

### 6. Improve security

- move the JWT secret to a required environment variable
- improve HTTP and WebSocket payload validation
- review error messages and HTTP status handling
- consider expiration, refresh tokens, and secret rotation
- restrict WebSocket `CheckOrigin`

### 7. Improve local runtime ergonomics

- resolve default port conflicts between services where applicable
- decide whether `chat` should remain a pure worker or expose a healthcheck
- add healthchecks and readiness
- create bootstrap scripts or a fuller compose setup for local development

## Low Priority

### 8. Observability

- structured logging
- per-service metrics
- tracing for the `gateway -> websocket -> redis -> chat` flow

### 9. Product features

- conversation history per user
- delivery and read acknowledgements
- conversation list
- message pagination
- notifications

## Technical Notes Found

- `presence_service` is still not integrated into the main flow
- `tests_e2e/chat_test.go` is empty
- `tests_e2e/main_test.go` is empty
- there is hardcoded configuration in several places
- the old documentation mixed implemented features with backlog items

## Suggested Implementation Order

1. finish `presence_service`
2. fix sender identity in the WebSocket flow
3. externalize configuration
4. strengthen tests
5. add observability
