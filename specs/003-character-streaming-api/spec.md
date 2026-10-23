# Syntelle character streaming API

Status: local implementation candidate, October 21, 2026. The original product scope is retained below; the bounded Syntelle-only implementation is documented in [Local accounts and rendered streaming](../../docs/development/accounts-and-streaming.md).

## Current local implementation

- Select `/?view=stream` for the local WebRTC player and private Chromium RGB-D render worker. The previous default remains at `/`; Café/Woodland are legacy resources and are not selectable in the new streaming service.
- The website and integration API share session-owned idle, message, literal-speech and interruption behavior. Go enforces account/key ownership and idempotent commands; private materials are excluded from the player build.
- Initial output is opaque 720 × 960, with a 30 fps target and two concurrent sessions as a configurable capacity safeguard. Measured results and remaining acceptance gates belong in `.tools/accounts-streaming-49/`; targets are not guarantees.
- Accounts save Syntelle configurations and issue integration keys. Google/GitHub and Resend login remain gated by configured OAuth credentials and an explicit verified `RESEND_FROM`; fixture verification does not establish live identity/email delivery.
- No new portrait preparation, image generation, public deployment, Internet TURN configuration or cloud provisioning is included. Use the linked development record for current setup, API contracts and verification evidence.

## Confirmed decisions

- Deployment target: Azure Container Apps Express with CPU-only compute. Express excludes GPU workloads. Local NVIDIA-assisted measurements do not establish deployment concurrency or economics. The private renderer/media path still needs CPU-only Linux and public connectivity validation; no separate GPU service or client-side rendering change is implicitly authorized.
- Visitors can talk to Syntelle, explore and start creating without signing in.
- Accounts add saved characters/settings across devices and integration API keys.
- API consumers can select a character, send a conversational message, provide the exact text to speak, and receive idle animation.
- The output is rendered live video with synchronized audio. The consumer plays the result rather than receiving the rendering engine or animation data.
- Conversation-history synchronization is not part of the selected first account scope.

This deliberately extends the original public-demo MVP's account/embedding exclusions. The preceding [public demo](../001-syntelle-demo/spec.md) and [optional login](../002-optional-login/spec.md) remain separate records. The local candidate is implemented; public-service readiness must not be inferred from it.

## Core experience

Create one stream session for an authorized, prepared character. The session starts in idle and remains connected between utterances. A message or supplied-text command changes that same session to speech; completion returns it to idle. The consumer should not have to request or stitch a new video for each utterance.

| Capability | Expected behavior |
| --- | --- |
| Select character | Choose a ready character the account owns or is explicitly allowed to use. |
| Start session | Obtain session identity and scoped connection information, then receive idle video. |
| Send message | Ask Syntelle to generate a reply, then speak and animate it through the existing conversation/voice pipeline. |
| Speak text | Speak the supplied text without asking the conversation model to rewrite or answer it. Preserve the complete text rather than silently shortening it. |
| Idle | Keep the selected character's current blinking, gaze, breathing and head behavior between speech commands. Idle itself makes no model, recognition or synthesis call. |
| Interrupt | Cancel the current response and discard obsolete speech/animation; resume idle without replaying late output. |
| Stop session | End media and provider work and release rendering/encoding resources. |
| Observe status | Receive session, command, preparation, speaking, idle and error events with identifiers. |

Conversation messages and literal speech are distinct commands. Providing text must not unexpectedly select the other behavior.

Character selection at session creation is sufficient for the first version. Seamless character replacement inside an active media track is not assumed.

## Accounts and API keys

- API keys are issued by Syntelle for integrations; they are not Gemini, Azure or user-supplied provider keys.
- Key management requires an authenticated account. Proposed controls: name, create, display the secret once, show a nonsecret identifier/last-use state, and revoke.
- Store only a verifier for the integration secret. Do not put raw keys in logs, diagnostic snapshots, media URLs or public browser assets.
- Each protected action checks key validity, scope, character access and session ownership. A key is not an owner/admin credential.
- Long-lived integration keys belong on the integrator's server. Browser/mobile players receive short-lived, session-scoped connection credentials instead.
- Revoking a key prevents new commands and terminates its active stream sessions. Logging out of the website ends browser authentication, not unrelated server integrations.
- Public Syntelle visitors use a separate guest-session policy; the website does not embed a shared developer key.

These access boundaries follow [OWASP REST guidance](https://cheatsheetseries.owasp.org/cheatsheets/REST_Security_Cheat_Sheet.html#api-keys), including per-resource authorization beyond possession of an API key.

Public concurrency, usage and cost policies need explicit server enforcement before public exposure. Existing local-owner uncapped operation and historical provider allowances stay unchanged; ordinary accounts do not inherit that policy.

## Media requirements

- Deliver live rendered video and synchronized speech audio, with stable dimensions throughout a session.
- Preserve the existing character's RGB-D appearance, opaque dentition, mouth articulation, eyelids, gaze and restrained motion.
- Compose the selected background, background-only blur and framing on the server. Capturing the current transparent character canvas alone is insufficient because these currently live in separate CSS/DOM layers.
- Use a shared server-owned media timeline for PCM, native visemes, facial animation and outgoing frame timestamps. Neither command arrival nor network callback arrival drives articulation.
- Keep the stream connected across idle, preparation, speaking and interruption.
- Drop obsolete frames under congestion rather than accumulating a progressively delayed animation.
- Make connection failure/recovery and capacity rejection explicit. A media reconnect does not repeat paid message/speech commands.

Proposed first-delivery default: an opaque scene using an existing background. Transparent-overlay video, downloadable rendered videos and OBS/meeting-specific outputs are separate extensions unless the owner selects them.

WebRTC is the initial transport candidate for interactive audio/video, based on its [real-time media specification](https://www.w3.org/TR/webrtc/). Transport, codec, relay and hosting choices require a local proof before they become fixed architecture.

## Reuse and new work

Reuse current portraits, foregrounds, depth, eyes, oral packages, animation logic and native speech. Do not regenerate images or replace the accepted RGB-D appearance merely to change delivery.

Rendering moves off the consumer's device, but does not require an immediate native rewrite of WebGL. The first candidate is a private headless-Chromium render host, reusing the existing engine. [Chrome Headless](https://developer.chrome.com/docs/automation-and-testing/headless) supports running the browser without a visible interface; its suitability, GPU access and capacity for this workload are not yet verified.

New work includes session-owned clocks, complete-scene composition, frame capture, encoding, audio-track publication, media transport, isolation, capacity management and account authorization.

The public website still captures microphone input locally and uses the recognition pipeline. Remote-video playback needs its own playback-state/clock adapter so barge-in, cancellation and echo handling do not depend on the old local AudioBuffer player. Sending textual messages through the integration API does not require adding a general-purpose transcription API.

The website remains Vue on Cloudflare and becomes a client/player for the same session API. Go remains responsible for account/session authorization, commands, conversation, voice orchestration and PostgreSQL records. FastAPI remains responsible for preparation. A private render/media worker is a proposed addition; its host is not selected yet.

[Azure Container Apps ingress](https://learn.microsoft.com/en-us/azure/container-apps/ingress-overview#protocol-types) documents HTTP/TCP and WebSocket support. That is not proof that the current Go WebSocket endpoint provides WebRTC media connectivity. Validate ICE/relay/egress and the media host separately before choosing deployment.

## Functional acceptance

| ID | Requirement |
| --- | --- |
| STREAM-01 | A valid authorized session receives an idle stream before any speech command. |
| STREAM-02 | A message creates a conversational reply; supplied speech text bypasses the LLM. |
| STREAM-03 | Idle, speech and return-to-idle use the same session and media connection. |
| STREAM-04 | Output includes the selected scene/background and synchronized native speech articulation. |
| STREAM-05 | Interruption discards late work and does not restart it after reconnect. |
| STREAM-06 | Commands carry session/command IDs and deduplicate repeated submissions. |
| STREAM-07 | A different account/key cannot read or control an unauthorized character/session. |
| STREAM-08 | Revocation, stop, disconnect expiry and service shutdown release active work. |
| STREAM-09 | Account character/settings restoration works independently of conversation-history sync. |
| STREAM-10 | Browser/mobile playback does not need the private rendering engine, meshes or source materials. |
| STREAM-11 | Idle uses no paid AI calls, and recording/usage distinguishes stream time from model/speech consumption. |
| STREAM-12 | Existing local demo/resources remain available until the streaming candidate passes review. |

Idle still consumes rendering, encoding and bandwidth. Measure active and idle cost; do not describe a connected idle stream as zero-cost or scale-to-zero.

## Original proof and delivery order

1. Prove one local stream using Syntelle and one cached utterance: idle, speech, interruption, idle, stop. Use a private render host, a complete opaque scene and a separate player. No paid calls or new character preparation are needed.
2. Measure startup, first audible response, audio/video offset and drift, frame cadence, memory, encoding overhead and resource release. Compare against the current client renderer. A fixed 30 fps output is a proposed prototype target, not a capacity guarantee.
3. Add the authorized command/session interface and minimal account/key ownership model.
4. Add saved-character/settings restoration and the account/key management UI behind optional login.
5. The owner narrowed the implemented delivery to Syntelle only, one concurrency check and bounded browser/mobile checks. Keep Café/Woodland in legacy trials. Changing the default remains subject to owner review.

Before public release, define guest/account quotas, key scopes, stream limits, session expiry, retention, relay/codec support, deployment capacity and actual cost. Do not deploy an uncapped owner endpoint publicly.

## Boundaries

No billing/subscriptions, organizations, role editor, general-purpose SDK, arbitrary character-generation API, image regeneration, new animation model, wide-angle reconstruction, production deployment or automatic history synchronization is included in this draft.

Saving characters requires a durable private rendering package and account ownership. The existing development archive is not an implemented production account library. Define retention for temporary uploads versus saved likeness/render materials explicitly.

The streaming prototype does not complete the outstanding multi-character public-upload validation, physical-device acceptance or prior visual/performance gates.


## Browser backgrounds (October 22, 2026)

The owner authorized moving background composition and blur to the frontend while
keeping all Syntelle character processing in the backend. The current live worker
sends a 1440 x 960 H.264 frame with premultiplied character RGB and full-size alpha
side by side; the public receiver produces the 720 x 960 presentation and PiP video.
This supersedes the preceding complete-opaque-scene contract. Character source
materials, geometry and animation code remain private. The original bitrate cap,
media clock, audio tracks, provider policy and prepared assets remain unchanged.
See [browser background composition](../../docs/development/frontend-background.md) for implementation, rejected checks,
bounded validation and remaining performance/device limits.
