# Emma laughter and conversational presence

## Owner request and current result

Phase 107 implements the approved natural/expressive blend with existing character resources. The local demo uses `syntelle-render-native:emma-presence-107` and a profile derived from phase 106. Emma and the original A/V playback element/rate remain selected. No image generation/analysis, new dependency/model, public endpoint, cloud deployment or additional conversation synthesis was introduced.

Two separately reserved nonverbal requests succeeded: `[laughter]` with `en-US-Emma:DragonHDLatestNeural` in en-US and es-CO, using the personal Canada Central resource. The experiment `owner-requested-emma-laughter-2026-10-27` consumed exactly two attempts / 20 UTF-16 characters, without retries or historical budget changes. English lasts 2,100 ms; Spanish 850 ms. Both preserve the full original PCM16/24 kHz WAV and native events, satisfy the existing 2.5-second maximum and play through Linux WebKit audio.

**Both recordings are audibly approved and active.** The owner listened to the original clips and replied "Ambos suenan bien". Exact audio-hash-bound acceptance receipts are stored in PostgreSQL and `audible-review/`; new immutable `profile-v3` enables both preview and automatic laughter through the existing content/cooldown policy. No new synthesis request was made. The earlier pending profile-v2 and snapshot 762 remain preserved. The agent did not claim to hear audio itself; the real owner supplied the required audible classification. Other vocal styles remain unqualified.

## Reusable preparation

Run `app.workflows.speech.qualify_laughter` from `services/processor`. `--catalog` freezes regional availability; `--language en-US` and `--language es-CO` use the existing private SDK path, with committed request inputs before calls. The separate identity and advisory lock bound two fixed language slots. Completed application-rejected results remain immutable and do not reopen a slot; provider/uncertain failures halt new calls. Complete request/results/audio are exported from PostgreSQL, including failed outcomes and timeout recovery through `--export`.

`--review LANGUAGE --decision accept|reject` records an actual owner listening decision bound to the exact original WAV SHA-256. It must only run after that listening decision exists. `--prepare --profile-name PROFILE-REVISION` exports a new immutable profile; only language-specific accepted listening receipts enable recordings. Keep prior profile revisions and the full experiment history. A later owner response needs no synthesis or budget reset: record the decision, export a fresh profile revision, select it in the local Compose override and restart the API. The existing availability path then restores preview and automatic eligibility for each accepted language.

## Speaking presence

The existing attention/posture directors now select `conversational-presence/1`. Speaking adjustments take 280-500 ms with irregular 800-1,600 ms holds. Reassurance or worker-approved energy at or below 0.35 uses 1,600-2,800 ms holds. Proposed mood intensity cannot bypass the worker's effective direction. Near-identical speaking targets remain quiet without extending holds by the former extra 900 ms. Idle timing/random consumption remains unchanged.

Speaking varies diagonal leadership instead of making every adjustment a lateral turn. Posture ceilings stay 3.6 degrees yaw, 1.7 pitch and 0.45 roll, further limited by the same qualified character bounds. Opposite-side sweeps remain blocked. Existing semantic nod/shake speeds, eight-cycle rolling admission, 2.2-second spacing and non-overlap remain unchanged.

Speaking glances have shorter 250-600 ms holds, then return toward the viewer. Head-follow stays disabled. Standalone semantic brow cues now share the existing competing-motion suppression envelope; shared head/brow anticipation/release and final-head gaze composition remain intact. Existing mood, smile/contact protection, torso breathing, pause, reduced motion and phase-102 laughter recovery remain authoritative. No new animation controller or unsupported body motion was added.

A deterministic 30-second state trace changes background moving steps from phase 105/106's 139/871 (16.0%) to 212/871 (24.3%), maximum continuous adjustment 0.467 seconds. Quiet delivery has 133 moving steps and unchanged idle 141 in the corresponding diagnostic. These are state diagnostics, not a realism quota or frame-rate measure.

## Verification and material limits

- Focused native animation/adapter/media race suites and API speech/conversation/streaming/shared-contract checks pass. Tests cover approved quiet energy, brief timing, bounds, opposite-side sweeps and phrase continuity; the existing pause/contact/laughter regression suites remain passing.
- Ten processor tests cover identity/PCM/event gates, original-byte preservation and hash-bound audible approval before enablement. Three desktop/two mobile player checks plus frontend types pass with the unchanged phase-106 playback path.
- Matched English comparisons replay three recent actual Emma deliveries and directions (affirm/explain/playful), with original provider WAVs restored and SHA-256 checked against delivered records. Spanish reuses the exact phase-106 cache with explicitly authored offline explain/ask/reassure directions. Neither fixture makes provider/model calls, and its PostgreSQL database is isolated from owner history/budgets.
- Both actual laughter recordings pass isolated Playful 50%/100% completion, interruption and subsequent exact cached speech playback. Existing CPU state regression checks cover pause and continuous recovery. Browser recordings are retained, but their visual fluency is limited by the occupied GPU.
- Linux WebKit plays both full cached laughter clips. This excludes WebRTC/video and audible classification.
- The serial 30-second comparison measured about 2.43 fps baseline / 2.23 fps candidate while the GPU stayed at 100%. P95 gaps were about 561 ms in both; maxima about 884/917 ms. Resource samples and initial recording/software/visibility diagnostics are preserved. These observations do not qualify the 30 fps target or establish a motion-related performance regression; unrelated GPU contention prevents an isolated conclusion.
- Five diagnostic marker/tone pulses show no measurable extra decoder-to-canvas delay, with rate 1.0 and the original audio/video tracks. Under the same GPU saturation, received image lag behind audio analysis was 367-550 ms. This offline marker path uses readback, so these values are diagnostic rather than normal-demo latency. Phase-106 mouth sampling and receiver source code remain byte-identical; end-to-end real-time quality must be checked with GPU capacity available.

Initial missing-PCM replay errors were corrected by restoring the original provider WAV from PostgreSQL and verifying its delivered hash, rather than substituting unrelated audio. A too-short follow-up playback deadline was corrected for the full multi-phrase reply. Original diagnostics remain preserved. Neither issue consumed paid calls.

## Delivery and rollback

The local demo is installed with new movement and owner-audibly-approved profile `profile-v3`. Run `.tools/emma-presence-107/install.ps1` for this review candidate or `rollback.ps1` for the exact phase-106 API/web/worker/profile. The prepared package hash remains `eb1c1df5e15d773cab77da7f877f048447f3b2b1cffb6865524d69d645745da9`; all geometry, paired views and oral/eye materials remain unchanged.

`app.audits.finalize_emma_presence` archives complete sources, requests/results, original clips, profiles, decisions, rejected diagnostics, comparisons and worker binaries in PostgreSQL. It restores the new snapshot and unchanged resource snapshot 730 with byte checks. Parents 753/754 preserve the previous complete source/runtime closure. The receipt is `.tools/emma-presence-107-archive.json`.

Final naturalness, an uncontended real-time comparison, full WebKit streaming and physical-device qualification remain open. Owner audible confirmation is complete for both clips. The original two-request allowance stays exhausted regardless of these later reviews.
