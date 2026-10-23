# Contextual performance direction

The centered speaking posture described below is superseded in the default local demo by [conversational speaking attention](speaking-attention.md). It remains available at `/?attention=previous`. Direction, expressions, rejected laughter materials and provider evidence retain their existing status.

October 20, 2026. Experiment `authorized-character-performance-2026-10-20`; evidence `.tools/character-performance-34/`.

## Delivered behavior

The local conversation director requests `performance-intent/1` structured output in the existing Gemini 3.8 Flash Interactions request. The internal `Director` interface retains the preceding `Model` name as an alias. Another trusted controller can implement this interface without controlling geometry, assets or executable actions. The application prompt and JSON schema live in `internal/conversation/performance.go`; no Codex skill runs in the application.

The envelope contains spoken `text`, a bounded semantic `tone`, and optional `nod`, `shake`, `emphasis`, `smile` and `laugh` cues. Each anchor must be a unique verbatim span. Unknown cue fields, invalid anchors and excess cues are omitted. A strict envelope prevents JSON, bracket tags and control data from reaching synthesis. A complete decoded text field is emitted before the direction tail completes; incomplete JSON strings and provider reasoning are never displayed. The completed envelope is validated before speech preparation. The original provider event stream remains immutable evidence.

The server resolves head cues against decoded PCM energy and pause boundaries near proportional text-span estimates. This is approximate timing, not word alignment. At most two head gestures start at least 2.4 seconds apart. Pitch nods, alternating yaw shakes and small emphasis beats use fixed application amplitudes within the existing ±3° yaw / ±1.2° pitch / ±0.3° roll envelope. Negation is a direction decision rather than a keyword rule.

Optional `performance` metadata is attached to the existing prepared-speech payload. The separate `performance` operation stores validation, omissions and resolved timing in PostgreSQL; the original speech operation, WAV and native events are unchanged. Historical payloads without metadata remain playable. Cancellation and obsolete-turn handling keep their existing identifiers and queue behavior. Read aloud makes no direction/model request.

The demo eases idle head and glance contribution to zero over 700 ms while speaking, retaining the existing breath and blink clocks. Idle returns over 1.8 seconds afterward. A directed nod or shake returns to center. The unconditional completion smile is removed from this behavior. A directed closed-mouth smile uses a short silent tail on the same audio clock, after articulation is available, and a 30-second local performance-session interval. Automatic expressions and gestures respect reduced motion. The preceding demo motion/expression behavior remains selectable at `/?performance=previous`; laboratories retain their preceding defaults. Compare the same cached spoken audio and camera framing.

## Laughter status: rejected facial composition, disabled

Both actual Ava samples were audibly reviewed by the owner as usable. The current voice remains `en-US-Ava:DragonHDLatestNeural` on the personal Canada Central resource; no alternate voice, region or provider was tried. The English laugh is 1.65 seconds and the Spanish laugh 1.15 seconds. Audio and full native results are cached in the new experiment.

The initial image had coherent expression but changed face scale: stable registration scale 1.58461 against an expected 2.0 failed the existing registration gate. The sole visual correction passes stable registration (scale 2.03389, anchor RMSE 6.93 px). Both original outputs and request records are retained.

The offline companion registers the outer lip seam, cheeks and eyelids together using semantic MediaPipe rings. Closed neutral aperture points are not triangulated as skin: near-coincident detector points would invert when a hole opens. An independent inner contour clips the cavity and opaque canonical dental sheets. Original eye material supplies the visible irises; generated iris and dental pixels are not installed. All pixels outside the automatic facial support remain source-exact. There are no manual character masks or placement overrides. The common topology gate reduces the final deformation to 85%.

Actual composition review rejected `companion-corrected/` for dental/cavity seams. `companion-corrected-v2/` fixes rectangular cavity sampling at the wider corners and premultiplied filtering, but still shows an unnatural dark upper dental strip and a lip/material seam. Numerical non-inversion checks do not establish visual acceptance. Neither package is installed or supplied by the demo catalog. The existing character resources remain unchanged. The two-image allowance is exhausted; no further generation or contour analysis is authorized by this experiment.

The optional playback infrastructure supports a validated companion, checks voice/language/region and audio hashes, and schedules the original spoken clip plus cached laughter as one interruptible audio buffer. Its facial envelope follows energy inside the known laughter segment. Policy permits at most one laugh, at least 60 seconds apart, never in consecutive turns, and only playful or demonstration direction. Unavailable companions omit laughter while normal speech continues. Fixture tests exercise this path; the actual application keeps it unavailable because composition review failed. This is not a claim that complete facial laughter has been delivered or visually accepted.

## Capability evidence and ceiling

| Kind | Authorized | Actual | Result |
| --- | ---: | ---: | --- |
| Gemini structured probes | 2 | 2 | Both completed; English playful and Spanish serious |
| Ava TTS probes | 2 | 2 | Both completed; owner accepted audible output |
| Images | 2 | 2 | First registration rejected; correction used for rejected compositions |
| Contour analyses | 0 | 0 | None |
| Retries | 0 | 0 | None |

`cmd/performance-probe` accepts exactly four fixed probe slots. A PostgreSQL session lock serializes the experiment; input records/reservations commit before provider calls. Existing started, failed or uncertain work prevents another call, and received slots cannot be rebound. Image preparation reuses the established recorder with a separate two-image/zero-analysis preparation budget under the new identity. All four capability probes and the owner's audible review are prerequisites to image expenditure. The correction requires a hash-bound rejection of the first image. Older allowances are not reopened.

The [Gemini Interactions reference](https://ai.google.dev/api/interactions-api.md) documents JSON-schema `response_format`; the actual two probes established it for the pinned model. [Azure HD documentation](https://learn.microsoft.com/en-us/azure/ai-services/speech-service/high-definition-voices) lists laughter paralinguistics; the actual voice/resource/language samples, rather than documentation alone, established usable audible output. No word-exact timing, unsupported bookmarks or silent voice substitution is claimed.

## Reproduction and review

Do not rerun paid probes: their allowance is exhausted and their original request bindings are immutable. Replay `.tools/character-performance-34/tts-en.wav` and `tts-es.wav`. Inspect full provider replies, audio review, both image requests, rejected registration, composition previews and package-specific visual reviews in that directory. `app.workflows.motion.bind_laughter_reference` is an offline preparation command and refuses rebinding an existing completed package. Earlier failed source snapshots remain in PostgreSQL. `app.audits.finalize_performance` archives the final source/evidence tree and verifies restoration independently; do not use historical finalizers.

The remaining acceptance item is a visually acceptable facial laughter composition. Owner visual acceptance, physical-device checks and public-upload validation remain pending. This experiment does not deploy a public control endpoint or public speech access.

## Bounded verification results

The twelve Chromium conversation/performance cases pass using intercepted services and cached audio. They include malformed metadata, gesture spacing, both shake directions, stationary neutral speech, absent completion smiles, cooldowns, microphone errors, transcript streaming, obsolete turns, interruption, composer layout and customization focus. Three Linux WebKit cases pass, including real Web Audio playback, pausing/resuming inside the cached laughter segment, interruption and the consecutive-turn limit. The latter exercises an explicitly isolated fixture companion, not the rejected production facial asset. Windows WebKit's lack of Web Audio is not counted as a playback pass.

Native Linux Go tests pass with race detection and an isolated, identity-verified PostgreSQL database on a network with no external access. A directed-turn fixture verifies that resolved timing is committed while original provider evidence and WAV bytes remain unchanged. The default build, frontend type checking, scoped lint and validation-only production build pass. No performance benchmark, phone validation or owner facial approval is inferred from these checks.

Preserve initial harness failures: the integration fixture's default WebSocket read limit was too small for its WAV, and the first Linux browser URL lacked a secure localhost context. The fixture now permits its known audio size and runs through a localhost proxy to the dedicated offline Vite server. Neither correction changes production access policy or contacts a provider. Offline preparation failures and the two rejected composition packages remain separate evidence.
