# Local owner conversations

## Current Gemini setup

Chat uses **Gemini 3.8 Flash** (`gemini-3.8-flash`) through Google's Interactions API. Azure Speech still handles continuous recognition and Dragon HD audio/native visemes. No Foundry deployment is required for the current choice.

Keep these values privately in the root `.env`:

```dotenv
SYNTELLE_LOCAL_OWNER=1
SYNTELLE_CHAT_PROVIDER=gemini
GEMINI_CHAT_MODEL=gemini-3.8-flash
GEMINI_API_KEY=YOUR_PRIVATE_KEY
AZURE_SPEECH_KEY=YOUR_PERSONAL_SPEECH_KEY
AZURE_SPEECH_REGION=canadacentral
```

The existing Gemini key is reused server-side. Never put keys in chat, browser variables or archived evidence. Rebuild/restart the API after changing configuration and refresh the workspace. Azure and Gemini use their respective provider billing; Gemini does not require access to the failed Foundry deployment.

Gemini receives text history and current instructions, with `stream:true`, `store:false`, `thinking_level:low`, `thinking_summaries:none` and a 1,024-token per-response safety bound. `low` is the model's lowest supported thinking level; `minimal` is unsupported. Only text from `model_output` steps is presented/spoken. A terminal `interaction.completed` with completed status is required. Incomplete streams, model-identity changes and provider errors stop the turn without fallback or paid retries.

The local owner policy has no accumulated application allowance for conversation, recognition or Read aloud. Provider charges and quotas still apply. Historical speech budgets and recordings remain unchanged. Bind Docker ports to loopback only; this mode must not be deployed publicly.

Sources: [Gemini 3.8 Flash](https://ai.google.dev/gemini-api/docs/models/gemini-3.8-flash), [Interactions API](https://ai.google.dev/api/interactions-api), [stateless storage](https://ai.google.dev/gemini-api/docs/interactions-overview).

## Feature boundaries

Go owns orchestration, provider requests, native Speech SDK and PostgreSQL. Vue separates transcript state, socket lifecycle, microphone capture, AudioWorklet resampling and existing prepared-speech playback. ChatPanel owns conversation controls; ConversationMessages owns the transcript; Customize owns editable instructions. The workspace selects Chat or Read aloud and supplies the current character's playback context.

`services/api/internal/conversation` contains explicit Gemini/Responses adapters, socket orchestration, recognition guards and PostgreSQL storage. `internal/speech` contains the native providers, speech ledgers and conversation adapters; `internal/httpapi` owns HTTP/access/lifecycle helpers, and `cmd/api` wires the executable. See [Go service organization and reliability](go-service.md). `apps/web/src/features/conversation` contains the Vue components/composables and streaming audio utilities. No character rendering assets or model preparation are changed.

Raw microphone PCM is transient and never archived. Structured transcripts, original model output, bounded spoken text, provider settings/results, usage and TTS audio are retained. Model requests use the latest 20 exchanges and current instructions, `store:false` and streamed Gemini Interactions with low thinking, without automatic paid retries or model fallback.

## Browser transport: WebSocket and SSE

The browser/Go conversation uses one bidirectional WebSocket. The client sends `configure`, `submit`, `cancel`, microphone start/stop, repeated PCM audio messages and playback/activity events. Go returns recognition partials/finals, model text deltas, status/errors and the completed prepared-speech audio/viseme payload. This matches the simultaneous upstream microphone/control and downstream conversation flow in [the socket service](../../services/api/internal/conversation/service.go) and [browser socket lifecycle](../../apps/web/src/features/conversation/composables/useConversationSocket.ts).

SSE is a reasonable alternative for text-only response streaming or preparation-progress events. It carries a server-to-client UTF-8 event stream; native `EventSource` receives that stream and does not send microphone/control messages. An SSE response to a POST can also be consumed with `fetch`, while a persistent EventSource stream would need separate client POSTs. A hands-free SSE design therefore still needs an upstream audio/control path and coordination between its lifetimes. WebSocket keeps those messages on the existing conversation connection. These are transport tradeoffs, not a measured latency or cost advantage. [SSE standard](https://html.spec.whatwg.org/multipage/server-sent-events.html), [WebSocket standard](https://websockets.spec.whatwg.org/).

The Go-to-Gemini connection already consumes SSE over HTTP through [the Gemini adapter](../../services/api/internal/conversation/gemini.go). The two boundaries can use different transports: model text arrives progressively from Gemini, while the browser also uploads live microphone audio and sends interruption events. Currently synthesis begins after the bounded model reply completes, and `prepared_speech` delivers the resulting package; choosing WebSocket alone does not make TTS progressive or guarantee immediate provider cancellation.

Keep the existing transport for the current voice conversation. A future text-only/progress endpoint can consider SSE within its own task. This rationale changes no endpoint, dependency, paid-call policy or deployment decision.

## Local operation and safety bounds

Start the existing PostgreSQL preparation profile, rebuild the API and restart the local API/web containers after private configuration changes. The web proxy supports WebSockets. `/api/v1/conversation/status` reports missing setup without exposing credentials. `/api/v1/conversation/ws` accepts only the two local browser origins (`localhost:5173`, `127.0.0.1:5173`), and the owner HTTP wrapper rejects unrelated origins and Host headers. Compose ports remain bound to loopback.

The initial transcript restoration loads the latest 100 turns; every earlier turn remains in PostgreSQL. New chat creates a new session, never deletes old history. Each character and selected language has its own persisted session reference in local storage. Instructions are editable preferences and are committed on the next request. The database, not local storage, owns message content.

Per-message protections are 500 UTF-16 characters for spoken text, 500 input characters in the UI, 2,000 instruction characters, 1,024 Gemini output tokens, a 75-second turn timeout, 20-second synthesis timeout, bounded socket/PCM queues, and one active turn per conversation. These do not accumulate into trial allowances. Provider quotas and billing remain in effect. A failed or uncertain operation is not retried automatically. The five-second shutdown includes upgraded sockets and native provider cancellation; a stalled native operation cannot extend the process deadline.

Microphone input is PCM16 little-endian at 16 kHz. The AudioWorklet preserves its filter history and resampling phase, uses a windowed-sinc low-pass filter, and requests browser echo cancellation/noise suppression. A 120-ms adaptive gate interrupts local audio; only Azure final recognition can submit a new turn. Recognition offsets are compared with playback windows to reject late echo results, even after audio ends. This is not a guarantee of physical-device echo rejection. Hidden tabs stop capture and pending work; returning requires explicitly enabling the microphone.

The pinned native/Go Speech SDK remains 1.52.0. WebSockets use `github.com/coder/websocket v1.8.15`. The current Gemini adapter uses Go's HTTP client and the pinned model ID, with stateless streaming. The historical Responses adapter remains available only through an explicit `SYNTELLE_CHAT_PROVIDER=foundry` selection; it is never an automatic fallback. There is no fallback model, region change or phoneme extraction.

## Persistence and verification

The embedded version-1 migration creates `syntelle_conversation.sessions`, `turns`, `operations` and `recognition_results`, with ordered relational identifiers, JSONB metadata and TTS BYTEA. Provider input is committed before calling Azure. Completion, failure, cancellation and uncertain restart outcomes are recorded. Recognition finals are persisted incrementally without retaining an ever-growing in-memory audio/transcript buffer. Old speech allowance rows and requests are unchanged; only the two genuinely authorized historical speech ledgers can supply cache hits, excluding the halted synthetic incident ledger.

Focused PostgreSQL tests create a distinct temporary database and verify its identity before applying migrations or inserting fixtures. They cover history/JSONB restoration, BYTEA cache recovery, idempotency, owner-mode usage beyond the old caps, cancellation, native-result deduplication and socket shutdown. Browser fixtures cover streamed replies, prepared speech without another synthesis, character switching, Read aloud, resampler chunk invariance/alias suppression, local interruption and hidden-tab capture shutdown.

Two actual native recognition requests were verified using existing Ava English/Spanish TTS recordings, with no new synthesis or model calls. Final transcripts arrived at 5,193 ms and 5,470 ms respectively from stream start, including real-time feeding of roughly 4.25/4.35-second source recordings. This is not complete microphone-to-reply latency. Their normalized SDK evidence is retained in `.tools/conversation-29/` and PostgreSQL. Later SDK calls additionally retain the provider JSON result.

The earlier Luna plan was not live-verified because Foundry account/deployment access could not be completed. Gemini now replaces that plan. Physical microphone, speaker/headphone echo behavior and real-device acceptance remain owner checks. Windows WebKit lacks Web Audio in this environment: typed replies remain available and unsupported playback has an explicit message; its native microphone test is skipped, not passed. Current renderer performance and physical-device limitations remain as previously documented.

## Sign-in troubleshooting

An account-not-in-directory message on the Foundry account picker means the selected identity does not belong to the directory being opened. Select the personal account associated with the personal Azure subscription, not the university/work identity. In Azure portal, use the account's directory/subscription switcher to select the directory containing the personal Speech resource, then open Foundry. A tenant policy blocking device-code login is not evidence of missing model quota and must not be bypassed. Use the normal browser sign-in or contact the relevant administrator if that flow is also blocked. [Microsoft account/directory mismatch guidance](https://learn.microsoft.com/en-us/troubleshoot/entra/entra-id/app-integration/error-code-aadsts50020-user-account-identity-provider-does-not-exist).


## Historical Foundry option

Use the owner's personal Azure subscription, never the work subscription. In Azure AI Foundry, create or select an Azure OpenAI resource and deploy `gpt-6-luna` as **Global Standard**, preferably in **Canada Central**. Check the subscription's model quota first. If quota or the model is unavailable, stop and request quota; do not substitute another model or region. Global Standard does not guarantee Canadian processing residency.

Copy the resource's endpoint and API key from **Keys and Endpoint**, and the exact deployment name from **Deployments**. Save these privately in the repository-root `.env`:

```dotenv
SYNTELLE_LOCAL_OWNER=1
AZURE_OPENAI_ENDPOINT=https://YOUR_RESOURCE.openai.azure.com
AZURE_OPENAI_DEPLOYMENT=YOUR_LUNA_DEPLOYMENT_NAME
AZURE_OPENAI_API_KEY=YOUR_PRIVATE_KEY
```

Keep the existing personal `AZURE_SPEECH_KEY` and `AZURE_SPEECH_REGION=canadacentral`. Do not paste credentials into chat, use `VITE_*` credentials, or commit `.env`. Restart the API after changing private settings. Resource creation and quota requests are owner-operated.

The local owner policy has no application usage allowance for conversation, recognition or Read aloud. Azure charges and quotas still apply. Historical speech budgets and recordings remain unchanged. Bind Docker ports to loopback only; this mode must not be deployed publicly.

Sources: [model regions](https://learn.microsoft.com/en-us/azure/foundry/foundry-models/concepts/models-sold-directly-by-azure-region-availability), [Responses API](https://learn.microsoft.com/en-us/azure/foundry/openai/how-to/responses).



## Gemini live verification (October 20, 2026)

The owner explicitly replaced the undeployed Luna model with Gemini 3.8 Flash. Three live browser turns passed through Go, Gemini Interactions, PostgreSQL, Dragon HD audio/native visemes and the existing character playback layer: two English turns with Syntelle (including recall of the number 23 from the preceding exchange), then a Spanish reply with Café/Andrew. Gemini's first text deltas arrived at 3,552 / 1,804 / 1,527 ms; complete model responses at 3,639 / 1,812 / 1,539 ms. These are server-observed model timings, not physical microphone-to-speaker latency. Three additional TTS requests completed and are saved as BYTEA; no retries or fallback were used.

Provider operations use kind `gemini` and preserve the exact stateless request body, model identity, complete SSE events, usage and adapter version. Thought steps are excluded from user-facing deltas and speech. Historical Luna/recognition/speech records and image/analysis allowances remain intact. The failed Foundry plan and the phase-29 code/evidence remain available in snapshot 434 and `.tools/conversation-29/`.

Verification is bounded: focused provider/PostgreSQL tests, the pinned native Go container build, frontend checks and Chromium/WebKit conversation fixtures. Physical hands-free microphone/echo and real-device acceptance remain owner checks. Phase-30 evidence is `.tools/gemini-chat-30/`, archived and restored through the generic PostgreSQL preparation archive.

## Go reliability follow-up (October 20, 2026)

The current command/internal package organization and bounded checks are documented in [Go service organization and reliability](go-service.md). Full output queues wait for space with producer cancellation and a five-second congestion bound. Existing Host/Origin protection now covers owner and historical trial speech endpoints, with shutdown composition retained. Interruption-write failures emit a nonfatal `persistence_warning` after `cancelled`; the client displays it without affecting a newer turn or overriding an existing error. Server diagnostics contain only a stable code, IDs and sanitized failure classification.

The native/race Go suite, stub build, frontend checks and two intercepted Chromium cases pass offline. The live recognition and running-service proxy checks remain explicitly skipped. Existing SQL/migration bytes, provider records, budgets, caches and prepared assets are unchanged. This delivery neither retries a provider nor deploys/rebuilds the running owner services. Evidence is `.tools/go-reliability-33/`; historical finalizers must not be rerun against relocated sources.
