# Natural conversational presence

Phase 118, October 29, 2026. The owner selected natural conversation, identified head/body movement as artificial, then clarified that both idle and speaking were too still. The final candidate therefore prefers acoustic opportunities while retaining bounded background activity when speech has no useful pause. This supersedes the initial plan's indefinite no-opportunity hold.

## Current behavior

The optional internal `conversational_motion.presence_version: conversational-presence/3` selects a video-loop-owned coordinator. Missing metadata keeps the original behavior. No public request, provider, schema, dependency or audio-buffer setting changes.

Speaking adjustments prefer existing pauses/unoccupied emphasis. Normal deadlines are 2–3 seconds, with acoustic opportunities allowed up to500 ms earlier but never closer than 2 seconds; quiet/reassuring delivery uses4–5 seconds with a4 second minimum. An uninterrupted phrase still receives small background adjustments. Speaking transitions take 650–750 ms, within the approved450–750 ms envelope. Idle uses550–900 ms transitions and1–1.7 second holds. Very small proposed changes become a distinct bounded adjustment rather than another long unchanged target. These are art-direction settings, not measured human biomechanics.

Each adjustment changes yaw by at most1°, pitch by 0.5° and roll by 0.15°, within the existing prepared bounds. Pitch/roll start35/70 ms after yaw; trajectories settle smoothly. Existing anti-sweep limits prevent broad opposite-side turns. Semantic gestures preserve their existing speeds, use available angular headroom around the held posture, and no longer pull the background pose toward center. An unexpectedly overlapping adjustment decelerates for 120 ms. Approved energy affects future actions rather than rescaling the visible pose. Interruption releases the remaining gesture over 240 ms. Listening no longer rescales the held head pose immediately.

Emitted PCM progress advances speaking motion. Prefix analysis replacement preserves the session and consumed cursor; newly discovered past cues cannot replay an action. Ending cues are not posture triggers. Underflow/waiting freezes speech-linked movement; idle/recovery uses the existing pause-aware animation clock. Existing gaze compensation follows the final head. Torso breathing, original facial/eye/oral/jaw/laughter paths, samples, visemes, transport and shaders remain unchanged.

## Verification and limits

Evidence is `.tools/natural-presence-118/`. The source is frozen phase 117 plus seven explicit file changes recorded in `source-manifest.json`; unrelated canonical edits and rejected historical rendering changes are excluded. `prepare.py` copies the current 120 resource files byte-for-byte and changes only the motion selector, identity record/package hash and matching private profile hash. `invariants.json` also confirms byte-identical API/contracts, PCM queue/progressive transport and GPU shader sources.

- `tests-final.log`: focused opportunity, activity, spacing, bounds, gesture composition, overlap settlement, prefix/underflow, interruption/phrase continuity, energy, pause/reduced-motion and selector checks pass. In the 30 second fixture, idle has 274 visible steps/900 with 1.733 s longest quiet run; uninterrupted speech has 221/900 with 2.333 s longest quiet run. Counts demonstrate activity, not owner acceptance.
- `worker-build-final.log`: focused native animation/adapter/media race suites and production native codec/GPU binaries pass. Legacy tests remain intact.
- `browser-baseline-v2/` and `browser-candidate-final-prefix/`: the same five cached Emma recordings/directions cover explanation, question, agreement, disagreement and reassurance, preceded by 10 seconds idle. Actual native H.264/Opus playback drains; normal-size frames show no new visible face/neck/material artifact. Sample contact sheets are inspection aids, not motion-capture validation.
- The final prefix test decodes audio before allowing complete synthesis to return, then plays the following phrases. No microphone, real model, recognition or synthesis request occurs. Provider-fixture completion remains explicitly controlled.
- Serial screenshot-free checks: baseline 29.89 fps and49.9 ms p95 frame gap; candidate 29.93 fps and44.3 ms p95. These brief desktop observations do not establish sustained/device performance. Cached command-to-audio observations are fixture timings, not live-provider latency benchmarks; the API and audio buffering are unchanged.

The initial fixture-counter assertion failure, preliminary candidate, failed preparation/build attempts and earlier recordings remain preserved. No image, audio, model or analysis generation was performed. Naturalness is a bounded internal result; owner preference and physical-device review remain open.

## Installation and reproduction

The local entry remains `http://127.0.0.1:5173/`, with `syntelle-render-native:local` / `syntelle-render-native-local`. The API/frontend images are unchanged. `installation.json` records the worker/image/package identity and healthy services. Private assets are in the dedicated `syntelle-natural-presence-118-assets` volume; profile/resources are selected from this experiment's package.

`sync_source.py` replays explicit changes over frozen117 without rewriting historical evidence. `worker.Dockerfile` uses pinned existing toolchains; `fixture.py` creates labelled isolated PostgreSQL/API/GPU fixtures with cached recordings and no provider credentials. Stop them through that same fixture script. Do not build from the unrelated canonical rendering edits wholesale.

`rollback.ps1` restores the exact preceding worker, package/profile settings and volume. The complete new sources, package, recordings, failed checks and receipts are stored through the generic `app.audits.archive_preparation` workflow. Successful restoration and byte comparison are recorded in the external `.tools/natural-presence-118-archive.json`; no historical finalizer is used. Publication isolates this change against the tracked baseline and preserves unrelated working changes.
