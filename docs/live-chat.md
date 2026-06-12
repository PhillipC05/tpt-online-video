# Live Chat

Real-time chat for live streams over WebSocket. Each stream has an independent room. Messages are persisted in PostgreSQL and broadcast across API instances via Redis pub/sub.

---

## Architecture

```
Browser (WS)  →  Chat Handler (HTTP Upgrade)
                      │
              ChatHub (in-memory room registry)
                      │
               Redis pub/sub  ←──→  Other API instances
                      │
              ChatRepository (Postgres)
```

- **ChatHub** manages per-stream rooms and Redis subscriptions. When the first viewer joins a room, the hub subscribes to `chat:stream:{streamID}` in Redis. When the last viewer leaves, the subscription is torn down.
- **ChatService** enforces bans, timeouts, chat lock, and message length before persisting and publishing.
- The WebSocket protocol is implemented from scratch (RFC 6455) — no external library required.

---

## WebSocket Endpoint

```
GET /api/v1/live/streams/{streamID}/chat/ws
Upgrade: websocket
```

Authentication is optional. Unauthenticated clients can read messages but cannot send. Pass a JWT via the `Authorization` header or the `?token=` query parameter (the browser WebSocket API cannot set headers, so the query param is the standard approach).

On connect the server immediately sends a `history` event with the 50 most recent messages, followed by a `joined` event.

### Client → Server frames (JSON text frames)

| Type | Fields | Description |
|------|--------|-------------|
| `message` | `body: string` | Send a chat message (authenticated only) |
| `ping` | — | Keep-alive; server replies with `pong` |

### Server → Client events

| Type | `data` fields | Description |
|------|---------------|-------------|
| `joined` | `stream_id`, `user_id`, `display_name` | Sent once on connect |
| `history` | `messages: ChatMessage[]` | Last 50 messages sent on connect |
| `message` | `ChatMessage` | New message from any user |
| `deleted` | `message_id: string` | A message was soft-deleted |
| `chat_locked` | `locked: true` | Streamer locked chat |
| `chat_unlocked` | `locked: false` | Streamer unlocked chat |
| `banned` | `stream_id` | Current user was banned |
| `timed_out` | `stream_id`, `expires_at` | Current user was timed out |
| `timeout_removed` | `stream_id` | Current user's timeout was lifted |
| `error` | `message: string` | Something went wrong (e.g. banned, chat locked) |
| `pong` | — | Reply to a client `ping` |

### ChatMessage shape

```json
{
  "id": "uuid",
  "stream_id": "uuid",
  "user_id": "uuid | null",
  "display_name": "Alice",
  "body": "Hello!",
  "deleted": false,
  "created_at": "2026-06-12T10:00:00Z"
}
```

Deleted messages have `deleted: true` and `body: ""`.

---

## REST Endpoints

### Message history

```
GET /api/v1/live/streams/{streamID}/chat/messages
Query: before={messageID}, limit={1-100}
Auth: none required
```

Returns messages in newest-first order. Use `before` for cursor-based pagination.

---

## Moderation Endpoints

All moderation endpoints require authentication. The caller must be either the **stream owner** or a **moderator/admin**.

### Delete a message

```
DELETE /api/v1/live/streams/{streamID}/chat/messages/{messageID}
Auth: owner or mod/admin
```

Soft-deletes the message and broadcasts a `deleted` event to all viewers.

### Timeout a user

```
POST /api/v1/live/streams/{streamID}/chat/users/{userID}/timeout
Body: { "duration_seconds": 300 }
Auth: owner or mod/admin
```

Mutes the user for the given duration. The affected user receives a `timed_out` event. Default is 300 s (5 min); max is 86 400 s (24 h).

### Remove a timeout

```
DELETE /api/v1/live/streams/{streamID}/chat/users/{userID}/timeout
Auth: owner or mod/admin
```

### Ban a user

```
POST /api/v1/live/streams/{streamID}/chat/users/{userID}/ban
Body: { "reason": "optional reason" }
Auth: owner or mod/admin
```

Permanently bans the user from the stream's chat. The affected user receives a `banned` event.

### Unban a user

```
DELETE /api/v1/live/streams/{streamID}/chat/users/{userID}/ban
Auth: owner or mod/admin
```

### Lock / unlock chat

```
POST   /api/v1/live/streams/{streamID}/chat/lock    ← lock
DELETE /api/v1/live/streams/{streamID}/chat/lock    ← unlock
Auth: owner or mod/admin
```

When locked, the server rejects all incoming messages with an `error` event. All viewers receive a `chat_locked` / `chat_unlocked` broadcast.

### Report a message

Use the general report endpoint:

```
POST /api/v1/live/chat/{messageID}/report
Body: { "reason": "..." }
Auth: required
```

---

## Message rules

- Body must be 1–500 characters (whitespace trimmed).
- Users who are banned, timed out, or chatting in a locked room receive an `error` event; the message is not persisted or broadcast.
- Anonymous (unauthenticated) messages sent via WebSocket are silently ignored.

---

## Database schema

```sql
live_chat_messages (id, stream_id, user_id, display_name, body,
                    deleted, deleted_by, deleted_at, created_at)

live_chat_bans     (id, stream_id, user_id, banned_by, reason, created_at)
                   UNIQUE (stream_id, user_id)

live_chat_timeouts (id, stream_id, user_id, timed_out_by,
                    duration_seconds, expires_at, created_at)
                   UNIQUE (stream_id, user_id)

live_streams.chat_locked  BOOLEAN DEFAULT FALSE
```

Migration: `migrations/000010_live_chat.up.sql`

---

## Redis channels

| Channel | Purpose |
|---------|---------|
| `chat:stream:{streamID}` | Broadcast chat events to all API instances serving that room |

Payload is a JSON-encoded `ChatEvent` (`{"type":"...","data":{...}}`).

---

## Frontend

`LiveChat.tsx` connects on mount, reconnects automatically after 3 s on disconnect. It receives history on join, renders messages in chronological order, and reflects moderation events (`deleted`, `chat_locked`, `banned`, `timed_out`) in real-time without a page reload.

The component is embedded in `LiveWatchPage.tsx` alongside the video player in a responsive two-column grid that stacks to a single column on narrow viewports.

---

## Deferred

- **Typing indicator** — intentionally deferred. Would require ephemeral Redis keys (TTL ~3 s) and a debounced `typing` client→server event.
- **Emote/emoji picker** — not planned for initial release.
- **Chat history export** — may be added as an admin feature.
