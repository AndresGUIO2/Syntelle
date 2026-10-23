# Syntelle demo implementation tasks

These tasks implement [the specification](spec.md) and follow [the technical plan](plan.md). Checkboxes represent verified work, not estimated progress.

The smaller [first local build](../../docs/development/first-build.md) tracks image-file validation separately. Completing that slice does not complete tasks that also require facial compatibility, cloud deployment, preparation, or conversation.

## Definition and planning

- [x] T001 Record the product, scope, confirmed limits, and deferred decisions.
- [x] T002 Preserve the original prompts and analyze the local example strictly as a reference.
- [x] T003 Compare providers with priority given to Google and produce a cost scenario with explicit assumptions.
- [ ] T004 Finalize supporting tools and services. Host Vue separately on Cloudflare, with Workers Static Assets recommended. Run the Go API and independent Python/FastAPI processor as two Azure Container Apps services with scale-to-zero. Evaluate Express against standard Consumption and verify WebSockets, memory, and preparation duration. Finalize sessions and origins, private panel access, service authentication, persistence, and durable dispatch compatible with Azure.

## Local foundation

- [x] T042 Implement and verify the first local Vue ÃƒÂ¢Ã¢â‚¬Â Ã¢â‚¬â„¢ Go ÃƒÂ¢Ã¢â‚¬Â Ã¢â‚¬â„¢ FastAPI image-file validation slice, with actual JPEG/PNG/WebP checks, the exact byte boundary, orientation-corrected dimensions, preview and error states, and both themes. Build and run Docker Compose with FastAPI internal, run frontend/Go/Python checks, and verify Chromium/WebKit desktop and mobile flows plus actual processor failure and restart. Evidence and remaining launch checks are in [the first-build notes](../../docs/development/first-build.md).

## Preparation and animation prototype

Local progress is recorded in the [A-viseme trial](../../docs/experiments/oral/viseme-trial.md). The first Gemini-generated A mouth layer passed alignment, lossless export, exact overlay reconstruction and outside-mask pixel preservation checks. Static review found a usable opening with some upscaling softness. The broader tasks below remain open until browser/server integration, the variant inventory, calibration, motion quality and owner acceptance are verified.

A development-only browser inspection view now plays the cached A overlay using direct cuts or crossfades, with manual blend and mouth magnification. Chromium/WebKit checks cover playback, layout stability, outside-crop pixel preservation, reduced motion and load failures. A 50% crossfade shows doubled lip contours; owner motion acceptance and production lip-sync remain pending.

The owner accepted the first playback and authorized a landmark-guided morph comparison. A 48-vertex WebGL patch now reduces doubled lip contours by deforming both cached textures before blending, with a fixed alpha mask and exact endpoint rendering. Intermediate pixel preservation, non-inverting geometry and unavailable-WebGL fallback are verified. Some interior stretching remains; the full variant inventory, owner acceptance of this morph and speech synchronization are still open.

Owner feedback on tooth movement led to a fixed cavity texture revealed by the moving lip contour. Reference upper-tooth pixels are checked across intermediate openings, without additional generation. This replaces interior stretching in the local A experiment; a complete jaw model and owner acceptance of the revised motion remain pending.

The [remaining-viseme batch and automatic dental-reference trial](../../docs/experiments/oral/viseme-batch.md) retain the original generations and the owner-rejected synthetic plate. Current offline preparation automatically ranks original dental visibility/separability and selects EI. It proposes masks, preserves the selected tooth pixels, derives lower-row offsets with a bite stop, and exposes source/mask review in the browser. FV and SZ remain static-only. This does not complete public preparation, general semantic segmentation or owner visual acceptance.

The owner-selected [Argon mask composition trial](../../docs/experiments/oral/argon-composition-trial.md) now runs in the local browser using existing generations. Deterministic teeth-over-tongue ownership retains full tongue evidence and protects visible EI tissue; no new provider calls or image generations occurred. Verification passed 94 processor tests, frontend type checking/build and 44 Chromium/WebKit desktop/mobile checks. Existing per-pose annotations, phonetic limitations and public preparation remain unfinished; no broader task below is marked complete from this single-character experiment.

The [dental motion correction](../../docs/experiments/oral/dental-motion-fix.md) replaces dental opacity interpolation with opaque enamel and geometric covers. At the owner's request, experimental phonetic candidates can now play when their meshes are valid: 70 directed transitions are enabled, with U/CHSHR geometry still blocked. All 94 processor checks, frontend type checking/build and 48 Chromium/WebKit desktop/mobile checks passed. SZ/FV quality metadata remains experimental; phonetic correctness and public preparation remain open.

The [jaw spacing calibration](../../docs/experiments/oral/dental-spacing-trial.md) now derives lower-row movement from both lip edges and uses a recorded scale-aware clearance. SZ's minimum opposing gap increases from one to five crop pixels without changing EI, canonical tooth textures, cleaned tissue or original generations. All 96 processor tests, frontend type checking/build and 48 browser checks passed. This remains a visually calibrated single-character trial, not validated automatic visitor preparation.

- [ ] T005 After T004, prepare the local Vue/Pinia, Go API, and Python/FastAPI foundation for separate static deployment on Cloudflare and two Azure Container Apps services. Retain the selected TypeScript/Vite and net/http choices and pinned dependencies; separate public configuration from secrets and configure the selected processor service authentication. Verify SPA routes, HTTPS API access, WSS connections, and cross-origin sessions.
- [ ] T006 Gather owned or authorized photographs for development and validation without importing protected example images.
- [ ] T007 Detect face count, landmarks, pose, and regions. Evaluate MediaPipe Web in a worker and validate on FastAPI before paid operations rather than trusting browser results alone. Calibrate acceptance rules and errors with the test set.
- [ ] T008 Configure Gemini image integration and run a small measured batch when credentials are available. Compare Nano Banana 2.1 and Lite for identity preservation, cost, and failures.
- [ ] T009 Generate variants from the same base, record parameters, and reject incompatible results.
- [ ] T010 Align variants and automatically extract transparent mouth and eye layers, checking simultaneous composition.
- [ ] T011 Pin a reproducible Depth Anything Small version and integrate it into Python/FastAPI. Load one copy per instance; start with one process and one concurrent inference. Compare the reference implementation and ONNX Runtime CPU for compatibility, duration, memory, cost, and quality. Measure startup and model loading; reuse the map in the browser.
- [ ] T012 Build the versioned manifest and resource package with consistent coordinates.
- [ ] T013 Test mesh and layer transitions, M/B/P closures, sustained vowels, and blinking. Select variants based on visible improvement.
- [ ] T014 Implement parallax and subtle motion with a 2D fallback, checking edges, jaw movement, and breathing.

## Speech and viseme trial

- [ ] T015 Select Azure Speech Neural voices compatible with visemes and compare quality using English and Spanish phrases, prioritizing English.
- [ ] T016 Integrate Azure Speech audio and viseme events from Go; normalize offsets and map identifiers to the character's poses. Measure time to first audio with sufficient animation data.
- [ ] T017 Verify the Go SDK and its native dependencies in the Azure Container Apps container, including startup, memory, resource cleanup, interruptions, and discarded canceled segments. Configure credentials only on the server when available; do not make paid calls before configuration.
- [ ] T018 Normalize audio and events by turn and segment and synchronize the mouth to the playback clock. Verify that MediaPipe's 52 coefficients are treated neither as audio visemes nor as a ready-made deformation rig.

## Complete demo flow

- [ ] T041 Apply the selected Syntelle graphite and soft violet palette from [Brand and interface colors](../../docs/product/brand.md), keeping Colors Studio as the company brand. Implement light and dark themes and verify readability of controls, status indicators, and errors on desktop and mobile. The final product logo is pending; use a typographic wordmark for mockups.
- [ ] T040 Prepare an original semi-realistic illustrated sample character using owned or authorized resources, following the broad style of the protected reference without copying its identity or assets. Replace the photorealistic human from the UI mockups. Distribute its versioned package with the static demo. Display it on entry alongside custom image upload. Verify conversation without per-visitor image preparation or creation quota consumption, using the same speech and listening quotas when switching characters.
- [ ] T019 Implement JPG/JPEG, PNG, and WebP uploads up to 5 MB per image, enforcing size in both browser and server before paid preparation. Add validation errors and job progress for initial preparation lasting up to ten minutes. Continue accepted jobs after tab closure and recover their progress or completed package in the same browser within retention, without duplicate paid work or creation consumption. Default to one automatic retry for transient failures within budget, preserving completed stages. Verify that execution does not rely on a request exceeding ingress limits.
- [ ] T020 Save and restore the character package and configuration using the selected local storage mechanism, evaluating IndexedDB, and handle incomplete or unavailable storage.
- [ ] T021 Add an editable prompt, English/Spanish selection, and a generic task without inventing company information.
- [ ] T022 Integrate Gemini responses with bounded context and output and progressive speech.
- [ ] T023 Integrate the microphone and progressive Google recognition, with echo cancellation and text fallback.
- [ ] T024 Implement local interruption, remote cancellation, late-segment rejection, and conversation context continuity.
- [ ] T025 Handle partial and final recognition results, silence, and stream renewal without duplicates or gaps.

## Quotas and costs

- [ ] T039 Implement the owner-only private Vue panel for inspecting and changing speech and listening quotas through Go. Protect administrative operations with server authentication and owner authorization, validate values, and persist versioned settings without redeployment. Apply saved quotas to subsequent operations, including active windows, without resetting consumption. Verify rejection of unauthorized calls, consistency across instances, and settings retention through scale-to-zero.
- [ ] T026 Implement anonymous identity and a rolling 24-hour window from first counted consumption as the default; distinguish local character storage from server quotas.
- [ ] T027 Enforce one character per visitor every 24 hours and an owner-configurable speech quota, initially 180 seconds of generated audio, without reversing consumption on interruption. Consume creation quota exactly once when the character becomes ready and release its reservation on failure; verify recovery without duplication. On speech exhaustion, block text and microphone input, close recognition and synthesis, and keep the local character. Verify Go enforces settings and the interface displays them.
- [ ] T028 Enforce an owner-configurable listening quota, initially 360 seconds of processed audio per visitor every 24 hours, with a counter separate from speech. On exhaustion, close recognition, disable the microphone, and reject further audio; permit text with spoken responses while speech quota remains. Verify configuration changes, accumulation across sessions, and that opening the page without sending audio consumes no listening quota.
- [ ] T029 Implement atomic spending reservations, idempotency, and reconciliation of costs and concurrent operations.
- [ ] T030 Configure the global budget and request, context, and retry limits. Test exhaustion and connection loss during active operations.
- [ ] T031 Delete server photographs and temporary resources 24 hours after each file is created, including failed or abandoned jobs. Verify cleanup preserves the local character, quotas, and operational settings. Keep secrets on the server and verify they are absent from responses, logs, and public files.

## Verification and delivery

### Local selected-dentition experiment (separate from public preparation)

- [x] Complete the actual EI/Argon tooth fragments, preserving prompts, request settings, raw output, registration and original material. Two HTTP attempts consumed: PNG rejection and one JPEG completion; working exports are lossless PNG/WebP.
- [x] Stage the versioned oral package, joint lip fitting, opaque shared dentition and color-only shadows. Preserve original portrait/visemes and the previous package for comparison.
- [x] Verify all nine endpoints and 72 directed transitions, exact endpoint agreement, outside-mask preservation, material opacity and controlled shading. Use the same compositor for exports and live frames; explicitly update the stage when the renderer mutates its canvas.
- [x] Record local processor and Chromium/WebKit desktop/mobile checks in `docs/experiments/oral/completed-dentition-trial.md`. Keep raw records and visual comparisons in `.tools/dentition-completion-04/`.
- [ ] Resolve the reported oral artifacts to visual acceptance. The earlier `.tools/oral-artifacts-05/` passed technical checks, but owner feedback and the semantic comparison still expose incorrect FV contact and lower-material seams. Preserve that history; see `docs/experiments/oral/semantic-oral-preparation.md` for the current candidate and remaining failures.
- [x] Analyze all nine cached poses with source-bound Argon semantic contours, preserving exact requests/responses and limiting the batch to eighteen attempts without transport retries. Nine requests used; no image generation.
- [x] Register five owner-supplied corpus portraits with three development and two held-out inputs; preserve bytes and local face-detection results. This records resources, not completed multi-character preparation.
- [ ] Complete multi-character semantic preparation and visual acceptance. The local FV source and material correction are documented in `docs/experiments/oral/oral-repair.md`; the rejected new lower completion was replaced by generic observed-material continuation, not a complete anatomical reconstruction.
- [x] Prepare nine-pose development candidates for cafe and woodland through the recorded generic pipeline; add bounded, measured FV lip-contact correction and three-character browser inspection. Preserve failed FV corrections and all request usage. This does not complete visual acceptance; see `docs/experiments/oral/automatic-oral-corpus.md`.
- [ ] Prepare and validate all five corpus characters with the frozen pipeline; record blue's failed face detection and do not claim held-out visual success from preflight alone.
- [x] Store local preparation evidence in the owner's selected PostgreSQL database: actual file bytes, immutable snapshots, prompts/configuration, masks, failures, source hashes and per-request usage. Verify real-file restoration and transactional request limits; keep credentials separate. See `docs/preparation/preparation-database.md`.
- [x] Correct local FV articulation and automatic incisal registration; preserve both new image requests, including the rejected lower completion, and verify the corrected single-character package without manual masks.
- [ ] Obtain owner motion/phonetic acceptance and real-iPhone verification. Public-upload teeth/tongue preparation remains incomplete.

### Local motion candidate (separate from public preparation)

- [x] Prepare one closed-eye pose per development character through the generic, source-bound command; preserve three paid requests in the separate six-image PostgreSQL budget.
- [x] Extract eye layers, transparent foregrounds and pinned CPU depth maps for Syntelle, cafe and woodland without manual character corrections.
- [x] Add geometric blinking, restrained depth movement, torso breathing, independent controls, reduced-motion/visibility handling and graphics cleanup in a selectable candidate.
- [x] Install the three owner-supplied backgrounds, thumbnails and persistent background-only blur. Verify that selection/blur never changes character pixels or calls a model.
- [x] Verify fixed exposed iris pixels, eye/mouth separation, exact endpoints, bounded geometry, graphics-context reuse, processor tests, frontend build and Chromium/WebKit desktop/mobile checks. See `docs/experiments/motion/motion-trial.md` for evidence and remaining limits.
- [ ] Obtain owner visual acceptance of the motion candidate and verify it on a real iPhone before launch.
- [ ] Reach the strict 60 fps desktop target for Syntelle on WebKit; the current local stress measurement is approximately 57 fps. Keep the candidate selectable without replacing the default trial.

### Foreground refinement and code organization

- [x] Replace binary foreground edge cleanup with pinned source-resolution alpha/color matting and automatic semantic trimaps for all three development characters; reject and preserve the translucent-braid candidate, then rerun the common interior-opacity correction.
- [x] Inspect native foregrounds against black, white, checkerboard and the three supplied backgrounds, with blur and staged browser motion; retain previous foregrounds and record internal review separately from owner approval.
- [x] Separate shared Vue character types/rendering/composables from experiment catalogs and views; preserve compatibility imports, explicit package contexts, three-context reuse and region-only uploads. Add scoped lint/format checks.
- [x] Fix and test Go graceful shutdown, active-request draining, timeout closure and startup failure without adding service layers.
- [x] Verify known-alpha matting fixtures, opaque source pixels, unchanged eye/depth resources, 147 unchanged rendering exports, processor tests and Chromium/WebKit desktop/mobile regressions. Archive and restore complete new evidence through PostgreSQL.
- [ ] Obtain owner acceptance of the refined foreground edges and complete real-device/public-upload validation. Internal development review does not satisfy these gates.

- [ ] T032 Measure preparation, first speech, interruption, synchronization, and rendering against plan targets.
- [ ] T033 Verify conversation and interruptions in English and Spanish, prioritizing English quality.
- [ ] T034 Verify desktop and Android coverage and required launch support for Safari on a real iPhone. Exercise sample and custom character journeys, text and microphone input, speech, interruptions, local storage, speakers, orientation, permissions, audio activation, and audio suspension. Record tested device and browser versions.
- [ ] T035 Test photograph acceptance and rejection, job recovery after tab closure, and local persistence with authorized material. Verify return before completion, return after completion, expired resources, and lost browser identity without duplicate generation or creation consumption.
- [ ] T036 Record cost per successful character and conversation, including failures and retries, and replace financial assumptions with measurements.
- [ ] T037 Verify limits, retention, and deployment: confirm actual scale-to-zero without traffic, startup duration, quota persistence after shutdown, and termination of paid operations when budget is unavailable.
- [ ] T038 Document execution, configuration, limitations, and test results. Keep tasks incomplete unless supported by evidence.

### Remaining hair-background contamination

- [x] Correct coarse foreground certainty in the automatic hair boundary; use pinned native tiled matting for all three characters without hand-edited masks.
- [x] Preserve rejected trials, previous foregrounds, model/source/checksum records; verify cache reuse, source-exact opaque RGB and unchanged eye/depth/oral resources.
- [x] Inspect native and browser compositions against black/white/checkerboard and supplied backgrounds, with blur and motion. Pass 151 processor tests and 40 Chromium/WebKit desktop/mobile motion scenarios; frontend checks pass.
- [ ] Obtain owner visual acceptance and complete the outstanding held-out/public-upload and real-device checks.

### Natural head motion and local Dragon HD speech

- [x] Prepare a shared optional anatomical rig from cached MediaPipe XYZ, semantic maps and depth for all three characters; validate the sampled envelope and byte-identical replay without new images.
- [x] Add seeded posture changes/holds and audio-energy gestures, preserving common eye/mouth deformation, torso-only breathing, persistent graphics resources and the previous movement comparison.
- [x] Verify Ava and Andrew Dragon HD through the pinned Go/native SDK in English and Spanish on the owner's personal Canada Central resource: four real requests, 257 characters, with native viseme IDs/offsets and actual WAV bytes committed to PostgreSQL.
- [x] Add bounded local speech playback, shared audio-clock articulation, pause/resume/stop, stale activation guards, visibility/reduced-motion behavior and cache reuse.
- [x] Correct and preserve the integration-test fixture ledger incident; isolate subsequent PostgreSQL tests and verify they leave development records unchanged.
- [x] Run processor/Go/frontend checks, 64 browser regressions, 12 Linux WebKit speech scenarios and 12 real-audio replay/layout cases. See `docs/experiments/motion/natural-head-speech.md` for scope and limitations.
- [ ] Reach the strict Syntelle desktop WebKit 60 fps target and obtain owner/real-iPhone acceptance before promoting the new movement by default. Preserve the selectable candidate and preceding demonstration.
