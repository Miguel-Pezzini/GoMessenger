# TODO

Feature backlog based on the current system state.

Already shipped and not repeated here: JWT auth, friend requests, direct chat, cursor-paginated history, file/image attachments, typing indicators, chat-open presence, delivery/read receipts, in-app notification routing, audit logging, and admin log/presence views.

## Priority 1 - Most Desirable Next Features

### Conversation inbox

Why it matters: users should open the app and immediately see the chats that need attention, not only a friend list.

- add a `GET /conversations` API with friend profile, last message, unread count, last activity, and presence summary
- persist per-user conversation state: unread count, last read message, archived, muted, pinned, and custom nickname
- update the frontend sidebar to use conversations ordered by recent activity
- keep conversation state in sync from WebSocket message, read, friend, and presence events

### User discovery and profile basics

Why it matters: starting a chat should be easy without manually exchanging opaque IDs.

- add user search by username and friend code
- add profile fields: display name, avatar, short bio, and optional status text
- expose profile lookup with friendship/request state
- support avatar upload through the existing media service

### Message reliability and reconnect sync

Why it matters: chat needs to feel trustworthy when a tab refreshes, a phone sleeps, or the network drops.

- add client-generated `client_msg_id` to deduplicate retries and reconcile optimistic messages
- add event sequence or cursor support so clients can request missed events after reconnect
- make WebSocket reconnect restore selected chat, unread state, delivery state, and pending sends
- document retry behavior for message send, attachment upload, and receipt publishing

### Blocking, privacy, and abuse controls

Why it matters: social features need safety controls before broader sharing or group chat.

- add block and unblock APIs
- enforce blocks in friend requests, direct messages, notifications, presence visibility, and profile lookup
- add privacy settings for who can send requests and who can see online status
- add report user and report message endpoints backed by audit logs
- add rate limits for friend requests, login attempts, and message sending

## Priority 2 - Core Chat Polish

### Message actions

- edit a message within a short time window
- delete a message for self
- delete a message for everyone while allowed
- reply to a specific message with an original-message snapshot
- forward a message or attachments to another conversation
- pin or star important messages

### Reactions and lightweight expression

- add emoji reactions with per-message summaries
- publish reaction updates over WebSocket
- show reaction counts and the current user's reaction in history responses
- consider stickers and GIF support after reactions are stable

### Search and shared content

- add message search by keyword within one conversation
- add global message search across direct conversations
- add shared media, links, and files views per conversation
- generate link previews for common URLs

### Notification center and preferences

- persist user-visible notifications, not only transient WebSocket events
- add mark-as-read and clear notification APIs
- add per-event notification preferences
- add batching rules to avoid noisy repeated notifications
- prepare the pipeline for offline push notifications

## Priority 3 - Account and Personalization

### Account and session management

- add refresh tokens and logout
- list active sessions/devices
- revoke one session or log out from all devices
- support password change and password reset flow
- make token expiry and rotation behavior visible to the frontend

### Conversation personalization

- mute and unmute conversations
- archive and unarchive conversations
- pin conversations in the inbox
- save local drafts per conversation
- add per-friend nicknames
- add theme or appearance preferences

### Media experience upgrades

- generate image/video thumbnails
- add upload progress and resumable upload behavior in the frontend
- support voice notes
- add attachment retention and cleanup policies visible to users
- validate and display richer file metadata

## Priority 4 - Group Chat Expansion

### Group conversations

- create group conversations
- invite and remove members
- add member roles: owner, admin, member
- add group profile fields: name, avatar, description
- emit system messages for joins, leaves, renames, and role changes

### Group realtime behavior

- fan out messages to all group members
- support group delivery and read receipts
- add mentions and mention notifications
- add group mute and notification preferences
- define group-specific abuse and moderation actions

## Priority 5 - Moderation, Retention, and Export

### Moderation tools

- build an admin review queue for reports
- connect reports to audit logs and message/user snapshots
- add moderation actions: warn, restrict messaging, suspend account
- add admin filters by user, event type, and time range

### Data controls

- export direct conversation history
- define retention rules for deleted, expired, and reported content
- add account deletion with data cleanup policy
- document what data is retained for safety and audit requirements

## Feature Enablers

These are not the main product roadmap, but they unblock the feature work above.

- add health and readiness endpoints for every service
- add Prometheus metrics for gateway, websocket, chat, media, notification, and presence
- add per-connection WebSocket writer queues so slow clients do not block fan-out
- add reconnect/load-test scenarios for WebSocket, chat, notification, and media flows
- keep production configuration strict: required secrets, explicit allowed origins, and no permissive defaults
- update stale docs that still reference missing files or pre-media/presence gaps
