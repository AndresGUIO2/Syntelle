# Local Syntelle accounts and rendered streaming

Current routing supersedes the original candidate arrangement: `/` and `/?view=stream` both use the [backend-only player](backend-only-player.md), with the latest microphone-only chat interface. Former client-demo and laboratory routes no longer activate browser rendering. Original source/evidence remains preserved. Only Syntelle is selectable in the public workspace.

The candidate deliberately extends the original demo: sign-in is optional, and account resources are separate from the local owner's historical preparation, conversations and provider allowances. It does not deploy public access or finish arbitrary portrait preparation.

The [native rendering follow-up](native-rendering.md) supersedes the historical Chromium worker, whose unused implementation and private web entry are now retired. Current build/start instructions use native Go only. Historical results below retain the original implementation's measurements. Public media routing and owner/device acceptance remain separate from selecting backend rendering.

## Service boundaries

Vue captures microphone input and plays a WebRTC stream. Go authorizes accounts/keys, owns stream/command identities, runs existing Gemini and Azure Speech orchestration and forwards signaling. A private Go worker renders the existing RGB-D character, including oral/eye layers and complete background/framing, and encodes native H.264/Opus. FastAPI retains image-preparation responsibilities.

The worker makes an outbound authenticated control connection to the Go API. Each stream owns its scene, encoders and Pion WebRTC peer; immutable prepared assets are shared privately. Consumer builds contain neither the engine nor its character materials. PCM-driven timing synchronizes speech, visemes and movement. Idle makes no AI request but still consumes rendering, encoding and bandwidth. Playwright/Chromium is used for browser verification, not worker rendering.

Default output is opaque 720 × 960 with a 30 fps target. The API's configured admission capacity is separate from measured worker concurrency. Public Internet ICE/TURN, hosting capacity, pricing and physical-device acceptance are not established by this local implementation.

The [streaming efficiency follow-up](streaming-efficiency.md) records the earlier browser worker's launch, codec and draw/capture improvements. Its source snapshots and measurements remain historical evidence; they do not describe the current runtime.

## Private configuration

Keep all credentials in the root `.env`, never `VITE_*`, screenshots, source snapshots or browser storage.

- `SYNTELLE_ACCOUNTS=1`, `SYNTELLE_STREAMING=1` enable the local candidate.
- `SYNTELLE_APP_ORIGIN=http://127.0.0.1:5173` is the canonical sign-in origin.
- `SYNTELLE_AUTH_SECRET` and `SYNTELLE_RENDER_WORKER_TOKEN` must be random private values of at least 32 characters.
- `SYNTELLE_PACKAGE_HASH` identifies the immutable Syntelle materials.
- `RESEND_API_KEY` and explicit `RESEND_FROM` configure email delivery. The owner must verify the sender's domain with Resend. The key alone does not establish sending readiness. Disable email open/click tracking for authentication messages in the Resend domain settings.
- `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`, `GITHUB_CLIENT_ID`, `GITHUB_CLIENT_SECRET` configure social login. No application resources are created automatically in those providers.

Create a Google OAuth **Web application**, configure the consent screen and register this exact authorized callback:

```text
http://127.0.0.1:5173/api/v1/auth/google/callback
```

Create a GitHub OAuth application with homepage `http://127.0.0.1:5173` and callback:

```text
http://127.0.0.1:5173/api/v1/auth/github/callback
```

Go requests identity/email permissions only, exchanges Authorization Codes with PKCE and validates state. Google additionally uses OIDC signature, issuer, audience and nonce validation. Provider access tokens are not persisted as user sessions.

Email uses random six-digit codes, ten-minute expiry, five attempts and single use. Sending is limited to one per 60 seconds, five/address/hour and twenty/IP/hour. These authentication safeguards do not reset or replace owner provider usage policies. Store a keyed challenge verifier, delivery ID/status and timestamps; never archive the code, rendered mail body, OAuth tokens or Resend authorization. Email failures do not trigger automatic retries.

Accounts have a unique verified login email; social identities use stable provider IDs. A new social identity matching an existing account requires explicit connection consent and fresh email/account proof. Missing/unverified social email must be verified. Two existing accounts are never silently merged. Ordinary accounts receive no owner authority.

Missing provider configuration is reported by `/api/v1/auth/status`. Unconfigured methods remain disabled; no development backdoor pretends to complete authentication.

## Run locally

The preferred complete-stack command is now `.\dev start` from the repository root. It starts/reuses the existing services and native renderer in the background and waits for readiness. Use `.\dev stop`, `.\dev status` and explicit `.\dev start --build` after source changes. See [local development commands](local-dev.md) for selective cleanup, cache/log retention and failure behavior. The lower-level setup commands below remain for initial provisioning and explicit worker inputs.

Use the repository's bundled Node executable or a compatible Node 24 runtime. From the repository root:

```powershell
# Records the source-bound package manifest; preserves existing private secrets.
node services/render-worker/scripts/configure-local.mjs

# Existing native Go/Azure Speech container and local PostgreSQL.
docker compose --profile preparation up -d --build postgres processor api web

# Private native build is separate from the public player.
npm --prefix services/render-worker run build
npm --prefix services/render-worker run start
```

The native worker requires the existing offline resource export. Follow the [worker setup](../../services/render-worker/README.md) for prepared assets, the pinned Linux build and optional existing-volume input.

Open `http://127.0.0.1:5173/?view=stream`. Select Enable audio when needed; microphone capture always requires explicit activation. Hidden tabs stop capture and the stream; returning does not restart the microphone. Stop reply suppresses local audio immediately while server cancellation discards obsolete output. A dropped control connection expires after 30 seconds without a heartbeat. Stream/provider work participates in the existing five-second shutdown deadline.

The configuration command derives a hash from cached Syntelle resources only. It generates no image, depth or provider result. Source/package records live in `.tools/accounts-streaming-49/package-manifest.json`.

Build the consumer-only distribution from `apps/web` with `npm run build:player`. A build gate rejects private renderer modules, character textures and geometry. This is a local review artifact, not authorization to deploy the uncapped owner service.

## Account and integration behavior

Signing in does not save guest work automatically. Use Your account → Saved setups → Save to claim only the current Syntelle instructions, voice/language and appearance settings. Setups can be loaded, updated and deleted. Saved setups do not expire; temporary guest configurations expire after 24 hours. Cross-device conversation-history restoration is not part of this version.

Your account → Developer access creates an integration key shown only once. Keep it on the integrator's server. Only a verifier is stored. Revocation terminates that key's active streams. Browser logout ends browser sessions without revoking unrelated integration keys.

Requests carrying browser cookies require the allowed Origin and `X-Syntelle-Request: 1` for mutations. Server integrations use `Authorization: Bearer <Syntelle integration key>`. All routes remain loopback-only; unrelated browser origins are rejected.

| Interface | Behavior |
| --- | --- |
| `GET /api/v1/auth/status`, `/auth/me` | Provider readiness and current principal. |
| `POST /api/v1/auth/email/start`, `/email/verify` | Request and consume a code. |
| `GET /api/v1/auth/{google,github}/{start,callback}` | Social authorization flow. |
| `/api/v1/configurations` | Account-owned setup listing/creation; ID routes support update/deletion. |
| `/api/v1/keys` | Account-only listing/creation; ID DELETE revokes. |
| `POST /api/v1/streams` | Create a Syntelle stream with validated `settings`; returns ID and a one-use, 60-second connection ticket. |
| `GET /api/v1/streams/{id}` | Authorized stream state. |
| `POST /api/v1/streams/{id}/commands` | Submit an idempotent `message`, `speak`, `interrupt` or `configure` command. |
| `POST /api/v1/streams/{id}/ticket` | Issue fresh scoped connection credentials without replaying commands. |
| `GET /api/v1/streams/{id}/ws` | Player signaling/status/microphone connection; authenticate in the first message. |
| `DELETE /api/v1/streams/{id}` | Stop provider/media work and release the session. |

Commands have `{id,type,text?,settings?}`. `message` asks Gemini to reply; `speak` bypasses Gemini and preserves supplied text through existing speech segmentation. Duplicate IDs with the same content never repeat provider work; changed content under an existing ID is rejected. The stream remains connected across idle and speech. Character selection is restricted to Syntelle.

Player signaling uses `{type,session_id,command_id?,data?}`. First send `authenticate` with `data:{ticket}`. Exchange `signal` messages containing RTC `description` or `candidate`; the private worker produces the offer. Heartbeat every 15 seconds. The WebSocket relays control/signaling, not video. Public conversation events exclude prepared audio and private conversation identifiers.

## Persistence and verification

`syntelle_accounts` contains versioned account/identity/session/configuration/key/auth-delivery records. `syntelle_streaming` contains versioned stream, command and measurement records. Existing `syntelle_conversation`, `syntelle_speech` and `syntelle_preparation` records remain separate. Restart marks unfinished stream work explicitly rather than retrying paid operations.

Verification uses fixture OAuth/mail/model/speech providers and existing authorized cached recordings. Disposable test databases must have the explicit test prefixes checked by the integration helpers. No raw microphone audio is retained. Complete private render exports and immutable implementation/evidence files are archived and restored through PostgreSQL; live secrets and authentication codes are excluded.

Evidence belongs in `.tools/accounts-streaming-49/`. Preserve earlier failed checks alongside final results; passing fixture media markers does not establish phonetic or real-device acceptance. See the final verification record in that directory for measured results and remaining setup gates.

### Measured local candidate

The full fixture route passed through the real Go service, PostgreSQL, native Chromium worker and Vue player: email-code sign-in with captured delivery, explicit configuration save, an integration key starting a stream, revocation, exact-text speech, chat, interruption and sign-out. Two real video sessions ran together; a third was rejected and explicit stop released both. Native Go tests and race checks, frontend lint/format/type checks and both public builds passed. Disposable account and streaming database restorations matched, including all 19 tables in the full fixture database. Automated checks made zero paid provider calls.

At 720 × 960, short receiver samples measured approximately 28.5–29 fps, below the strict 30 fps target. First-frame startup was 0.81–0.94 seconds through Go, and 1.59 seconds in the isolated worker check. Cached decoded speech began 115 ms after its playback command in the isolated check. A tone/video transport marker measured video 31 ms ahead of audio; this is not phonetic validation. Worker interrupt control took 6.1 ms, while the UI suppresses obsolete audio immediately; receiver audible-stop latency and live provider first-response latency remain unmeasured.

Short Windows working-set samples for the private Node/Chromium process tree were 351 MiB without streams, 745 MiB with one and 1,154 MiB with two; receiver processes were excluded. These are snapshots, not peak or dedicated GPU memory. Chromium exercised real media; WebKit covered mobile layout. Physical-device, real microphone/echo and live OAuth acceptance remain owner review items. The later sender setup uses `no-reply@syntelle.online`; one explicitly authorized plain-text test was accepted by Resend. Social client configuration remains required.

`archive-evidence.py` in the evidence directory freezes source, package inputs, both builds and bounded verification records through the generic preparation archive. It checks configured secrets before persistence, restores the complete snapshot to a separate directory and compares every file hash. Its report is written outside the archived tree at `.tools/accounts-streaming-49-archive.json`. Fixture database dumps are evidence for isolated restoration only; never restore them into the owner's live account or provider schemas.

### Connection recovery and email presentation follow-up

The phase-50 client fixes the stopped-video state reported after tab changes and service restarts. Unexpected signaling/media loss retries transport at 0.5, 1.5 and 3 seconds. A surviving stream gets a fresh scoped ticket and retains its server context; a missing stream starts fresh idle video. Recovery sends no model or speech submission and never restarts the microphone. Obsolete output is suppressed and interrupted. Hiding the tab still releases capture/media; returning restores idle video while keeping the draft. Rejected email verification or failed sign-out restores idle video without clearing the account error. Unavailable controls now state that connection is needed instead of promising readiness shortly. Superseded video-play promises cannot leave a false autoplay warning.

`scripts/review-stream-recovery.mjs` covers same-stream reattachment, hidden/visible suspension, missing-session recovery, preserved drafts, rejected sign-in recovery, no microphone reactivation and no message/speech replay. Chromium uses the actual Go/worker video; WebKit covers the unavailable mobile state with bounded retries. Evidence is `.tools/stream-recovery-50/`; this is not a new performance benchmark or real microphone/device acceptance.

Sign-in email now sends self-contained HTML plus the original text alternative. `internal/accounts/templates/sign-in.html` follows the graphite/violet palette, with an emphasized one-time code, expiry, safety guidance and system fonts. Tables and inline styles avoid a new rendering dependency. Dynamic content is escaped by Go's `html/template`. No remote images/fonts, links, scripts or template tracking resources are included; actual codes and rendered login bodies remain memory-only. Preview evidence in `.tools/email-template-51/` uses six placeholder bullets, never a real challenge. It is browser layout evidence, not a claim of identical rendering in every mail client. No additional email was sent during this change.

Resend domain-level open/click tracking must remain disabled, separately from template content; see [Resend's domain settings](https://resend.com/docs/api-reference/domains/update-domain). The configured key returned 401 for a read-only domain lookup, so this run could not verify or alter that provider setting. No provider configuration was silently changed. The earlier owner-authorized plain-text test remains a separate delivery record.

### Stream workspace presentation

Phase 52 removes the duplicate in-workspace Syntelle label and extra toolbar row. The application brand stays in the original global header; the conversation header owns quiet new-conversation, customization and account actions. New conversation uses an accessible icon with a 44 px target, and resets the conversation mode while preserving an unsent draft.

`components/StreamComposer.vue` owns the controlled text field, autoresizing and integrated arrow action. Enter sends, Shift+Enter adds a line, and composition events do not submit. Empty/offline submissions remain disabled. The Chat/Read aloud tabs are removed. Literal speech remains under Customize → Say exactly what I type, with an explicit indication when active and a return-to-conversation action. The backend still receives the same `message` and exact `speak` commands; no public interface, package, renderer or provider policy changes.

`.tools/stream-ui-52/` preserves the preceding sources, screenshots, current sources and focused browser evidence. `scripts/review-stream-ui.mjs` uses real idle video with intercepted fixture text commands, plus a WebKit mobile layout fixture. It covers keyboard submission, multiline drafts, exact-text routing, new-chat reset, disabled sending, icon targets and both themes. Frontend lint/format/type checks and the consumer build pass. No paid model/speech call was made; this presentation change does not resolve the Azure speech-preparation failure in the owner's screenshot.

### Workspace spacing follow-up

Phase 56 gives both public workspaces a centered 1,520 px shell, 24–40 px responsive side gutters, 32 px vertical padding and a 40/60 portrait/conversation split with a 32 px gap. The header aligns with the content. Long replies use 28 px transcript margins and a 72-character width preference. The remote video keeps its original aspect ratio; the local character's desktop presentation frame is reduced to `min(88cqh, 135cqw, 800px)`. Mobile keeps 16 px side gutters and uses a shorter portrait row without shrinking its relative character crop twice.

`.tools/workspace-spacing-56/` retains previous sources and layout, current sources, screenshots and a bounded layout review. Chromium inspected actual idle video at 1,920 × 960 and 1,440 × 900, plus the existing local workspace. WebKit inspected the 390 × 844 unavailable-stream layout with a fixture, not real media or a physical phone. The measured 1,920 px stream video width fell from 666 px to 563 px, about 15%, with no document overflow in the inspected layouts. Text replies were browser fixtures; no model, speech, authentication email or image request was sent. Existing animation, media resolution, provider behavior and technical trial dimensions are unchanged. Scoped frontend lint/format/type checks and the consumer build pass. The evidence archive restores through PostgreSQL with file-hash verification.
