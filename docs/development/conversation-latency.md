# Faster turn completion and first audio

October 29, 2026. Local phase117 changes the active phase116 API and phase114 native worker through an explicit frozen-source patch manifest. It preserves the current character package, profile, Emma voice and approved delivery, frontend image and historical evidence.

## Turn boundary

Azure retains its 700 ms segmentation setting. The socket now inspects the existing 16 kHz PCM in 20 ms windows without retaining microphone bytes. Sustained activity uses an 80 ms latch and RMS 0.012 threshold; even the first above-threshold candidate frame postpones a due deadline. Confirmed text normally waits for 1,100 ms of observed silence in total, with a minimum 250 ms after the final callback. This replaces adding 1,800 ms after all provider processing on the usual fresh-audio path. Missing/stale activity evidence retains the old conservative fallback.

Obvious trailing conjunctions/prepositions in English/Spanish and ellipses retain 2,500 ms of quiet. This is a limited pause heuristic, not semantic certainty that a thought has ended. Partial/start callbacks suspend the quiet timer. PCM beyond the final result's confirmed audio interval also prevents release while recognition is catching up. Queued microphone/recognition input wins over an expired timer. The existing four-second idle watchdog is refreshed by actual local voice as well as provider continuation callbacks; it cannot release while fresh voice is arriving. Confirmed-only text, deduplication, echo checks and stop/disconnect behavior remain.

New recognition records describe `audio-quiet/1` and its timing values. Historical records are unchanged.

## Progressive opening

The model is asked for a useful short complete opening, normally 5–12 words, followed by the full answer. This is not a response-length cap. A first phrase with no requested gesture cues and at most 240 characters is eligible for progressive native synthesis. Phrases requiring anchored gestures or laughter retain complete-recording preparation; all later phrases retain their existing path.

The pinned Speech SDK's `Synthesizing` callbacks provide raw 24 kHz PCM. Immutable WAV prefixes carry original samples and native visemes over the existing private worker connection. The first prefix needs at least 800 ms of audio and native viseme coverage; later snapshots advance by at least 400 ms. The worker retains 200 ms of lookahead, prepares each immutable prefix outside its audio lock, and replaces analysis without resetting animation identity or the PCM cursor. A stalled prefix freezes the source cursor and emits silence; it is never treated as a complete phrase. Native admission bounds an opening to 30 seconds. When complete synthesis is validated and archived, the final recording extends the same clip without replaying the prefix. Short recordings or providers with late visemes may finish before progressive delivery is possible.

Worker-approved emotional delivery is established before synthesis. A streamed opening remains quiet throughout: whole-recording acoustic emphasis is not added retroactively at completion. Following phrases retain normal cues. The first browser proof exposed and preserved precisely that final-direction mismatch before it was corrected. First-PCM activation is archived immediately, so echo checks do not wait for the native recording to finish. Input records precede provider calls; partial output has separate `speech-prefix` evidence and is not a reusable successful speech recording. Failure cancels remaining playback without retry/fallback. The public browser protocol never receives native prefixes.

## Bounded evidence

All results are in `.tools/conversation-latency-117/`:

- `endpoint-baseline.log`: the preceding implementation fails the new responsiveness check at **1,802.13 ms** from confirmed result to turn.
- `integration-final.log`: the candidate releases that same controlled quiet case in **402.46 ms**, about **1.40 seconds earlier**. A 4.4-second PCM continuation with no SDK start/interim callback remains unsent, including a subsequent quiet interval awaiting its final result. The two original fragments become exactly one model turn.
- The separate `integration.log` also passes existing segment grouping and unresolved-interim recovery. PostgreSQL fixtures construct and verify disposable database identities; no test database remains afterward.
- Speech and delivery fixtures prove request-before-call, prefix-before-completion, early activated-only echo history, cache reuse, cancellation and failure without successful partial caching. Native fixtures prove exact ordered PCM, no repeated opening, duplicate/stale rejection, missing completion detection, underflow behavior, released buffers and stable animation identity. API/public-boundary and native race checks pass in the pinned Linux builds.
- `browser-v2/report.json`: actual Chromium/Vue/private API/native GPU/H.264/Opus replay decodes cached Emma audio **before the fixture allows complete synthesis to return**, then drains the complete phrase. Command-to-decoded-audio was 141.5 ms with already cached audio/model fixtures. This number **excludes real recognition/model/TTS latency** and is not a live-provider benchmark. There were zero microphone calls and zero paid provider calls. `browser/` preserves the failed first proof and its diagnosis.
- `installation.json` records healthy API/frontend and unchanged frontend image; `runtime-before.json`, `install.ps1` and `rollback.ps1` preserve exact local images. Runtime names remain `syntelle-api:latest`, `syntelle-render-native:local` and `syntelle-render-native-local`. The profile and `syntelle-emotional-presence-114-assets` volume are unchanged.

The browser proof predates only the final buffer-release cleanup/first-frame guard; their focused tests and final image builds pass. No renderer geometry, prepared image, provider identity, dependency, cloud endpoint, schema or budget changed. A long pause followed by a new thought cannot be predicted from silence. Quiet/noisy microphones, actual provider chunk timing, physical phones, owner naturalness and total microphone-to-speaker latency still need live user observation; no paid qualification call was made.

## Reproduction and rollback

`source/` contains the complete frozen build input. `source-manifest.json` records the explicit additions/patches over phase116 API and phase114 worker, excluding unrelated/rejected working-tree rendering changes. `sync_source.py` reconstructs this source without modifying historical folders. API/worker Dockerfiles use the existing pinned local toolchain images. `endpoint-baseline/` retains the old endpoint implementation with the new test.

Use the generic `app.audits.archive_preparation` archive/restore command for this directory and verify restored hashes. The external `.tools/conversation-latency-117-archive.json` receipt records the resulting snapshot and byte verification. Do not invoke a historical motion finalizer. Run `.tools/conversation-latency-117/rollback.ps1` only to explicitly restore the preceding API/worker images; it leaves profile, resource volume and frontend unchanged.
