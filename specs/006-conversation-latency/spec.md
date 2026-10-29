# Faster conversational replies

**Status:** approved for implementation by the owner's explicit request · **Date:** 2026-10-29

Source request: “Implementa esas mejoras para que empiece a hablar más rápido, pero sin cortarme cuando sigo hablando.”

- **REQ-1:** Quiet confirmed speech becomes a turn earlier than the preceding fixed 1,800 ms post-recognition wait. Given live microphone PCM and a resolved utterance, the quiet deadline uses observed silence rather than adding the entire old delay again.
- **REQ-2:** Continuing speech prevents premature submission. Given renewed microphone speech, queued input, or unresolved recognition, no quiet deadline may submit a partial thought; preserve confirmed fragments and the bounded idle recovery path.
- **REQ-3:** The first eligible reply phrase can begin playback before synthesis completes. Given progressive provider audio, retain ordered original PCM, native visemes and cancellation; do not repeat the opening when the complete recording arrives.
- **REQ-4:** Preserve exact voice, approved emotional delivery, complete replies, historical recordings, request-before-call evidence, failure/no-retry policy and private audio boundaries. Failed partial audio must not become a successful cache entry.
- **REQ-5:** Verify changes with bounded offline fixtures and preserve the active local runtime for rollback. Do not make paid requests, change character geometry/assets, or qualify physical-device latency without measurement.

Non-goals: new providers/models, semantic end-of-turn inference claims, public deployment, paid benchmarks, or a guarantee that arbitrary future speech can be predicted from silence.
