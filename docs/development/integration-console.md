# Integration console and website player

## Product structure

The console prioritizes creating and publishing characters for external use. Overview, Characters, Integrations, Usage and Billing are the main destinations. Team and Settings belong to the selected workspace. Personal conversation testing and its history are secondary to the selected character; there is no global recent-chat list in the console sidebar.

Signed-in initial entry opens `/console`. Existing explicit conversation links remain valid. `/characters` keeps personal creation and collection access; `/console/integrations`, `/console/usage`, `/console/billing`, `/console/team` and `/console/settings` share the same shell. Staff administration remains separately verified. Workspace changes invalidate pending reads and close sensitive reveal states. The desktop sidebar starts at 264 px, can be resized between 240 and 320 px or collapsed, and becomes a modal drawer below 1024 px.

The retained Test view keeps its existing navigation suspension: leaving stops capture and playback, persists a paused session and requires an explicit Continue. Opening navigation or account settings alone does not create a rendering session.

## Ownership and publishing

A personal ready character can be explicitly copied into an immutable workspace publication. The publication stores its package binding, approved configuration and portrait. Deleting the personal source does not delete that publication. Package garbage collection counts published references. Revoking a publication blocks future integration sessions and invalidates active access.

An integration has separately saved draft and published configuration. Publishing increments its revision. It selects an immutable publication, exact allowed website origins, language, appearance and a daily usage ceiling. A key is issued for one published integration. Existing personal keys keep their preceding meaning; old unscoped workspace keys are not silently activated or migrated.

External visitors do not become the key owner's account. Their requests cannot load personal history, saved setups, workspace administration or private instructions. Public session responses use a restricted player configuration. The server command allowlist protects the publication from visitor changes. Private identity and delivery instructions still reach the native worker, never the widget configuration.

## Website installation

After publishing a website integration, copy its snippet from Integrations:

```html
<script src="https://YOUR_SYNTELLE_HOST/widget/v1.js"
        data-widget="PUBLISHED_WIDGET_ID"
        data-mode="inline"
        async></script>
```

Use `data-mode="floating"` for the launcher. Inline inserts the player immediately after the script. The snippet contains a public widget ID, never a secret API key. Visitors require no customer backend and no Syntelle account. Opening the launcher is free of rendering, microphone and synthesis work; **Start** inside the player creates the session. Closing or removing the widget stops presentation and releases its session. Reopening requires another explicit Start.

The iframe `/embed/{widget_id}` loads only the shared native-stream player, not the console. Its public metadata contains display name, portrait, appearance, allowed origins and availability. The production edge and local Vite middleware derive exact `frame-ancestors` from that publication. Console pages cannot be embedded. Do not serve the SPA directly through a host that removes these headers.

The handshake checks the exact parent origin, window source and per-frame nonce. The public handshake contains no credentials or transcript. A Start obtains a short-lived single-use bootstrap, completes server-validated Turnstile with action `widget_start` and that bootstrap's `cData`, then requests a session. Turnstile must be configured for the **Syntelle iframe host**, where its challenge runs. The capability stays in iframe memory and authorizes only its session read/delete/ticket operations. Existing one-use WebSocket tickets are preserved. Browser cookies and personal API requests are omitted.

Configure `SYNTELLE_TURNSTILE_SITE_KEY`, `SYNTELLE_TURNSTILE_SECRET` and `SYNTELLE_TURNSTILE_HOSTNAME` locally. Public access remains unavailable when challenge or paid-use admission is unconfigured. Origin checks reduce browser misuse; they are not a secret credential or protection from arbitrary non-browser clients. Server challenge checks, rate limits, daily ceilings, paid credit and concurrency remain authoritative.

## Usage and payment boundary

Billing is sandbox-only. See [Paddle sandbox setup](paddle-sandbox.md) for credentials, catalog and verification. Only the workspace owner or billing manager can change payments; administrator status alone grants no financial authority. Checkout completion in JavaScript never enables service. Signed, durably stored server events and reconciled provider state establish entitlements.

Before starting an integration renderer the server atomically reserves paid seconds and a concurrency slot. Monthly seconds are used before top-ups. Active session time, including quiet time, is metered from the server clock; closing releases unused reservation. Short leases recover abandoned sessions. The local owner's demo exemption never bypasses integration usage. No overage billing or automatic recharge is implemented.

## Validation and limits

Local fixtures cover two-origin installation, explicit Start, scoped capabilities, microphone/media stop, close during verification, quota refusal, draft/publish boundaries, tenant permissions, key revocation and publication retention. Chromium checks include generated local media tracks. Windows WebKit checks cover navigation/control behavior, not actual Web Audio decoding. These fixtures do not qualify a public deployment, cross-network TURN, real-device media or a live Paddle checkout.

No image, analysis, synthesis or conversation-model calls are part of this implementation. The native renderer, Emma, prepared packages, articulation and approved laughter remain unchanged. The owner has no Paddle Sandbox account yet: actual checkout/payment and real Turnstile qualification remain pending configuration.

## Local review and rollback

Evidence and installation receipts are in `.tools/integration-console-126/`. The preserved baseline is phase125 web, phase124 API, unchanged phase123 native renderer and PostgreSQL archive snapshot 846. A local pre-migration account/stream backup is kept separately from experiment archives so private customer records are not copied into review evidence.

The installed candidate is available at `http://localhost:5173/console`; guest access requires sign-in. Web/API routes and the unchanged native worker were checked after installation, with speech and recognition configured. The review startup did not start the character preparation consumer or submit queued generation work. Docker images are `syntelle-web:integration-126` and `syntelle-api:integration-126`. Because registry DNS was unavailable, the API was built from the pinned cached Go sources and cached Speech SDK; the binary was checked against the preserved Debian runtime, and all eight Speech SDK shared-library hashes match. Build provenance is retained in the backend receipt.

```powershell
& .tools/integration-console-126/rollback.ps1
```

Rollback selects the preserved web/API images and reconnects the existing native worker. It does not delete new database records, characters or volumes. Integration access is unavailable under the older API. The separate database backup is for explicit disaster recovery, not automatically restored over newer user data.

Archive only reviewed sources, fixtures, logs and screenshots with `python -m app.audits.finalize_integration_console`. The finalizer verifies byte-exact restoration and never calls a provider. Do not rerun historical navigation finalizers against this source.
