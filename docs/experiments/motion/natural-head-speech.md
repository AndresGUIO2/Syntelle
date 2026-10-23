# Natural head motion and Dragon HD local trial

The current rotation correction is documented in [Brief head rotations](head-turns.md). It supersedes the phase-12 movement below, preserves it under `head=natural-previous`, and uses `app.finalize_head_turns` for new evidence. Do not rerun the historical finalizer against the frozen phase-12 snapshot.

## Current result

The optional candidate is available at `http://127.0.0.1:5173/?trial=motion&character=syntelle&head=natural`; replace the character with `cafe` or `woodland`. The previous movement remains at the same URL without `head=natural`. This candidate is not promoted by default: Syntelle still falls below the strict desktop WebKit frame-rate target, and owner/real-device acceptance remains outstanding.

The owner's personal Speech resource is in `canadacentral`. Actual Go SDK synthesis succeeded with Ava and Andrew Dragon HD in English and Spanish. Four provider requests used 257 requested UTF-16 characters out of the separate 12-request/6,000-character allowance. Cache reads and browser review made no further provider requests. Character images, eyes, visemes, foregrounds and depth were not regenerated.

| Case | Requested voice | Language | Audio | Native events | Complete request time |
|---|---|---|---:|---:|---:|
| Syntelle English | Ava DragonHDLatestNeural | en-US | 4.25 s | 52 | 2.90 s |
| Syntelle Spanish | Ava DragonHDLatestNeural | es-CO | 4.35 s | 66 | 4.03 s |
| cafe/woodland English | Andrew DragonHDLatestNeural | en-US | 4.60 s | 56 | 2.47 s |
| cafe/woodland Spanish | Andrew DragonHDLatestNeural | es-CO | 5.05 s | 75 | 3.26 s |

These are short complete-response measurements, including transport and local persistence, not streaming first-audio latency or an Azure billing statement. `speech/` under `.tools/head-motion-12/` contains actual requests, settings, audio, IDs/offsets, results, database records and verified restoration evidence.

## Preparation and boundaries

`app.prepare_head_motion` adds `anatomical-head-rig/1` to the existing optional motion package. Cached MediaPipe XYZ supplies face geometry; depth continues it around hair and the silhouette. Semantic maps and anatomical distances supply common head/neck/torso weights and a neck pivot. No per-character placement overrides or manual masks are used.

Each 64-by-64 grid is validated over 375 sampled yaw/pitch/roll/breath configurations. The minimum triangle-area ratios are 0.670 for Syntelle, 0.270 for cafe and 0.298 for woodland; all are positive and pass the 0.20 gate. Preparation can reduce bounds when a grid fails. Replaying the three preparations produces byte-identical package files and reuses cached inputs.

Idle postures use seeded targets every 3–7 seconds, smooth transitions and quiet holds. Maximum configured idle angles are 1.5/0.8/0.6 degrees; speaking limits are 3/2/1.2 degrees. Audio energy peaks, separated by at least 1.2 seconds, produce restrained phrase-level emphasis. Posture blends into and out of speech rather than resetting. Breathing weights exclude the face. Mouth and eyelid patches are composed before the common deformation; backgrounds remain independent.

## Code boundaries

- `preparation/head_rig.py` derives and validates the optional rig; `prepare_head_motion.py` binds cached inputs and records it.
- Shared `headMotion.ts` plans posture/energy gestures; `speechTimeline.ts` validates Azure offsets, decodes the pinned WAV format and maps IDs to the existing nine poses.
- `useSpeechPlayback.ts` owns requests, cache, audio activation, the audio output clock, interruption and stale-operation guards. `useMotionRenderer.ts` applies expression and movement with persistent graphics contexts and partial texture uploads.
- `SpeechControls.vue` owns layout and explicit models for text/language/voice. Trial views select contexts and assemble controls.
- Go's small package retains separate HTTP validation, SDK provider and PostgreSQL ledger files. The five-second shutdown lifecycle remains tested. Speech is disabled unless explicitly configured; no public speech deployment was added.

The versioned `azure-nine-pose/1` adapter intentionally approximates tongue-specific Azure IDs with available poses. It preserves MBP closures and FV articulation, but nine images do not reproduce all native distinctions. Native callback arrival time never drives animation. Essential mouth articulation remains available under reduced motion; autonomous movement/blinking is disabled. Hidden tabs pause playback and animation.

## Local setup and reproduction

Create an owner-controlled Azure Speech S0 resource in an allowed HD region. The local trial verified Canada Central; region-scoped credentials must match it. Keep these values only in the root `.env`:

```dotenv
AZURE_SPEECH_KEY=your-own-key
AZURE_SPEECH_REGION=canadacentral
SYNTELLE_SPEECH_TRIAL=1
```

The existing local PostgreSQL preparation profile is also required. From the repository root:

```powershell
docker compose --profile preparation up -d postgres
docker build -t syntelle-api:speech-12 services/api
```

From `services/processor`:

```powershell
.venv/Scripts/python.exe -m app.prepare_head_motion
.venv/Scripts/python.exe -m app.run_speech_trial
```

The helper starts only the named local speech container on `127.0.0.1:8082`; Vite proxies its speech routes separately from the previous API. Restart this helper after changing server credentials or rebuilding its image. The normal trial/API remains usable without Speech configuration.

`app.verify_speech_trial --case ava-en|ava-es|andrew-en|andrew-es` is an explicit live verification command, not a transport retry mechanism. The four recorded cases have already been sent and refuse repeat execution. Use the stored results/cache for further inspection. The initial text in each browser character matches its cached English verification.

The official Go Speech SDK and native Linux library are both 1.52.0. The native archive is checked against SHA-256 `2573d5336cdba31759967b77b3596dcd450b802fa8c1a410245c541a5df0fb47`. The Debian CGO image archives its SDK license. Actual runtime versions, source and native archive are retained in the final evidence snapshot.

## Persistence and recovery

The separate `syntelle_speech` ledger commits input/settings and a transactional budget reservation before each paid call. It stores complete WAV bytes, IDs/offsets, SDK/native identity, provider result/status, errors and timestamps before accepting a result. It serializes active synthesis, binds request identity to text/language/voice/region/SDK/format, refuses automatic retries and halts on provider failures. Cancellation remains a counted attempt. Audio/context resources are released when the view is destroyed. Keys are never persisted or sent to the browser.

An integration-test isolation error was found before the first real call: changing `pgx.ConnConfig.Database` then calling `ConnString()` reused the original DSN. Twelve known fake records occupied the local ledger. Their exact three-byte fixture audio and empty provider identity established that these were not Azure calls. All original rows/timestamps were preserved in a separate halted fixture budget; no authorized limit was reset or increased. The corrected test constructs a distinct database URL and asserts the server's database name before mutations. It now verifies the development ledger stays unchanged. The incident is preserved under `test-ledger-incident/` and PostgreSQL snapshots 249/250.

`app.finalize_head_motion` audits source dependencies and request counts, freezes complete evidence, restores archived file bytes and verifies the speech ledger in a separately named temporary database. Existing image/analysis/eye budgets remain unchanged.

## Verification and remaining work

- 153 processor tests pass. Go tests/vet, the isolated PostgreSQL test and the Linux CGO build pass. Frontend lint, formatting, type checking and build pass.
- 64 Chromium/WebKit desktop/mobile-layout scenarios pass. Windows Playwright WebKit lacks Web Audio: those lanes explicitly verify failure before synthesis. A separate Linux WebKit run passes all 12 speech scenarios with actual Web Audio, including pause/resume and delayed-activation cancellation.
- Actual persisted Azure audio was replayed for all three characters, both languages and both layout sizes: 12 browser cases. Pausing freezes the character, replay uses the client cache, and no obsolete audio resumes after Stop. Instrumented mouth-uniform timing best matches the audio output clock at zero additional lag among sampled 0–100 ms offsets; this is a coarse diagnostic, not perceptual synchronization certification.
- Runtime exports cover all nine endpoints and all 81 directed/self pairs at five blends and three eyelid positions: 1,215 combinations per character, including EI/FV 35% and FV/A 41%. Endpoint jumps, exposed-iris changes and outside-eye edits are zero in the recorded invariants. Maximum-pose compositions were inspected against supplied backgrounds with blur off/on; no new displaced layers or silhouette tears were found in development review.
- Chromium stress measurements are approximately 60 fps. WebKit Syntelle desktop measures 57.6 fps versus 58.5 for the preserved movement in the same short stress scenario; other desktop characters and mobile viewport emulation exceed the requested thresholds. The sub-60 Syntelle result remains unresolved. Mobile viewport tests use desktop hardware, not real mobile GPU/CPU measurements.

Owner visual/phonetic acceptance, real-iPhone validation, strict Syntelle/WebKit 60 fps, first-streamed-audio latency and outstanding held-out/public-upload validation remain pending. This does not add microphone recognition, conversation, infrastructure or deployment.

References: [Speech quickstart](https://learn.microsoft.com/azure/ai-services/speech-service/get-started-text-to-speech), [regional support](https://learn.microsoft.com/azure/ai-services/speech-service/regions?tabs=tts), [Go SDK](https://github.com/microsoft/cognitive-services-speech-sdk-go). The OrgLens reference was inspected read-only for native viseme integration; no work-account credentials were reused.

## October 20 speech continuation

The original allowance was exhausted at twelve received requests and 1,921 characters; no synthesis was stuck. The owner then explicitly authorized a separate twenty-request / 5,000-character allowance, recorded in `docs/characters/speech-continuation.json`. Active budget `authorized-dragon-continuation-2026-10-20` is inserted independently; the previous budget and all twelve records remain unchanged. Exact cached recordings from either authorized budget are resolved before limits and do not consume a new request. The unrelated halted synthetic-fixture incident budget is excluded.

New reservations retain transactional limits, one active synthesis, no automatic retry, provider-failure stopping and server-side credentials. Database errors, request exhaustion, character exhaustion and active synthesis now produce separate messages. No public interface/schema change or new image allowance is introduced. Focused Go tests, a separately created PostgreSQL integration database and the pinned Linux Speech SDK build pass. Verification evidence is retained in `.tools/speech-continuation-27/`; current usage remains authoritative in PostgreSQL rather than this historical document.
