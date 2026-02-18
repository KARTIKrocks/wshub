# The Architecture

```
                  ┌─────────────────────────────────┐
                  │         GATEWAY LAYER           │
Client A ──WS────▶│  Node 1                         │
Client B ──WS────▶│  (just holds connections,       │──publish──▶ Kafka
Client C ──WS────▶│   no business logic)            │
                  └─────────────┬───────────────────┘
                                │ subscribe (outbound)
                                │
                  ┌─────────────▼───────────────────┐
                  │        BACKEND LAYER            │
                  │  Service A  Service B  Service C│◀──consume── Kafka
                  │  (stateless, processes messages)│
                  └─────────────────────────────────┘
```

## Inbound — Client → Backend

Simple. Gateway is just a pipe:

1. Client sends message
2. Gateway receives it
3. Gateway publishes raw event to Kafka:

   ```json
   {
     "clientID": "...",
     "userID": "...",
     "payload": "...",
     "timestamp": "..."
   }
   ```

4. Backend service consumes from Kafka
5. Backend processes it (business logic lives here)

### Gateway Handler (Minimal)

```go
hub := wshub.NewHub(
    wshub.WithMessageHandler(func(client *wshub.Client, msg *wshub.Message) error {
        return kafka.Publish("gateway.inbound", Event{
            ClientID: client.ID,
            UserID:   client.GetUserID(),
            Data:     msg.Data,
        })
    }),
)
```

---

## Outbound — Backend → Client (The Hard Direction)

This is where it gets interesting.

Backend processed the message and needs to send a response back.
But it doesn't hold any connections — the gateway does.

---

### Step 1 — Connection Registry

When a client connects to the gateway, the gateway registers it:

```
Redis: SET conn:{clientID}  →  "gateway-node-1"
Redis: SET conn:{userID}    →  ["gateway-node-1", "gateway-node-3"]
```

---

### Step 2 — Backend Publishes Outbound Message

1. Backend looks up:

```
conn:{clientID} → "gateway-node-1"
```

2. Backend publishes to:

```
Kafka topic: "gateway.outbound.node-1"
```

```json
{
    "clientID": "...",
    "payload": "..."
}
```


### Step 3 — Gateway Delivers

Each gateway node subscribes only to its own outbound topic.

```go
// gateway-node-1's Kafka consumer
for msg := range consumer.Messages("gateway.outbound.node-1") {
    hub.SendToClient(msg.ClientID, msg.Payload)
}
```

---

# Why This Is Powerful

| Concern              | Who Owns It                  |
| -------------------- | ---------------------------- |
| Connection lifecycle | Gateway                      |
| Authentication       | Gateway (BeforeConnect hook) |
| Message routing      | Kafka                        |
| Business logic       | Backend services             |
| Scaling              | Each layer independently     |

### Operational Benefits

- Gateway goes down → clients reconnect, backend services unaffected
- Backend goes down → gateway keeps connections alive, queues in Kafka
- Traffic spike → scale backend services without touching gateway
- Deploy new business logic → zero effect on active connections

---

# How Our Library Fits in This Architecture

Our library is exactly the **gateway component**.

You would use it like this:

```
wshub (gateway)
  ├── WithHooks.BeforeConnect   → authenticate the HTTP upgrade request
  ├── WithMessageHandler        → publish to Kafka, nothing else
  ├── WithHooks.AfterConnect    → register clientID → nodeID in Redis
  ├── WithHooks.AfterDisconnect → remove from Redis registry
  └── Kafka consumer goroutine  → calls hub.SendToClient() for outbound
```

### Important Design Principle

`WithMessageHandler` **never processes a message** in this model — it just forwards.

The Router we built would live in the **backend service**, not the gateway, since that’s where dispatch to business logic handlers happens.

---

# What Discord Specifically Does

Discord's gateway is intentionally minimal:

- Handles the WebSocket protocol and heartbeats
- Enforces rate limits (WithLimits equivalent)
- Publishes every event to their internal bus
- Subscribes to per-session outbound channels

Their backend "dispatch" service routes events to the correct gateway node using the connection registry.

The gateway itself has **no idea** what a "message" or "guild" is — that’s entirely backend concern.

---

### Why This Matters

This is why Discord’s gateway can handle **millions of connections per node** — it does almost no work per message.
