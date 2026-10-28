# Mouth presentation synchronization and Emma

## Owner correction

The owner reported a late-looking mouth and rejected the current Serena voice. This local correction selects `en-US-Emma:DragonHDLatestNeural` and removes an additional receiver video playback stage. It preserves the phase-105 brief posture behavior, phase-102 laughter recovery, prepared geometry, paired views, oral textures and native articulation timeline. It introduces no images, analysis calls, models, dependencies, public endpoints or deployment.

## Presentation path

Previously, background composition decoded native video into a hidden source, drew it into a canvas, captured that canvas as another video track and played it in a second element alongside the original audio. The inline player now keeps the original native audio and video tracks on one playback element and draws that element directly into the visible background canvas on its frame callback. Audio rate remains 1.0. There is no audio delay, viseme offset, longer mouth transition or changed gesture speed.

Mute, interrupted-audio suppression, background blur and source cleanup remain owned by the existing player. Picture in picture alone lazily captures the composed canvas as a video-only track; original audio continues on the native element. Capture tracks stop on exit/denial/disposal. Safari presentation-mode cleanup is explicit. This does not claim a zero-latency PiP presentation path or physical-device approval.

## Emma qualification

The personal Canada Central catalog confirms the Emma HD voice and English/Spanish language support. Its catalog lists Spanish locales rather than es-CO specifically; actual `es-CO` output was therefore checked separately. `app.workflows.speech.qualify_voice_change` performs a separate bounded two-request check, reserving complete inputs before SDK calls in PostgreSQL. Failed/uncertain slots cannot retry. It does not reset the earlier Serena or other budgets or limit ordinary owner conversation.

Both English and Spanish returned complete 24 kHz PCM and native visemes; exact provider results and word events remain archived. Historical Ava, Andrew and Serena identities/bindings remain explicit. Serena's experiment command retains its fixed historical voice identifier. The active capability file enables contextual plain Emma only; explicit styles remain empty. Emma has no qualified laughter companion, so automatic audible laughter and its preview are disabled. Existing facial resources and recovery remain intact, and no other voice is inserted between Emma phrases.

## Bounded verification

- Eight focused Chromium desktop player checks and three mobile player checks pass: original track identity, unchanged rate, no inline recapture, alpha/background pixels, mute/retry and PiP lifecycle.
- Shared contract and native animation/adapter/media race checks pass. Focused API speech/conversation/streaming race checks pass, including separate historical cache bindings and rejection of cross-voice capabilities. Frontend types, scoped lint and player build pass.
- A real isolated native replay delivered four exact cached Emma phrases through reservations and the installed presentation component, including curious/affectionate facial directions. No synthesis/model calls were made by replay. The recording and exact events are retained.
- Linux WebKit plays the actual cached Spanish Emma PCM through Web Audio. This excludes WebRTC/video; the earlier full-WebKit-streaming gate remains open.
- Five native RTP marker/tone pulses per variant compare the actual old/new player under identical source transport. Median added displayed-video delay relative to the decoder reference fell from 16.8 ms to 0 ms; maximum from 33.4 ms to 16.7 ms. Display relative to received audio analysis had median 33.4/16.7 ms and maximum 50/33.4 ms. Sampling has approximately one 16.7 ms display-tick resolution. These are bounded software receiver observations, not a physical-speaker, phoneme-alignment, sustained-performance or universal synchronization guarantee.
- The marker-only offline probe adds a coverage tile to its test scene so the marker survives background composition; this optional path is not used by the normal worker. Initial harness failures, default mute assumptions and fixture cleanup-label corrections are preserved. They consumed no provider attempts.

The new installed-candidate check confirms Emma, both native tracks, rate 1.0, visible composition and disabled unqualified laughter. Owner naturalness and physical-device review remain pending.

## Reproduction and rollback

Evidence is in `.tools/av-sync-emma-106/`; `measure-native.mjs`, `fixture.py`, `review.mjs`, `installed-check.mjs` and `webkit-pcm.mjs` use offline native/cache inputs. The matched baseline web image is `syntelle-web:before-av-sync-emma-106`; the prior worker is the phase-105 worker. Do not rerun the paid check to obtain more samples.

From the repository root, run `powershell -NoProfile -File .tools/av-sync-emma-106/install.ps1` to install the local candidate, or `powershell -NoProfile -File .tools/av-sync-emma-106/rollback.ps1` for the exact preceding API/web/worker and profile. The finalizer `app.audits.finalize_av_sync` stores sources, complete requests/results, failures, recordings, catalog, profiles and binaries in PostgreSQL and restores the snapshot to a new directory, checking every file hash. Unchanged prepared resources remain in verified snapshot 730; parents 744/746 preserve the preceding performance and posture sources. The receipt is `.tools/av-sync-emma-106-archive.json`.
