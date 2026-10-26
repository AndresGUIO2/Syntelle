# Streamed text, early voice and confirmed interruptions

The owner reported that a long story stopped speaking and that the complete text appeared before voice began. This correction supersedes the whole-model playback gate in [phase 38](progressive-conversation-speech.md). Character resources, attention, geometry, native visemes, provider identities and historical budgets remain unchanged.

## Findings

The directed Gemini adapter consumed SSE but emitted the entire decoded `text` field only after its closing quote. An opening could be prepared earlier, but its recording could not be delivered until the model completed. This made text appear as a single block and held an already prepared opening behind the remaining generation.

The owner's stored story has 2,203 original text characters, 2,197 normalized speech characters and three delivered audio parts. The final stored interruption reason is `disconnect`. That alone cannot establish the initial cause: the old interruption write overwrote an earlier cause on later disconnect. The incident's complete requests/results, five existing recordings (including the two cancelled records), status and timestamps are preserved in `.tools/streaming-reply-41/incident/`. Cancelled recordings are diagnostic evidence, not promoted cache entries or renewed request authorization.

Separately, the microphone path cancelled the reply immediately on 120 ms of sufficient acoustic energy, without requiring recognized speech. Noise or speaker leakage could therefore stop speech without creating a new user message. A browser fixture also reproduced a deferred playback-stop watcher clearing the identity of a replacement turn; that timing race is fixed synchronously.

## Current behavior

- The directed adapter emits append-only decoded text prefixes while the top-level text string grows. Incomplete escapes, surrogate pairs, JSON control fields and provider thoughts are not emitted. Final schema/model completion validation still determines whether the full reply succeeded.
- The existing stable opening phrase is prepared once using the existing native request key. Its exact text, native-audio hash and model-operation binding are committed in a `progressive-speech/3` delivery record before playback, even while the model is still running. The record explicitly marks `model_pending`.
- The remaining audio is prepared from completed model text with the existing two-worker pipeline. Its part zero reuses the same native recording and is not delivered twice. Opening articulation has no not-yet-known semantic direction; subsequent parts retain applicable validated direction. No renderer or facial-expression material changes are involved.
- A later model/schema/provider failure stops remaining audio and leaves an explicit failure. The first phrase may already have been heard; it cannot be retracted. Original partial text and all operation evidence remain preserved. No request is automatically retried or replaced by another provider.
- Completion includes the expected audio-part count. A missing part is an error, not successful completion; received parts are still deduplicated and played in order. A connection loss stops capture/playback and visibly marks the reply interrupted, with an instruction to reconnect using the microphone.
- Microphone energy and recognition-start events only arm interruption detection. A recognized non-echo final utterance uses the existing settling/grouping path to replace the reply. The Stop reply button remains immediate. This intentionally trades energy-only interruption speed for fewer false cuts.
- While playback is active, a normalized verbatim match of at least four recognized words against delivered speech is ignored as speaker leakage. Short commands remain eligible. This uses existing PostgreSQL text records and original recognition evidence. It is not a physical acoustic echo canceller and does not establish real-room echo acceptance.
- The first recorded interruption cause survives later cleanup/disconnect. Historical records are not rewritten to infer a cause that was lost.

No schema, dependency, public endpoint, model/voice selection, image generation or allowance change is introduced. The local owner service remains loopback-only. Full speech text is retained; preparation segment sizes are not reply-length limits.

## Bounded verification

Native Go checks use the pinned Linux Speech SDK and race detection, with fixture providers on a separate internal-only Docker network. Integration helpers construct and verify isolated PostgreSQL databases before any mutation. Tests prove durable first audio before model completion, exactly-once opening delivery, model failure/cancellation, raw stream/key protection, two-worker ordering, noise/echo handling, genuine recognized interruptions and preservation of the first cause.

Browser fixtures prove visible partial text and actual Web Audio playback before complete text, eight-part queue drain, missing-part rejection, explicit disconnection, noise without cancellation, replacement-turn playback, Stop reply and hidden-page suspension. Desktop/mobile Chromium checks are bounded; Windows WebKit has no Web Audio and is not represented as audio acceptance. No new model, recognition or synthesis requests are used in verification.

Before/final sources, incident bytes, failed fixture runs, the reproduced browser race and corrected results live in `.tools/streaming-reply-41/`. Archive and restore this tree using `app.audits.archive_preparation`, not a historical character/speech finalizer. Owner microphone/speaker review, physical devices, sustained rendering performance and public-upload readiness remain pending.

## Native viseme-tail validation follow-up (October 25, 2026)

A local failed speech record showed that Azure had completed a 5,450 ms recording with 72 native visemes, but the final event was at 5,560 ms. The previous 100 ms endpoint allowance rejected that result by 10 ms. Speech validation now allows a bounded 250 ms event tail while still rejecting invalid IDs, negative or descending offsets, missing audio and missing spoken visemes. Original provider events remain unchanged; multi-part composition continues to clamp only its derived timeline. This correction uses the existing failed record, with no retry or new provider request. It was not test-run in this change.

Commit verification subsequently found that the native media and animation admissions still used 100 ms. API and worker admissions now share the 250 ms bound in `render-contract`. An offline regression covers the recorded 110 ms case, the 250 ms boundary and rejection at 251 ms, preserving raw events while keeping the derived animation on PCM. This adds no synthesis request or automatic retry; physical-device acceptance remains pending.
