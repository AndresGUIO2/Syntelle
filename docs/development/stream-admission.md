# Streaming admission controls

The owner selected request/connection throttling by visitor and IP, plus a maximum number of active render sessions per visitor. These controls are implemented in `services/api/internal/streaming/admission.go` and apply to `/api/v1/streams` and its public HTTP/WebSocket routes. Existing loopback/Origin restrictions, private worker authentication and global render capacity still apply. This change does not expose a public service.

## Behavior

- HTTP streaming requests share an IP allowance. Authenticated requests additionally consume an identity allowance. Guest browser sessions have separate identities; all browser sessions and integration keys belonging to the same account share its identity.
- WebSocket connection attempts are limited before upgrade. Open sockets count against the IP and process limits while waiting for their authentication ticket, including failed handshakes. After authentication, identity connection limits also apply. Closing or rejecting a connection releases its reservation exactly once.
- WebSocket control messages share the HTTP identity/IP request allowances. PCM audio frames and microphone activity notifications are continuous media rather than API requests; microphone start/stop events consume the allowance. Existing message/frame size and queue bounds remain in force.
- Excess HTTP requests return `429` with `Retry-After`. Excess control messages receive an error event with `retry_after`; excess post-upgrade authenticated connection attempts close with a policy violation.
- The default visitor allowance is one active render session. Session admission checks and insertion run under the same mutex, so simultaneous creates cannot bypass this limit. A second session is rejected before database creation, conversation setup or worker delivery. Closing, revoking or expiring a session releases the visitor's slot. Existing player reconnection reuses its stream and can briefly overlap its preceding signaling socket.
- Audio/signaling traffic now also triggers bounded periodic authorization rechecks, so it cannot bypass expiry merely by omitting heartbeat messages. This preserves existing explicit revocation callbacks and the 30-second transport inactivity cleanup.

## Configuration

All settings are server-owned and supplied through the environment. Zero/negative explicit environment limits are rejected. Omitted programmatic values use these defaults:

| Environment variable | Default | Meaning |
| --- | ---: | --- |
| `SYNTELLE_STREAM_REQUESTS_PER_MINUTE` | 600 | Refill rate per visitor/account; burst at most 60 |
| `SYNTELLE_STREAM_IP_REQUESTS_PER_MINUTE` | 3000 | Refill rate per client IP; burst at most 60 |
| `SYNTELLE_STREAM_CONNECTIONS_PER_MINUTE` | 30 | Connection-attempt refill rate, independently per IP and authenticated identity; burst at most 5 |
| `SYNTELLE_STREAM_CONNECTIONS_PER_IP` | 8 | Simultaneously open player signaling sockets per IP, including unauthenticated sockets |
| `SYNTELLE_STREAM_CONNECTIONS_PER_PRINCIPAL` | 2 | Simultaneously open sockets per identity, allowing reconnect overlap |
| `SYNTELLE_STREAM_SESSIONS_PER_PRINCIPAL` | 1 | Active render sessions per visitor/account |
| `SYNTELLE_STREAM_SESSION_LIMIT_ENABLED` | true | Enforce the visitor/session cap; local Compose and `.env.example` explicitly set false for multiple development tabs |
| `SYNTELLE_STREAM_TRUSTED_PROXY_CIDRS` | empty | Comma-separated trusted ingress CIDRs |

There is an additional process-wide ceiling of 64 player sockets. Rate state holds at most 8,192 buckets, expires inactive entries after two minutes and fails closed when full. This uses existing Go synchronization without a new dependency, service, database schema or cleanup goroutine. These are access limits, not speech/model quotas or a monetary spending cap. Legacy local-owner conversation and speech endpoints retain their existing policy.

The owner explicitly disabled the per-visitor session cap for local development. The local switch does not bypass renderer capacity, request/connection limits, authentication or the [message inactivity policy](stream-inactivity.md). Outside that explicit local configuration, the constructor and absent environment switch continue to enforce one active session by default.

## Client IP and deployment boundary

By default the API uses the normalized socket peer IP and ignores forwarded-IP headers. When the immediate peer matches an explicitly configured trusted proxy, `X-Forwarded-For` is examined from right to left, stripping only trusted proxy hops. Invalid or oversized forwarding data falls back to the immediate peer. Universal `/0` trust is rejected. Never configure arbitrary client networks as trusted ingress.

Actual Azure/Cloudflare ingress must be verified and its trusted network configured before public deployment. A shared proxy/NAT otherwise shares an IP allowance; that must not be mistaken for separate visitor identities.

Admission state is local to one API process and resets on restart. This matches the current single-process worker/session coordinator. Multiple API replicas require coordinated admission and session routing before these can be advertised as global limits. Anonymous cookie rotation does not create a durable person identity; IP/process controls remain additional bounds. Distributed protection, usage budgets, GPU leases, paid provider caps and public deployment are separate work.

## Verification

Focused tests cover forged forwarding headers, trusted proxy chains, shared identity/IP allowances, refill, bounded state, simultaneous requests/session creation, pre-authentication socket admission/release, and HTTP/WebSocket rejection before downstream work. The existing PostgreSQL streaming integration fixture additionally verifies that a second stream creates no record, revocation releases the slot, and expired authorization closes a non-heartbeat session even after its request allowance is exhausted. Rejected control messages do not cause an extra authorization query per message.

All portable API tests passed. The streaming suite, including PostgreSQL integration, passed with the Go race detector in a disposable container with external networking disabled. Frontend type checking and formatting of the added Spanish messages passed. Those initial checks used fixtures without changing the owner's databases or running services. The subsequent [inactivity/local-development correction](stream-inactivity.md) rebuilt the local API/web, preserved the installed renderer image/assets, and verified two same-visitor idle-video sessions without provider calls.

Source copies before this change are in `.tools/stream-admission-2026-10-22/before/`. No character assets, preparation evidence or provider allowances are changed.
