# Conversational parallax and laughter recovery

This local candidate extends the shared preparation and native rendering pipeline. Only Syntelle was prepared, generated and visually reviewed. No Café/Woodland processing or arbitrary-upload qualification is implied. The normal local demo now selects the movement/oral-only `package-v2`. Expanded playback is withheld while a qualified left reference is missing; the correction below supersedes earlier activation and coverage claims.

## Shared pipeline

`app.workflows.motion.prepare_character_performance` accepts a source-bound input manifest, an existing `character-resource-bundle/1`, pinned local models and optional cached reference requests. It assembles conversational movement, oral recovery, registered side support and a native package. It never requests a provider. Reference acquisition is the separate `prepare_parallax_reference` command.

Inputs and algorithms contain no character-name branches, manual masks or character-specific placement overrides. Existing oral, eye, foreground and expression resources remain canonical. Source hashes, achieved reference pose, UV maps, geometry, model bindings and preparation-source hashes accompany the output. Invalid bindings, unsupported views and unsafe geometry stop preparation rather than silently installing a fallback.

The optional `registered-multiview/1` extension registers reference appearance into shared source UVs, supplies confidence/visibility and admits additional occupied grid cells. Reference foregrounds reuse the existing pinned semantic/native-hair matting pipeline and cached models. Mapping folds and their sampling footprints are excluded automatically and retained diagnostically. Photometric confidence rejects inconsistent material inside the existing opaque source. Both the preparer and native loader protect complete existing eye/oral, expression and laughter domains. Generated iris/enamel pixels cannot contribute to active materials.

Expanded occupancy must not replace the source matte's boundary classification: source uncertainty still requires the premultiplied boundary pass. A common chin-relative C2 neck taper attaches the whole head, including newly supported hair cells, to one rotation frame. Earlier preparations with rectangular missing edge cells, detached hair and inconsistent ear support were rejected and remain preserved. The final candidate is `final-preparation-v7`; v4's numeric and playback passes did not establish visual acceptance.

Depth remains a conditioned single-view estimate. Reference pose is a relative MediaPipe estimate, not a measured camera angle. This is not multiview stereo reconstruction, a measured anatomical skull or wide-angle hidden-surface reconstruction.

## Movement and dental behavior

The optional `conversational-motion/1` profile uses irregular 1.8–3.6-second speaking posture changes with 0.35–1.0-second holds. Diagonal posture, existing gaze and semantic gestures share the qualified angular envelope. Entry/exit blends preserve the preceding idle behavior; head motion still uses rotation and perspective, without a lateral-position animation parameter.

Private application-owned reply timing carries gestures across adjacent phrase boundaries. Acoustic opportunities occur every 2.8–5 seconds. API resolution, reply admission and native admission share an eight-cycle rolling 20-second allowance, 2.2-second minimum spacing and non-overlap. Repeated sequences reserve every cycle conservatively through their complete interval. Semantic cues take priority over acoustic proposals within prepared material. Long gaps discard expired phrase tails. Nod/shake speeds remain 420/580 ms. Older packages and legacy protocols retain their prior behavior.

The optional `oral-transition/1` companion joins the existing calibrated laughter and normal articulation placements. It derives the resting placements from each source's existing canonical rig; it does not impose Syntelle's offsets on other characters or infer hidden teeth from closed lips. Upper/lower materials remain opaque and rigid. Lip coverage reveals them geometrically, opposing-row overlap is checked and contact illumination follows each local aperture column.

Interruption cancels PCM immediately. An existing visible laugh can settle over 240 ms on the pause-aware animation clock; new speech retains articulation priority. Full laughter and the original manual resting smile remain exact at frontal endpoints. Existing speech registrations, MBP/FV contacts, emotion regulation and immutable dental material bytes are preserved.

## Reference allowance and results

The separate PostgreSQL identity is `authorized-conversational-parallax-2026-10-26`: at most six image attempts, zero remote analyses, no retries and stop on provider error. It does not reopen any historical allowance. Input records and reservations commit before paid calls.

Two image requests were made. The first remained approximately frontal and is rejected as a turn reference; its raw result is preserved. The second has an estimated relative yaw of approximately 13.4 degrees and supplies bounded side support. The pitch and two optional laughter-recovery slots remain unused. All review speech/laughter reuses cached material; no synthesis or conversation-model provider request is needed.

The original axis sweep passes 6-degree yaw, fails the unchanged area threshold at 8 degrees and inverts by 10 degrees. Stronger common depth-slope conditioning, registered support and revalidation admit the combined ±8-degree yaw / ±4-degree pitch / ±1.2-degree roll box. The area threshold is not reduced. Larger failed attempts remain evidence; 12-degree reference targets are not runtime promises.

## Offline reproduction

From `services/processor`, with the local preparation PostgreSQL profile available:

```powershell
.venv/Scripts/python.exe -m app.workflows.motion.prepare_character_performance `
  --config ../../docs/characters/conversational-parallax-inputs.json `
  --models ../../.tools/models `
  --references ../../.tools/conversational-parallax-96/references `
  --output ../../.tools/conversational-parallax-96/replay-new-directory
```

Use a fresh output directory. Omitting `--references` prepares the shared movement/oral companion with the existing envelope. Do not rerun paid image slots. Cached requests must retain their original source, settings and response hashes; failed or mismatched requests cannot be retried.

The candidate package is `final-preparation-v7/package/` under the experiment directory, hash `2e38d2e15e697f1fdd68909c760a69ba334b78e43e439e7844c47307d9d052b3`. `package-v2` is the movement/oral-only comparison at the preceding envelope. The source-bound fixture profile adds `motion_policy: conversational-motion/1`; ordinary profiles without it retain previous API admission behavior. The model direction schema, public commands and deployment remain unchanged.

The new optional resource fields have matching exporter, loader, CPU and CUDA handling. The private `render-frame` tool supports packed transport frames and both existing portrait heights for offline checks. No client character renderer is reintroduced.

## Evidence and remaining acceptance

The actual preceding runtime image, mounted package, profile, binary and source hashes were frozen in PostgreSQL snapshot 648. The running image differed from the historical phase-95 tag; both provenance and the actual runtime rollback tag are preserved. No owner-local image, package selection or container is promoted by this experiment.

Evidence includes failed preparations, the rejected frontal reference, UV diagnostics, 25 dental-coverage states, frontal endpoint comparisons, CPU/CUDA transport comparisons, deterministic motion checks, cached native playback and isolated 30-second scheduling measurements. Browser recording and screenshots are separate from cadence measurement. Fixture databases are separate, disposable PostgreSQL instances with no external provider access.

Final focused results:

- All 119 preceding texture resources retain their hashes; one registered support texture is added. Six frontal endpoints (rest/full laugh at smile 0/50/100%) are byte exact.
- Fifty-one oral/yaw/eye frames plus eight combined boundary frames are byte exact between CPU and CUDA. The unchanged area gate passes 81 combined geometry configurations at ±8/±4/±1.2 degrees; the minimum area ratio is 0.321. Larger failed configurations remain recorded.
- Twelve processor/archive checks, API/shared-contract race suites, native animation/adapter/render race suites and focused media cancellation/pause checks pass. Short-phrase resolution, phrase tails, long replies, rolling replenishment and legacy eight-movement behavior have direct tests.
- The isolated candidate's 30.031-second player run presents 887 frames: 29.57 fps, 33.5 ms p95 gap and 83.4 ms maximum gap. The preceding 30-second baseline is 29.53 fps / 50 ms / 66.7 ms. A second baseline cadence run is 29.70 fps / 33.4 ms / 99.9 ms; its later screenshot stage was interrupted during fixture teardown, so that run is not a completed preview check. These samples do not establish sustained 30 fps.
- Bounded active worker samples are approximately 906 MiB / 35% of one CPU for the candidate versus 777 MiB / 39% for the baseline. GPU samples are whole-device observations with the ordinary runtime still idle, not isolated per-process GPU attribution. Timing and resource records retain their individual capture context.
- Matching `baseline-motion.webm`, `direction-motion.webm` and `candidate-motion.webm` contain the same cached eight-phrase reply followed by laughter. Separate clips cover smile 0/50/100%, cached Spanish laughter and a quarter-speed release/mouth crop. Interruption screenshots show the recovering frame and subsequent speech; PCM cancellation is additionally checked by the native media tests.

Internal frame review checks likeness, hair/neck continuity, mouth attachment, opposing rows and eye/lip coverage at the selected boundary poses. It is a bounded candidate review, not owner acceptance or a claim that all possible expressions/angles are qualified. Failed fixture startups, the language-switch capture repair and rejected preparations remain evidence.

The archive includes the complete input-file closure, immutable original/request bytes, cached speech, pinned model bytes, current/baseline source copies and every rejected output. PostgreSQL restoration streams a bounded number of blobs at once; checksum, safe-path and no-overwrite rules remain unchanged and are covered by isolated archive tests. No schema migration is introduced.

Use the generic PostgreSQL archive/restore workflow for this experiment, never an older motion finalizer. Final verification and restoration identifiers live in `.tools/conversational-parallax-96-archive.json`; detailed measurements live in the experiment's verification record. Internal numeric/visual checks do not establish owner acceptance, competitive parity, physical-device qualification or reliable public-upload preparation.

## Reference and review correction (phases 98–99)

The owner requested the candidate in the normal demo, then objected to the incomplete reference pair and neutral delivery in the clips. The normal demo now uses `.tools/conversational-parallax-96/package-v2`, hash `9618de2563dd72e51d4bbbaee79bf331738b30e4864ee7dbe7aebef791c4f32c`, with the conversational profile. It retains the movement/laughter changes at the preceding envelope. Expanded export now requires independently qualified left/right yaw, signed direction, cross-axis, framing and scale checks. Accepted range uses achieved pose minus the common safety margin. Pose-facing visibility excludes opposing-side support. The earlier expanded candidate and all rollback resources remain preserved.

The separate one-image budget `authorized-parallax-left-correction-2026-10-26` is exhausted: its result measured +17.91° instead of a negative left turn. The owner then authorized prompt refinement under `authorized-directional-reference-refinement-2026-10-26`, capped at six images and zero analyses, with no transport retries. Actual usage is three images: guided +16.02° (wrong direction), simplified Spanish −4.04° (insufficient rotation), and closer owner wording −0.05° (essentially frontal). All are rejected. Historical budgets were not reset; remaining refinement slots are unspent.

The current default `docs/characters/parallax-reference-policy-v4.json` uses only the immutable original and a short Spanish instruction, with natural gaze accompanying the head. The verbatim owner prompt/screenshot are preserved in `.tools/reference-prompt-99/owner-prompt.txt` and `owner-prompt-screenshot.png`. Revised prompts have immutable request records. `parallax-reference-policy-owner-simple.json` preserves the subsequently rejected wording; it does not replace the default. The current model has not reproduced the owner's successful example. Failed results are not installed as 12° references. Sixteen focused processor checks pass.

Earlier eight-phrase gesture clips reuse unrelated phonetic audio and contain no emotion targets; they establish transport/timing only. A separate isolated controlled review uses exact cached text, “Hello. My voice and facial movements share the same clock.”, and explicit curious/affectionate facial targets. Calm/Expressive checks pass without providers. This verifies existing facial direction, not live LLM interpretation or emotional TTS prosody; synthesis remains text-only. Preserve `.tools/reference-expression-98/`, including failed reviews and both successful captures.

Request inputs/results are committed to PostgreSQL by the acquisition workflow. Complete sources, rejected outputs and restoration verification use `.tools/reference-prompt-99-archive.json` and the generic archive, never historical finalizers. No public API, deployment, model installation, manual character mask or canonical eye/dental material changes are involved.


Exact English prompt test (owner-requested): one original-only image was received under the existing refinement allowance (now four of six images, zero analyses). Relative MediaPipe yaw is -10.10 degrees, pitch -0.98, roll -1.45; signed direction/framing gates pass. This is a preparation reference candidate, not owner identity approval or runtime promotion. Exact prompt/input/output and qualification are preserved in .tools/reference-prompt-99/attempts/english-exact-04/yaw-left/. PostgreSQL snapshot 675 restores all 12 files with matching hashes; receipt: .tools/reference-prompt-99-english-exact-archive.json. Earlier rejected attempts remain unchanged.

## Default English prompt and paired views (phase 100)

The owner selected the successful English prompt as the shared pipeline default and requested integration of both recorded lateral views. `app.preparation.reference_policy.DEFAULT_POLICY` now selects `docs/characters/parallax-reference-policy-v5.json`. Its left prompt is exactly:

> Turn the head 12 degrees toward the viewer’s left. Keep the image identical except for this change. Rotate the head itself, not just the eyes. Let the gaze follow the head turn naturally.

The right template changes only `left` to `right`. Each new acquisition still uses its own immutable original and explicitly authorized, policy-pinned manifest. The prompt is source-independent; it does not contain a character name or placement override. Earlier Spanish/guided prompts and requests remain unchanged.

The current local candidate is `.tools/paired-parallax-100/prepared-v2/package`, hash `eb1c1df5e15d773cab77da7f877f048447f3b2b1cffb6865524d69d645745da9`. It registers the accepted English left reference (−10.10°) and the previously cached right reference (+13.44°). Both contribute actual registered material: approximately 71,348 and 46,419 nonzero confidence pixels, respectively. Source eye/oral/expression regions remain excluded. The 119 preceding texture hashes are unchanged, and two registered textures are added.

The first paired preparation produced a faint duplicate hair boundary and is rejected in `prepared-v1`. Face landmarks do not establish hidden hair contours. The shared correction permits reference color only in the original matte's confident interior; the source retains one continuous silhouette. The accepted package therefore adds no new occupied surface cells. Its geometry remains a conditioned single-view estimate with registered appearance support, not hidden-surface or stereo reconstruction. No manual mask, character-specific offset, new inference model or provider call is introduced.

The two-sided coverage margin permits up to 8.10° symmetric yaw; the unchanged combined geometry gate accepts ±8° yaw / ±4° pitch / ±1.2° roll. The 10° and 12° attempts remain rejected, and the area threshold stays unchanged. Thirty-five fixed states match exactly between CPU and CUDA FP64, including rest, MBP, FV, open A, blink, smile, laughter and combined corners. Six frontal rest/laughter endpoints at manual smile 0/50/100% match the preceding package exactly. Three actual FP32 boundary/frontal samples differ from CPU by at most one channel value; FP32 byte identity is not claimed.

An isolated exact-text cached review exercises curious/affectionate targets, conversational movement, full-strength head previews and cached laughter recovery. A separate 30.034-second run, without recording, presents 890 frames at 29.65 fps, with 49.9 ms p95 and 66.7 ms maximum gaps. The fixture uses CUDA FP64; the normal 720-high local worker retains FP32. This is a bounded check, not sustained 30 fps qualification or competitive parity. Controlled facial targets do not validate live LLM interpretation or emotional voice prosody. The actual installed demo passes its live-video/head-preview/laughter-release check with zero paid calls.

The normal local demo now selects this candidate and its package-bound conversational profile. API/native binary images and frontend code are unchanged; only private package/profile selection changes. `.tools/paired-parallax-100/activation.json` preserves prior settings and image identities; `before-paired-parallax-100` image tags preserve rollback. This is owner-requested local testing, not production promotion or visual acceptance.

Current offline reproduction, from `services/processor`:

```powershell
.venv/Scripts/python.exe -m app.workflows.motion.prepare_character_performance `
  --config ../../docs/characters/conversational-parallax-paired-inputs.json `
  --models ../../.tools/models `
  --references ../../.tools/paired-parallax-100/references `
  --output ../../.tools/paired-parallax-100/replay-new-directory
```

This input manifest is explicitly `offline-cached-only` and cannot authorize acquisition. Integration uses zero new image, analysis, speech or conversational-model requests. Existing request allowances remain unchanged. Complete sources, input closure, cached receipts, rejected/accepted preparations, frames, videos and rollback records use the generic PostgreSQL archive with verified restoration in `.tools/paired-parallax-100-archive.json`. Owner review, physical devices and future-upload qualification remain pending.

## Full-laughter exit correction (phase 101)

The owner accepted the other movement changes but reported a startle at the end of the full audible laugh. The initial near-zero handoff montage was a diagnostic of only the last 2.5% of the reference expression, not a full-laughter review. It is retained without treating it as owner acceptance or sufficient reproduction.

Two behavioral regressions reproduce concrete exit defects. The expression controller had internally treated the reference laugh as ordinary speaking, halving its manual smile baseline while the visible laughing frame separately restored the full setting. When PCM ended, that hidden half-strength baseline became visible and recovered over the next frames. A 50% baseline fell to approximately 25%, and a 100% baseline fell to approximately 50%. Reference laughter now keeps the controller's ordinary resting baseline throughout its active cue. Regular speech still uses its existing half-strength smile rule.

The reference jaw also retained unscaled audio energy near the end. Once the renderer quantized the laugh to zero, that amount could be interpreted as an ordinary open-A articulation. Its audio-driven amount now releases with the laugh envelope. Full-laugh appearance at peak, original resting/manual smile, interruption's 240 ms recovery, pause behavior, speech precedence and legacy resources remain unchanged.

Before/after review now records the **complete cached audible laugh and return** in English and Spanish at manual smile 0/50/100%, with motion/blinking disabled only in the isolated review sessions. The twelve full clips, 36 decoded full-laugh/return samples, initial failed assertions, rejected selector check and focused race-test results remain in `.tools/laugh-exit-101/`. The two defects fail before correction; native animation/adapter suites pass after correction. A too-strict initial floating-point assertion was corrected to a 1e-6 smile tolerance; its approximately 6e-8 residual is unrelated to geometry gates or visible movement. The inherited renderer executable is byte-identical to phase 100.

The local worker now selects `syntelle-render-native:laugh-exit-101` through its stable alias. The package, private profile, two registered views, default English preparation prompt, API and frontend are unchanged. `before-laugh-exit-101` preserves the actual preceding runtime. An additional complete Spanish laugh is captured from the installed demo. No provider request, generation, model/dependency change, schema or cloud deployment is involved.

Complete preparation inputs/assets remain in verified parent snapshot 676; this native-only change freezes its full worker source, changed-source baseline, compiled binaries, cached-audio/profile inputs, before/after clips and diagnostics in a separate generic PostgreSQL archive. Its restoration receipt is `.tools/laugh-exit-101-archive.json`. Owner visual acceptance of the corrected audible-laugh exit remains separate.

## Laughter return with an active expression (phase 102)

The owner reported that phase 101 still failed when an expression such as Playful was selected. Its earlier browser review had selected Neutral and therefore did not cover this case. Constant-Playful fixed frames reproduce the discontinuity: the reference mouth/lid path approached its neutral endpoint while ordinary emotional articulation, joint-smile skin and eyelid tension appeared at full weight when the reference branch ended.

The shared animation now supplies a presentation bridge through that common neutral endpoint. During the closing portion of a reference laugh, facial emotion and smile presentation follow the existing normalized release envelope. After handoff, the selected facial expression returns with a 320 ms quintic easing. This includes a brief neutral facial transition; it is not a direct multiview reconstruction or interpolation between independently calibrated emotional rigs. The selected mood, controller target/evidence and profile remain unchanged. Full-laugh appearance is unchanged, and Neutral/manual-smile playback retains phase-101 behavior. Canonical teeth remain opaque; no texture-opacity interpolation is introduced.

Interrupted laughter keeps its 240 ms recovery and carries the current presentation weight. The oral recovery joins a compatible idle mouth target continuously. Pause freezes the bridge, and new speech retains immediate articulation priority. Reduced-motion behavior bypasses the additional presentation transition. No character-name condition, manual mask, new reference or material/geometry preparation is added.

New regression cases reproduce the prior abrupt handoff with Playful at 50%, Playful plus manual smile, and restrained Joyful laughter. They assert bounded facial change, return to the original selected expression, pause and new MBP-speech priority. Native animation/adapter race suites pass. Complete audible English/Spanish Playful clips at 50/100% are retained before and after, plus an actual installed Spanish/50% clip. The failed Spanish-label selector run is preserved separately; it made no provider calls.

The local worker now selects `syntelle-render-native:laugh-emotion-exit-102`; `before-laugh-emotion-exit-102` preserves phase 101. Package hash `eb1c1df5e15d773cab77da7f877f048447f3b2b1cffb6865524d69d645745da9`, both views, English preparation default, API/frontend and source resources stay unchanged. The renderer executable remains byte-identical; this is animation-only behavior. Generic PostgreSQL archival/restoration is recorded in `.tools/laugh-emotion-exit-102-archive.json`, with verified parent snapshots 676/678 supplying unchanged inputs and earlier sources. New paid requests are zero. Owner visual acceptance, other moods' visual review and physical devices remain pending.
