# Go service organization and reliability

The October 20, 2026 change organizes the existing Go executable and corrects three reproduced conversation/speech failures. Package boundaries change; the Vue/Go/FastAPI services, hosting decisions, WebSocket/SSE boundaries, providers and dependency versions do not.

## Package responsibilities

Rendering and media now live in the independent [render-worker module](renderer-service-boundary.md). The API retains coordination and shares only the dependency-free wire/settings contract.

From `services/api`, the executable entry point is now `./cmd/api`:

```text
cmd/api/                   Configuration, wiring and shutdown-aware composition
internal/httpapi/          Image validation, JSON responses, local access and HTTP lifecycle
internal/conversation/     Socket/turn orchestration, model adapters and conversation storage
internal/speech/           Native/stub voice providers, ledgers and conversation adapters
go.mod, go.sum, Dockerfile Module and container configuration
```

Tests remain beside the behavior they cover. A flat Go package was valid for the original small build; these folders now identify concrete responsibilities rather than adding handler/service/repository layers. The command wires the packages explicitly. HTTP helpers import neither feature package; conversation uses HTTP access rules; speech adapters implement conversation contracts and reuse HTTP response helpers. There are no import cycles or DI containers.

The command only exports a binary. Internal exports are limited to constructors, service/provider/ledger contracts, speech input/results, and HTTP functions used across package boundaries. Native `speech && linux` and default stub selection stay intact. The existing `speech.sql` is embedded from `internal/speech` with identical bytes; conversation migration bytes, table names, budget identifiers and operation formats are unchanged.

Historical `.tools` paths and frozen sources remain where they were recorded. Do not rewrite old source hashes or rerun historical finalizers to account for this relocation. In particular, older speech finalizers refer to the original SQL/source paths and describe their frozen experiments. Current Docker builds copy `cmd` and `internal` and exclude `.tools` from the build context.

## Corrected behavior

### Output bursts

The output queue retains 64 events and a single socket writer. A full queue now waits for space for up to five seconds, matching the existing write timeout. Sends belong to their producer context: a cancelled turn stops waiting without closing its conversation. Sustained congestion or writer failure still closes the connection. No fragments are discarded or reordered and no queue size is increased.

### Local speech access

The existing Host/Origin rules cover both owner and historical trial speech routes, including browser-simple `text/plain` writes containing JSON. Rejected requests reach neither cache/reservation nor provider. Allowed origins, read-only status behavior and disabled speech responses for authorized clients remain unchanged. The shutdown-aware owner handler stays outside the access middleware so upgraded connections are still drained.

### Interruption persistence

Cancellation stops active work and publishes `cancelled` before attempting the existing one-second interruption write. A rejected write emits this additive, nonfatal socket event:

```json
{
  "type": "persistence_warning",
  "conversation_id": "the-existing-conversation-id",
  "turn_id": "the-interrupted-turn-id",
  "text": "Playback stopped, but this change could not be saved."
}
```

The Vue conversation composable handles this event before filtering obsolete turn IDs, while retaining its current-conversation/disposal checks. The existing message area displays the warning without changing pending work, turn status or playback. An existing error takes precedence. Other HTTP and socket representations are unchanged.

The server logs `interruption_archive_failed`, correlation IDs and a bounded classification (`database`, `deadline` or `cancelled`). It logs no raw database exception, message text, microphone audio, credential or connection string. A disconnected client may not receive a warning; the log still records the failure. No durable outbox, provider retry or historical-record repair is introduced.

## Local commands and bounded verification

From `services/api`:

```powershell
go test ./...
go build -o syntelle-api ./cmd/api
```

The default build uses the existing stub where native Speech is unavailable. The pinned Linux build runs `go test -tags speech ./...` and builds `./cmd/api` with the existing SDK/linker settings. These commands do not opt into live recognition or local-service verification.

Verification for this delivery uses a separate disposable PostgreSQL instance and simulated model, speech and recognition providers. The Linux test runner shares only that database's network namespace, which has external networking disabled; no private environment file/provider credential is passed. Test databases verify their identities before fixture mutation. The fault-injection constraint exists only in a disposable database.

The Go suite passes with race detection and native build tags; the default/stub suite and executable build also pass. Two explicitly gated checks (live recognition and the existing running-service proxy check) are skipped. Regressions cover a complete ordered 200-fragment reply with one model/voice call, bounded queue cancellation/congestion, owner/trial access before reservation, interruption warning ordering and shutdown composition. Existing tests retain coverage of successful interruption persistence, real-byte cache recovery and unchanged historical allowances using fixtures.

Frontend type checking and scoped ESLint pass. Two Chromium desktop fixture checks cover normal conversation cancellation/Read aloud and late warnings during a subsequent turn, primary-error precedence, unrelated-session rejection and stale audio/text suppression. Their API routes are intercepted; the separate Vite test server points its proxy at an unavailable local port rather than the owner's API.

Evidence is in `.tools/go-reliability-33/`: original source copies, the 19-file relocation map, protected-file hashes, native test output and the two browser results. This is offline code verification, not a new provider or device trial. No existing database record, allowance, character resource, dependency manifest or archived experiment is changed. No deployment or running-service rebuild is performed by these checks.
