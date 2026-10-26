# LLM-directed emotion previews

The local player supports a persistent user baseline and temporary phrase-level emotional direction. This is an implementation and calibration preview. **The ten-state visual acceptance requirement is not complete.** Neutral and joy read in the inspected samples; the other eight states remain explicitly unqualified for distinct mood readability at normal display size. No owner acceptance is claimed.

## Current local resources

- Native assets: `.tools/emotions-82/package-v11/`.
- Prepared inputs: `.tools/emotions-82/prepared-v11/resource-bundle.json`.
- Private performance profile: `.tools/emotions-82/active-profile/profile.json`.
- Stable image/container: `syntelle-render-native:local` / `syntelle-render-native-local`, started through `.\dev start`.
- Prior assets/profile remain in `.tools/speech-lighting-81/`; the preceding image is preserved as `syntelle-render-native:before-emotions-82`.
- `.tools/emotions-82/activation.json` records the three nonsecret local settings needed to restore the preceding configuration.

No model, speech-synthesis, image-generation or contour-analysis requests, model installations, database migrations or cloud changes were made. Two live recognition initializations were inadvertently triggered by the entry button during movement checks. Both were cancelled with 0 ms of recorded audio and no recognized segments. These records do not establish zero provider connections or independently verified billing. The corrected movement harness starts through the gesture preview without a microphone; see `live-provider-audit.json`. Ava synthesis, existing cached laughter, accepted head geometry/amplitudes, canonical dental materials and foreground resources retain their existing behavior.

## User controls and timing

Customize contains an emotion selector, intensity slider and Reset emotion alongside the existing facial controls. States are `neutral`, `joyful`, `playful`, `affectionate`, `curious`, `sad`, `angry`, `fearful`, `surprised` and `disgusted`. Neutral starts at zero; a non-neutral selection with zero intensity starts at 50%. Settings persist through the existing local preferences and authenticated settings paths. Only the baseline is saved; contextual targets do not write preferences.

The contextual-expression toggle enables temporary LLM variation. Emotion persists through phrase gaps. A reply ending, interruption or disabled variation returns to the selected baseline over 900 ms. New emotional targets use a 500 ms transition on the existing pause-aware animation clock. Reduced motion presents the baseline without contextual emotional transitions. Movement off removes emotional head-strength variation while retaining facial controls.

Changing the baseline during a reply suppresses that reply's remaining emotional targets without stopping its PCM. The next reply receives the new baseline. Manual smile and brow values remain adjustments, composed once within the prepared shared limits. Negative mouth actions constrain competing smile strength; the optional emotion pathway uses the prepared speaking range without the historical blanket half-strength reduction. Packages without the optional rig retain historical animation/rendering behavior.

## Director and private activation

`performance-intent/3` adds `emotion: {state, intensity, energy}` to each streamed phrase. Both numeric values are finite normalized values. Dialogue, emotional targets and semantic cues come from the same response; there is no classifier request. The director receives role instructions, the baseline and the last **activated** contextual target, with continuity instructions. Agreement/disagreement determines nods/shakes; energy can scale movement only within accepted bounds. Versions 1 and 2 remain readable.

Invalid optional emotion retains valid text and leaves the previous performance target intact. Direction is persisted before admitting its associated audio. The worker reports activation only when the corresponding prepared PCM starts and its encoded block is actually sent. Feedback includes turn, phrase, generation and baseline. Cancelled, stale, duplicate, suppressed, preview and unsupported-rig activations cannot enter subsequent context. Browser conversation JSON cannot populate the private activation field.

Direction and activation use existing PostgreSQL JSON operation records. Activation request identities include turn, phrase and generation. The integration-only fixture exports operation evidence and the actual contexts supplied to its deterministic director; no new production endpoint exists.

## Preparation, masks and visemes

The optional source-bound `emotion-rig/1` reuses cached 478-point MediaPipe anatomy, existing registered expression references, displacement fields, eyelid machinery and the joint oral rig. Emotion definitions combine reusable brow, cheek, lid, smile and lip-tension controls. It does not produce emotional viseme image sets.

Eye contents, canonical enamel, tongue/cavity ownership and the central lip/contact seam are protected. Lip actions deform the existing joint geometry while dental samples remain rigid and opaque. A continuous piecewise affine corner shear replaced an earlier column-based deformation that folded thin closed-mouth corner triangles. The accepted oral triangles are unchanged. The patch perimeter stays attached; central FV/MBP contact vertices remain pinned. This single-character calibration does not establish automatic visitor preparation.

Mask boundaries are a visual risk: protected regions can make neighboring facial motion look stiff or disconnected even when pixels and geometry pass. The body field therefore excludes mouth actions, which are solved with articulation instead of independently deforming tooth-containing images. Existing geometry gates were retained: Jacobian at least 0.45, gradient below 1.5, inverse residual at most 0.2 px, joint triangle ratio above 0.2. Twelve inverse iterations are used only by the new optional pathway; legacy packages retain their previous solver.

All ten quantized fields and 45 transition midpoints pass the bounded field checks. Five negative lip controls pass 45 configurations across each of 81 safe pairs; focused competing-control tests cover Rest/MBP, Rest/FV and open vowels. These measurements **do not qualify appearance**. Failed preparations and corner-fold diagnostics remain under `.tools/emotions-82/prepared*` and `oral-failure-diagnostics.jsonl`.

## Verification and remaining limits

- Baseline persistence/default/reset: Chromium desktop and WebKit mobile; existing facial controls/previews remain covered.
- Optional metadata, legacy versions, interruption, baseline edits, pause/reduced motion and stale activation: focused Go/race checks, including an isolated PostgreSQL database created and verified before fixture mutation.
- Actual cached audio: three contextual targets across a reply; a baseline edit suppresses later targets without cutting playback; cancellation excludes queued emotions. A subsequent response receives the last played target. Actual supplied contexts and JSON operation evidence are saved in `player-review.json` and `context-check.json`.
- All ten baselines replay cached speech through the actual CUDA/NVENC player without invoking a model or provider. Screenshots and a video are preserved. This is visual inspection material, not proof that eight unqualified emotions read distinctly.
- Neutral and original manual facial rendering match the preceding package byte for byte. Existing texture records and geometry match; canonical tooth buffers stay unchanged across all ten emotional states and focused MBP/FV/open-vowel combinations.
- All 38 CPU/CUDA recorded frames match exactly, with byte-exact current-frame software fallback. See `cpu-gpu-review-v11/comparison.json`.
- Pose-clock evidence samples nod/shake trajectories at 30 Hz without quantized holds. Browser records compare incoming source RTP/media timestamps with composed presented frames and landmark-region motion. Short checks show approximately 30 presented fps, with occasional 50–67 ms gaps. The reported 5 fps was not reproduced. **The strict p95 below 50 ms gate remains unqualified; no stutter fix or physical-device acceptance is claimed.** An incomplete fixture-expiry measurement and all failed checks remain preserved.

Read `visual-qualification.json` for individual state status. Do not treat supported state names, a successful package export, numerical geometry, screenshots or the local preview installation as owner acceptance or production readiness. No replacement references were generated to hide failures.

## Offline reproduction

From `services/processor`, use a fresh output directory:

```powershell
.venv/Scripts/python.exe -m app.workflows.motion.prepare_emotions --bundle ../../.tools/speech-lighting-81/prepared-v2/resource-bundle.json --landmarks ../../.tools/rgbd-motion-18/prepared-current/syntelle/inputs/landmarks.json --output ../../.tools/emotions-82/reproduction
.venv/Scripts/python.exe -m app.workflows.rendering.export_native --bundle ../../.tools/emotions-82/reproduction/resource-bundle.json --output ../../.tools/emotions-82/reproduction-package
```

Preparation rejects an existing output directory and records inputs, source files and failures. Use the generic `app.preparation.archive.Archive` workflow through `.tools/emotions-82/archive-final.py` for complete source/resource/evidence snapshots and hash-verified restoration. The resulting external record is `.tools/emotions-82-archive.json`. Do not run historical experiment finalizers against these sources.


## Positive-emotion mask correction (phase 83)

The owner reported mask artifacts in joyful, playful and affectionate. Phase 82's internal joy readability observation did not establish acceptable mask composition. Preserve that package and its PostgreSQL snapshot 603 as the preceding, owner-reported result.

Two causes were corrected without new images or hand-edited masks:

1. Positive emotions independently lifted cheeks around a protected mouth region after the registered/joint smile had already supplied cheek motion. Their extra cheek field is now zero; smile, brows and lids retain their separate prepared controls. The other seven field pages, field units and all oral geometry are byte-identical to the preceding candidate.
2. Joint lip textures followed source UV coordinates, while edit coverage was read from stationary destination pixels. Moving corners could be clipped or blended with the wrong ownership. The optional source-bound `deformed-oral-coverage/1` setting transports the existing red edit-coverage channel with the same UVs as soft tissue. The inner lip aperture and rigid canonical dental sampling remain separate. Historical packages without this setting retain their prior pixels.

The current local package is `.tools/emotion-masks-83/package-coverage/`, prepared in `prepared-coverage/`; its private profile is `active-profile/`. Only the emotion-field texture changes; all 118 existing texture hashes, source masks, joint triangles/deltas, foreground and canonical enamel remain unchanged. The registered resting smile is unchanged; smiling-speech coverage receives the same correction. Stable image/container names remain `syntelle-render-native:local` / `syntelle-render-native-local`; rollback is `syntelle-render-native:before-emotion-masks-83` plus the nonsecret settings in `activation.json`.

A focused regression reproduces clipping with a transported source-owned strip and verifies that corrected coverage follows it without leaking outside that ownership. Two processor checks verify single cheek ownership, unchanged other moods/materials and optional metadata. Two bounded recorded-frame checks cover the three moods at 50/100 percent, rest/open speech and focused MBP/FV combinations; all 76 CPU/CUDA frame pairs match exactly, including same-frame fallback. The native build checks pass. An isolated PostgreSQL fixture replays actual cached speech for the three baselines through CUDA/NVENC: no model/recognition adapter calls, three cached-speech deliveries and no player errors. The live installed visual-only check uses gesture entry with provider routes blocked. No paid provider request, generation, analysis, new model or schema change is involved in this correction.

Before, cheek-only candidate, corrected frames, clips and input/source snapshots remain in `.tools/emotion-masks-83/`. Use its generic `archive-final.py` and the external `.tools/emotion-masks-83-archive.json` for hash-verified PostgreSQL restoration. This is a composition correction, not owner acceptance of every mask or a new qualification of mood readability, cadence or arbitrary uploads.


## Source-skin correction (phase 84)

The owner supplied two screenshots showing that phase 83 still had a visible mouth/cheek mask. Those screenshots are preserved in `.tools/emotion-skin-84/owner-*.png`; phase 83 remains rejected for this artifact, rather than being treated as accepted because its geometry and GPU checks passed.

The remaining ring came from `prepare_skin_frames`: it mixed registered reference skin into an elliptical region while excluding the rectangular oral patch. Reference appearance and original skin therefore met around the mouth even after the moving-coverage correction. The new `source-skin-deformation/1` preparation removes that appearance blend. All 25 joint-smile skin frames now sample the original prepared foreground through the same reconstructed displacement field. Existing registered landmarks reconstruct the field without inference; every stored oral rest/open delta must agree within 1e-7 before export. No mask is painted, expanded or blurred to conceal the seam. Original references and the preceding 25 frames remain preserved.

Current resources are `.tools/emotion-skin-84/prepared-v2/`, `package-final/` and the package-bound `active-profile/`. The stable renderer image and binary are unchanged; only private resource selection changes. The neutral skin frame, registered manual resting smile, emotion fields, full-face reference laughter, lip geometry/contact constraints, opaque canonical enamel, head geometry and timing remain unchanged. Joint smiling speech uses the same motion with source skin instead of reference skin.

Reproduce offline from `services/processor` using fresh directories:

```powershell
.venv/Scripts/python.exe -m app.workflows.motion.prepare_expression_skin --bundle ../../.tools/emotion-masks-83/prepared-coverage/resource-bundle.json --landmarks ../../.tools/rgbd-motion-18/prepared-current/syntelle/inputs/landmarks.json --output ../../.tools/emotion-skin-84/reproduction
.venv/Scripts/python.exe -m app.workflows.rendering.export_native --bundle ../../.tools/emotion-skin-84/reproduction/resource-bundle.json --output ../../.tools/emotion-skin-84/reproduction-package
```

Five focused processor checks pass. Seven fixed neutral/manual/registered-smile comparisons are pixel-exact. The unchanged native binary renders 19 bounded states at two sizes with 38 exact CPU/CUDA comparisons and same-frame fallback; these cover the three positive moods at 50/100 percent, open speech, MBP and FV. An isolated PostgreSQL fixture replays three cached speech deliveries through CUDA/NVENC, with zero model/recognition calls and no browser errors. The installed player is checked through microphone-free gesture entry with provider routes blocked. These checks do not newly qualify cadence, emotion distinction, physical devices or owner visual acceptance.

The source-only comparison removes the distinct reference-skin ring in internal inspection. Owner acceptance remains pending. Preserve the failed first float-map preparation and first wrong-root fixture attempt as diagnostics. There are zero paid provider requests, image generations, new model installations or schema changes. Complete preparation, original screenshots, before/after frames, sources and rollback configuration use the generic PostgreSQL archive and hash-verified restoration in `.tools/emotion-skin-84-archive.json`. Roll back the three nonsecret settings from `activation.json` to phase 83; its binary already matches the current stable image. Do not run a historical finalizer.


## Remaining-emotion mask review and corner correction (phase 85)

The owner requested a review of the other seven states at rest and during speech. The phase-84 resource package remains installed unchanged: `.tools/emotion-skin-84/package-final/` and its private profile. The native renderer now additionally transports emotional lip geometry correctly when an emotion has no smile.

The bounded review covers neutral, curious, sad, angry, fearful, surprised and disgusted. Fixed states use 50/100 percent (neutral weights stay zero) with Rest, open A, closed MBP and FV: 56 frames before and after correction. The original mask ring was not observed in these states. However, sad at 100 percent exposed a detached FV mouth-corner fragment. This was retained as a failed result, rather than declaring all masks clean.

`oralShadeState.joint` previously recognized only a nonzero smile. Negative emotional lip deltas moved the mesh but still took the neutral endpoint UV shortcut and stationary edit-coverage path. Emotional lip deltas now also select the existing deformed UV/coverage path, even with zero smile. No triangles, amplitude, control mapping, source images, masks, lip/contact constraints or opaque tooth materials change. CPU and CUDA use the same prepared shading-state flags; no shader/ABI change is needed. Packages without emotional geometry retain their historical behavior.

The focused source-owned-strip regression fails at source/middle/target with the preceding renderer and passes with the correction. Native build checks pass. There are 38 exact CPU/CUDA comparisons across 19 selected states at two frame sizes, including same-frame fallback. Forty-two historical frames remain pixel-exact: the preceding 26 positive/manual cases and 16 neutral/curious cases. Existing source materials and the phase-84 resource manifest are unchanged.

An isolated PostgreSQL fixture plays 13 actual cached replies through CUDA/NVENC: neutral once and the six other states at 50/100 percent. Review retains 13 resting screenshots and five speech samples per reply, plus the browser recording. All model/recognition counts remain zero; 13 synthetic cached-speech adapter deliveries are not provider synthesis requests. Frame strips show actual articulation changes. No visible skin ring, detached corner or eye/brow mask boundary was observed in the reviewed corrected samples. This is an internal, bounded mask review, not owner approval or qualification of each mood's distinct readability, every possible articulation trajectory, cadence or physical-device behavior.

Complete frames, comparison strips, failed pre-correction regression, sources, recorded speech/player checks and private resource bytes are retained under `.tools/emotion-mask-review-85/` with generic PostgreSQL archival/restoration in `.tools/emotion-mask-review-85-archive.json`. The verified image is activated as `syntelle-render-native:local`, container `syntelle-render-native-local`; rollback image is `syntelle-render-native:before-emotion-masks-85`. Normal startup remains `./dev start`. There are zero paid provider calls, new generations, model installations, schema or cloud changes. Do not run a historical finalizer.


## Optional deterministic regulation (phase 86)

`Emotion stability` is enabled by default, with Calm, Warm and Expressive temperaments. All three support all ten states. Missing `emotion_regulation` settings normalize to `{enabled:true,temperament:"calm"}`; explicit false survives JSON, local preferences and saved configurations. The controls are available beside the baseline; temperament is disabled while stability is off. Off restores the original direct director and its 900 ms return. The automatic-expression switch remains independent.

Definitions are application-owned in `render-contract` under `emotion-regulation/1`. Role text and chat can influence proposals, never definitions. The controller guarantees limits, not correct semantic interpretation or immunity to prompt injection.

| Temperament | Intensity multiplier | Minimum state duration | Recovery |
| --- | ---: | ---: | ---: |
| Calm | 0.60 | 6 s | 4 s |
| Warm | 0.80 | 4.5 s | 6 s |
| Expressive | 1.00 | 3 s | 8 s |

Automatic ceilings before multiplication are neutral 0, joyful .65, playful .55, affectionate .45, curious .50, sad .45, angry .30, fearful .35, surprised .50 and disgusted .30. Proposals below .10 after capping/scaling are ignored; energy is bounded to .35-.65. Manual baseline values retain their full 0-1 range and existing prepared composition limits.

The per-session machine has baseline, reaction, bridge, holding and recovery phases. The first contextual state change is immediate; later changes require minimum duration and a maximum of two changes in a rolling 20-second active-clock interval. Positive (joyful/playful/affectionate) versus negative (sad/angry/fearful/disgusted) changes require two consecutive played proposals for the same destination within ten seconds, then two 500 ms transitions through neutral. Confirmation is cleared by contradictory proposals and interruption. Rejected proposals never activate on a timer.

Same-state changes smaller than .08 are ignored; accepted changes use a duration derived from the actual quintic easing derivative, bounding intensity speed to .15/second. A valid continuation can sustain a held reaction. Completion/interruption holds the last accepted reaction for two seconds and then recovers gradually. A new reply samples the existing transition and retains timing/budget history. Pause freezes the shared clock; reduced motion presents baseline immediately. Manual baseline/temperament/mode edits return over 900 ms, discard confirmation and suppress remaining targets for the reply without interrupting audio. No random mood generator or new provider request exists.

The native media session owns one value-copyable controller. It stages each audio block's decisions and commits only after successful PCM transmission with matching cancellation/configuration epochs. The video owner receives an immutable transition snapshot and samples it continuously; it never runs a second regulator. Automatic smile/laughter cannot contradict a held negative expression; manual controls and previews retain their paths. Prepared geometry, images, mask corrections and Ava synthesis are unchanged.

Private `emotion_decision` events distinguish raw proposals and effective targets with reason, phase, policy version, application-issued configuration revision, worker instance, sequence, turn, phrase and generation. Only committed effective transitions update model context. Recovery completion reports baseline; rejected/stale/unplayed proposals stay out of experienced context. A new private worker initializes at baseline; retired instances, old configuration epochs and duplicate sequences are rejected. Browser JSON cannot populate the in-process private feedback pointers, and events are not forwarded to the player. PostgreSQL evidence uses existing JSON operation records with instance/conversation-bound idempotency keys; no schema migration is introduced.

Focused controller/media/adapter and API race checks cover all profiles, ceilings, adversarial alternation, thresholds, duration, budget, opposition, recovery, slew, positive-cue gating, manual priority, failed sends, cancellation and settings epochs. Shared JSON tests cover default migration and explicit disable. Four Chromium-desktop/WebKit-mobile fixture checks cover controls, localization and persistence. Typecheck and the public production boundary build pass. The unchanged prepared resources pass 38 byte-exact CPU/CUDA comparisons and same-frame fallback, including MBP/FV/open vowels.

Actual CUDA/NVENC player checks use a deterministic director and cached audio in an isolated PostgreSQL database. They verify a .9 joy proposal becomes .39 under Calm, fast negative requests are rejected, recovery returns baseline, direct mode admits the previous .9 target, mid-speech edits preserve playback and record suppression, and interruption does not admit queued moods. Screenshots/video, context strings, complete decision evidence and failed harness checks are preserved in `.tools/emotion-regulation-86/`. Synthetic fixture adapter counts are not paid provider calls.

The stable local image/container remain `syntelle-render-native:local` / `syntelle-render-native-local`; resources/profile remain phase 84, with phase-85 mask rendering preserved. `syntelle-render-native:before-emotion-regulation-86` preserves the preceding native binary. Previous stopped API/web containers referenced image IDs that were unavailable before activation; complete before-source copies and the preceding PostgreSQL snapshot 606 preserve their source rollback instead of substituting unrelated historical images. New files introduced in phase 86 must be excluded when restoring that prior source tree. No credentials are included in evidence.

Use the generic PostgreSQL archive and verified restoration in `.tools/emotion-regulation-86-archive.json`, not a historical finalizer. There are no paid calls, image generations, model installations, schema changes or cloud deployments. Owner visual acceptance, distinct emotion readability, broad cadence and physical-device/public-upload qualification remain pending.

## Negative-expression readability calibration (phase 87)

The owner's screenshots identified weak anger and fear, central lip bowing in sadness, and similar anger/disgust expressions. The current local candidate is `.tools/emotion-readability-87/prepared-v3/` and `package-v3/`, with its package-bound `active-profile/`. Earlier candidates and the complete phase-84 package remain preserved. This is an owner-review candidate; distinct emotion readability is still unqualified, especially fearful and disgusted.

Cached MediaPipe brow geometry now separates converging/lowered inner brows for anger, raised inner worried brows for fear, and a gentler asymmetric brow/nasal lift for disgust. Smooth bounded fields retain the previous atlas units and seven unaffected body-field pages exactly. Sadness keeps its preceding brow field while moving less of the medial lip and concentrating its down-turn toward the corners. Fear adds horizontal lip tension; disgust uses an upper lateral lip lift rather than anger's downward action. Thin-triangle validation automatically reduces these actions; a requested amplitude is not a claim of visually equivalent output.

Optional source-bound `eyelid_tension` and `rest_opening` arrays distinguish anger's narrowed lids from disgust, and give fearful a slightly more open resting mouth. Both loaders enforce ten entries, neutral zero and finite values inside 0–0.4. Packages without these extensions retain the earlier constants. Speaking articulation owns the mouth while audio plays. No hidden iris is invented or exposed, and original eye materials, opaque canonical teeth, MBP/FV central contacts, source-only smile skin, transported oral coverage, head geometry, emotion regulation and Ava synthesis remain unchanged. No new masks or reference images are introduced.

Reproduce offline from `services/processor`, choosing fresh output paths:

```powershell
.venv/Scripts/python.exe -m app.workflows.motion.prepare_emotions --bundle ../../.tools/emotion-skin-84/prepared-v2/resource-bundle.json --landmarks ../../.tools/rgbd-motion-18/prepared-current/syntelle/inputs/landmarks.json --output ../../.tools/emotion-readability-87/reproduction
.venv/Scripts/python.exe -m app.workflows.rendering.export_native --bundle ../../.tools/emotion-readability-87/reproduction/resource-bundle.json --output ../../.tools/emotion-readability-87/reproduction-package
```

Nine focused processor tests pass, together with native calibration/legacy checks and the native image's existing bounded suite. All ten field states, 45 field transition midpoints and 81 lip pairs retain their geometry gates. The corrected fixed-state inspection includes actual MBP and FV endpoints, plus Rest/A; its 38 CPU/CUDA comparisons are byte-exact with same-frame fallback. The preliminary endpoint fixture accidentally sampled Rest for MBP/FV; those results remain diagnostic only. An isolated PostgreSQL CUDA/NVENC player check completes eight cached deliveries across the four requested moods at 50/100 percent, with zero model/recognition/provider calls. Samples include playback completion; they are not all spoken frames. Main-player microphone-free previews also pass. No new cheek/mouth mask seam was observed in these bounded samples. Fear and disgust remain subtle, and numerical checks do not qualify emotional interpretation, owner acceptance, cadence or physical devices.

The owner also reported live speech failures. Read-only operation evidence shows two `Canceled/ConnectionFailure` outcomes after an earlier successful synthesis. The running API's Docker host resolver rejected Azure Speech names; explicit resolvers returned valid addresses and a verified TLS connection. Local Compose now configures external DNS for the API only; internal service names still use Docker's embedded resolver. The installed API resolves both Canada Central TTS and recognition names and completes certificate-verified TLS without an authenticated synthesis call. Real provider synthesis was not retried or newly tested, and failed operations remain immutable. No credential, voice, region, budget or retry policy changes were made.

Stable runtime names remain `syntelle-render-native:local` / `syntelle-render-native-local`. Rollback uses `syntelle-render-native:before-emotion-readability-87`, the three nonsecret settings in `activation.json`, and `before-source/`; retain the DNS correction independently when rolling back only expression calibration. Use the generic archive/restore workflow recorded in `.tools/emotion-readability-87-archive.json`. Preserve the failed build caused by a missing private test fixture, the initial sadness test assumption, early visual candidates, and all source/input bytes. There are zero paid requests, image generations, new model installations, schema migrations or cloud deployments in this change.


## Disgust lip correction (phase 93)

The latest source-bound disgust correction is documented in [Disgust readability and lip correction](disgust-lip-correction.md). It supersedes phase 87 only for disgust, removes independent lip deformation and preserves nine other states. Owner visual acceptance and distinct emotion readability remain pending.
