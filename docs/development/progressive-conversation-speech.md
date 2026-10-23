# Progressive conversation speech

The whole-model completion gate below is superseded by [streamed text and early voice](streaming-voice-replies.md), at the owner's request. Its stable opening can now become audible before full model completion after its own durable delivery record. Existing request keys, cached native recordings, remaining-part preparation and immutable evidence are preserved.

The owner reported a long silence after the reply text appeared. The preceding path completed the model, prepared every speech part, assembled the complete recording and only then started browser playback. Long replies accumulated that work serially.

## Current path

1. The Gemini adapter identifies a stable opening sentence or word boundary inside the decoded top-level text string. It can prepare that phrase while the remaining text and optional direction are generated. Incomplete JSON escapes, surrogate pairs, tool output, thoughts and markup are excluded.
2. The opening request uses the same immutable key and normalized text as part zero of the final preparation. Its completed native recording is reused, not synthesized twice. If the request fails, the pipeline stops without a retry. An incomplete or changed final model reply cannot release speculative audio.
3. After full model lifecycle/schema validation and PostgreSQL persistence, the server publishes each available speech part. The first three parts have a 220 UTF-16-unit preparation target, with a short natural opening; later parts have a 900-unit target. These are segment sizes, never reply-length restrictions.
4. At most two native synthesis requests run concurrently. Results are reordered before delivery; callbacks, native operations and child workers are cancelled and joined on failure, interruption or disconnect.
5. The browser owns one turn-specific phrase queue. It plays each recording once, using that recording's native audio/viseme clock. Prepared speech makes no browser synthesis request. Successive parts omit the expression preview delay. Stop, hidden-page suspension, new chat and stale turn IDs discard queued audio.

The player keeps its existing AudioContext and character renderer. This is phrase delivery, not raw PCM or token-to-audio streaming. Native provider/network latency can still delay the first phrase, and unpredictable provider stalls can exhaust prepared audio. No claim of instant response or gap-free playback under every network condition is made.

The existing whole-recording prepared-speech event remains compatible with cached fixtures and older adapters. Historical Read aloud behavior is unchanged.

## Expressions and evidence

Directions are filtered to their actual phrase anchors. Nonfinal parts omit completion expressions, and head cues retain the whole reply's limits and a shared 2.4-second minimum spacing. Native WAVs and visemes remain immutable. The existing derived full recording and timing are retained for restoration and inspection; the browser does not wait for that recording before starting.

Each native operation is reserved before calling Azure, then completed with original audio in PostgreSQL. Additive speech-delivery records preserve phrase ordering, hashes, provider identity, direction and readiness time without storing a second base64 copy of the native recording. Model diagnostics include opening-prefetch and first-visible-text timing. Existing schemas, local-owner policy, credentials, image budgets and historical speech allowances are unchanged.

Speculative preparation may incur a native speech charge even if the later model envelope fails; such results are retained and never played or retried automatically.

## Bounded checks and live result

Focused tests cover two-worker preparation, order, cancellation, provider failure, retained full text and native PCM, early JSON decoding, incomplete-model rejection, isolated PostgreSQL persistence, browser queue drain/deduplication, interruption and hidden-page capture shutdown. Scoped frontend lint, formatting, type checking and production build pass. Native container checks reuse Speech SDK 1.52.0.

Four focused Chromium desktop cases and one WebKit mobile failure-state case pass. Windows WebKit has no Web Audio; it is not playback/device acceptance. Physical microphone/speaker echo and mobile-device behavior remain owner checks.

Three live Gemini replies and nine new native Dragon HD recordings were used while identifying and correcting startup starvation. The serial draft and two-short-part draft remain preserved. No retries, fallback, recognition request, image generation or analysis was used.

In the final live English sample:

| Event | Time after submitting the test turn |
| --- | ---: |
| Opening preparation requested | 3.184 s |
| Reply text visible | 4.119 s |
| Model reply complete | 4.313 s |
| First audio part available | 5.974 s |
| All audio prepared | 16.500 s |

The first audio was available **1.855 s after visible text**, 10.526 s before all audio was ready. Every later part arrived before its calculated playback deadline. These are one live server/WebSocket observation and a buffer calculation, not statistically matched provider benchmarks or physical microphone-to-speaker measurements.

Evidence and source copies are in .tools/speech-latency-38/, with final live records in final/ and preceding drafts in serial-draft/ and parallel-draft/. The probes deliberately did not play audio, so their disconnects correctly persist as interrupted turns; native/model results are complete and durable. Archive and restore this directory through the generic PostgreSQL preparation command, never a historical character finalizer.
