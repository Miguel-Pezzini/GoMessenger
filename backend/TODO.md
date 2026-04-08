# TODO

Backlog based on the current codebase state.

## Medium Priority

### 3. Improve local runtime ergonomics

- resolve default port conflicts between services where applicable
- decide whether `chat` should remain a pure worker or expose a healthcheck
- add healthchecks and readiness
- create bootstrap scripts or a fuller compose setup for local development

## Low Priority

### 4. Observability

- structured logging
- per-service metrics
- tracing for the `gateway -> websocket -> redis -> chat` flow

### 5. Product features

- block and unblock users
- user search by username
- profile basics: avatar
- direct message deletion for self
- edit message within a short time window
- reply to specific message
- forward message to another conversation
- send images and file attachments
- link preview generation
- emoji reactions on messages
- pin important messages in a conversation
- message search by keyword
- shared media / links / files tab per conversation
- archived and muted conversations
- message-level spam / abuse report

### 6. Product feature ideas by milestone

#### Social and account features

- onboarding flow for new users after registration
- suggested friends based on mutual connections
- user privacy settings: who can send requests, who can see presence
- user profile page with friendship state and shared friends count
- account session management and logout from all devices

#### Conversation experience

- recent conversations ordered by last activity
- per-conversation draft support
- resend failed messages from the client flow
- system messages for friendship accepted, blocked, or removed
- conversation-level settings such as mute, archive, and custom nickname
- jump to first unread message
- day separators and message grouping metadata

#### Realtime engagement

- presence-aware "currently in chat with you" state
- typing indicator with timeout and debounce semantics
- delivery lifecycle states: sent, persisted, delivered, seen
- reconnect sync so a client receives missed events after reconnecting
- multi-device fan-out so the same user can be connected on phone and desktop

#### Safety and trust

- block list API and enforcement across chat and friend requests
- rate limiting for friend requests and message sending
- basic anti-spam heuristics for repeated unsolicited messages
- report user and report message endpoints
- audit trail for moderation-sensitive actions

#### Content and media

- attachment metadata model and upload flow
- voice notes
- stickers and GIF support
- message reactions summary
- rich preview cards for supported URLs
- quoted replies with original message snapshot

#### Retention and discovery

- full conversation search
- filter conversations by unread, archived, or direct friend status
- star / save messages
- export conversation history
- retention rules for ephemeral or auto-expiring messages

#### Notifications

- offline push notification event pipeline
- notification preferences per event type
- batched notifications to avoid spam
- mark notifications as read
- in-app notification center

#### Group-chat expansion path

- create group conversations
- invite and remove members
- group roles such as owner and admin
- group system events: member joined, left, promoted
- group read receipts and mention notifications

#### Monetizable or premium-ready ideas

- premium-only large file uploads
- custom themes or chat personalization settings
- vanity usernames
- boosted storage / history retention
- business or community accounts with profile badges

## Technical Notes Found

- `presence_service` is only partially integrated today: connect and disconnect flow exists, but chat-open, typing, and read feedback are not wired yet
- presence and typing contracts are now documented in `docs/realtime-chat-feedback.md`
- there is hardcoded configuration in several places
- the old documentation mixed implemented features with backlog items

## Suggested Implementation Order

1. finish `presence_service`
2. ship the independent logging service
3. fix sender identity in the WebSocket flow
4. externalize configuration
5. strengthen tests

## Scalability and Load Testing

The goal here is to prepare the system for realistic high-concurrency benchmarks first, and only then chase bigger numbers like `10k`, `50k`, or `100k` connected users.

### 7. Add per-connection outbound queues and writer pumps

Why this matters:

- even after reducing lock contention, direct synchronous writes still let slow clients hurt throughput
- a dedicated write loop per client is the usual pattern for high-volume WebSocket servers

What to do:

- extend each connected client with a buffered outbound channel
- create one writer goroutine per client that is the only place allowed to write to the socket
- push outbound messages into the client queue instead of calling `WriteJSON` directly from listeners
- decide what happens when the queue is full: drop the client, drop messages, or backpressure for a limited time
- set write deadlines so dead or slow connections are detected quickly
- expose counters for queue depth, dropped messages, slow clients, and forced disconnects
- add unit tests for queue full, disconnect, and concurrent fan-out behavior

Definition of done:

- all outbound socket writes happen through the per-client writer pump
- one slow or broken client no longer blocks unrelated deliveries

### 8. Parallelize chat stream consumption safely

Why this matters:

- the chat stream consumer currently processes messages one by one
- this limits throughput and will likely become a bottleneck before the WebSocket layer is fully saturated

What to do:

- make worker concurrency configurable for the chat consumer
- process stream messages concurrently while keeping the existing ack-after-persist-and-publish behavior
- preserve idempotency using `stream_id` so retries and multiple consumers stay safe
- validate that multiple chat service instances can join the same Redis consumer group without duplicate persistence
- add integration coverage for multiple chat consumers reading from the same stream
- record stream lag and message processing duration so load tests show where backlog starts

Definition of done:

- the chat service can run with more than one worker or more than one instance
- throughput improves without breaking persistence correctness or message deduplication

### 9. Add observability needed for benchmarks

Why this matters:

- load tests without metrics only tell us that the system got slow or crashed
- we need to see where the bottleneck is: gateway, websocket, Redis, chat, or MongoDB

What to do:

- add `pprof` endpoints to the services that will be benchmarked first
- add metrics for active WebSocket connections, connection rate, disconnect rate, outbound queue depth, dropped writes, Redis publish latency, Redis stream lag, MongoDB persistence latency, and end-to-end message latency
- add a client-generated correlation field such as `client_msg_id`
- add an optional client timestamp such as `client_sent_at` so end-to-end latency can be measured during benchmarks
- make benchmark metrics available per service instead of only in shared logs

Definition of done:

- benchmark runs can report `p50`, `p95`, and `p99` connection and message-delivery latency
- the team can identify the first bottleneck from metrics and profiles instead of guessing

### 10. Create a repeatable load-test harness

Why this matters:

- we need a standard way to simulate high concurrency instead of ad hoc scripts
- the benchmark should be repeatable when code changes

What to do:

- create a dedicated load-test harness, preferably `cmd/loadtest` or `tests/load`
- support at least four scenarios: HTTP burst, connect storm, steady messaging, and long-running soak
- pre-create users and JWTs before the main benchmark so auth does not distort chat measurements unless auth is the thing being tested
- make user count, active sender ratio, message rate, payload size, and test duration configurable
- export results in a machine-readable format for later comparison between runs
- document how to run the load tests locally and how to run them across multiple generator machines

Definition of done:

- one command can reproduce the main benchmark scenarios
- results can be compared across commits and infrastructure changes

### 11. Benchmark the distributed deployment, not just `cmd/dev`

Why this matters:

- `cmd/dev` is fine for local development, but it is not a good benchmark target
- when every service runs on one machine, the benchmark mostly measures local CPU and memory contention

What to do:

- define a benchmark environment where `gateway`, `websocket`, and `chat` can run as separate processes or containers
- support running multiple `websocket` and `chat` instances during load tests
- tune OS limits before large connection tests, especially file descriptors and TCP backlog
- tune Redis and MongoDB client pools for the benchmark environment
- document the minimum hardware and system settings required for `10k+` and `100k` connection experiments

Definition of done:

- the benchmark environment is close enough to production-style deployment to make the results meaningful
- scaling tests can increase the number of service instances instead of only stressing one local process

## Recommended Order for Scalability Work

1. fix WebSocket fan-out lock contention
2. add per-connection outbound queues and writer pumps
3. add observability needed for benchmarks
4. parallelize chat stream consumption safely
5. create a repeatable load-test harness
6. benchmark the distributed deployment, not just `cmd/dev`
