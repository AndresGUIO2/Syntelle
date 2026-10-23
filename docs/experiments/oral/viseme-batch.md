# Local multi-viseme preparation

This milestone expands the approved A trial with EI, O, U, MBP, SZ, FV and CHSHR. It remains an owner-operated local experiment, not public character preparation. The owner approved reuse of A's dentition and internal mask review before developing automatic separation for visitor uploads.

Current status: offline preparation now automatically selects a dental reference from the eight original cached poses; the current winner is EI. Original tooth pixels and automatically proposed masks are used in the browser review. The rejected synthetic plate stays excluded. FV and SZ remain static-only. See the final section for the current workflow; earlier experiments below are retained as history.

## Immutable generation and records

The batch used the original `E:/Carpetas/Documentos/01-MisProyectos/Colors/Syntelle.jpg` for every request, with SHA-256 `3939c6033bd4cb2cd06c6d3405f244ed470c74330bb8adc9de35b9b00c4579e9`. It did not send A or another generated pose as the next input. The approved A was reused without regeneration.

Seven sequential Gemini requests produced seven raw results. Each used `gemini-nano-banana-2.1`, 1K, portrait 3:4, a JPEG response, `store=false`, a 180-second HTTP timeout, no redirects and no automatic retries. Working resources and composed exports use lossless PNG/WebP. Generation still has the previously observed resolution limit: a 896 Ã— 1200 provider image is aligned onto the 1792 Ã— 2400 base.

Each directory under `.tools/visemes-01/` retains the exact final UTF-8 prompt, source-prompt hash, original-input hash, request settings, timestamps, provider usage/duration, original response image/hash, raw landmarks, alignment metrics, masks and uncorrected composition. Prompt files use the actual line endings sent to the provider; integrity checks compare their bytes to the recorded request-prompt hash. The supplied source prompt document remains unchanged.

The command accepts only the eight documented identifiers. The batch requests only the seven remaining groups, caches before composition, refuses to replace a missing or modified paid response silently, stops on provider failures, and preserves completed poses. Geometry failures retain the raw image for local correction. A later fresh API request cannot be assumed to reproduce identical output; reproducibility here means replaying preparation from the retained original provider bytes and recorded settings.

From `services/processor`, the generation command used was:

```powershell
uv run --frozen --extra preparation python -u -m app.viseme_batch --input "E:/Carpetas/Documentos/01-MisProyectos/Colors/Syntelle.jpg" --reference ../../.tools/viseme-a-05 --output ../../.tools/visemes-01 --generate
```

Do not run this in a fresh directory as an inspection step: `--generate` authorizes paid calls. The existing batch reuses cached raw images. A single pose can also be inspected/generated through `python -m app.viseme_trial --viseme EI`; the default remains A for compatibility.

## Reviewed oral layers

The versioned character-specific review is [viseme-review.json](../../characters/syntelle/viseme-review.json). It stores annotation coordinates, canonical upper/lower tooth polygons, feathering, lower-row color refinement, per-pose tooth removal, protected tongue regions, rigid lower-row offsets, review decisions and notes. These are preparation data, not coordinates embedded in the renderer.

- Upper teeth use the approved A texture at a fixed position and scale. Lower teeth reuse A's texture without scaling or mesh deformation, with recorded rigid offsets when required.
- Generated teeth are removed using reviewed masks and deterministic OpenCV Telea inpainting at radius 3 pixels, restricted to the mouth cavity and excluding protected tongue pixels. For narrow all-teeth outputs without dark neighbors, the recorded A cavity sample fills the reviewed removal area. The full dilated removal mask is used to avoid an inpainting rim.
- The lower reference mask is refined using recorded color ratios to exclude adjacent lip pixels. This sample-specific calibration is not an automatic separator for arbitrary photographs.
- The soft cavity and tongue remain specific to each pose. They use their own mesh coordinates, so an interpolated aperture does not expose a prior pose's lip boundary. Dentition is composited separately, behind the moving lips.
- Rest and MBP hide teeth. FV hides the lower reference row but still requires correct upper-tooth/lower-lip contact to be accepted. The A and Rest endpoints retain their approved pixels.

No distinct tongue tip was confidently identified in these seven outputs; current real-image tongue-protection masks are empty and that fact is recorded. Synthetic tests verify that protected tongue tissue survives tooth removal. This does not establish quality for a future dedicated tongue pose such as LDTN, whose prompt remains undefined.

Preparation records include software versions, algorithm settings, configuration hashes, source-run hashes and hashes of exported artifacts. Each subsequent build archives the reviewed configuration, manifest and explicit pipeline-source snapshots in `.tools/visemes-01/preparation-history/`, keyed by the configuration/source fingerprint. Credentials stay in the local ignored `.env`; no credential or private input path is included in browser assets.

## Replay and browser review

Rebuild local oral layers and meshes from cached generations, without contacting Gemini:

```powershell
uv run --frozen --extra preparation python -m app.prepare_viseme_package --review ../../docs/characters/syntelle/viseme-review.json --output ../../.tools/visemes-01/package
```

The output is `.tools/visemes-01/package/`: canonical tooth layers/masks, per-pose tooth-free surfaces, corrected endpoints, removal/protection masks, the review snapshot, contact sheet, and a versioned manifest. Runtime-only lossless assets and that manifest are copied into `apps/web/src/experiments/viseme-a/poses/`. The complete working evidence remains local.

Open [the development trial](http://localhost:5173/?trial=viseme-a). From/To select any prepared pose; playback, manual interpolation, magnification and crossfade comparison remain available. Internal acceptance is separate from owner motion approval.

The renderer uses a shared crop and 48-vertex meshes with fixed boundary anchors. It tries both diagonals around lip quads, checks continuous signed triangle area, and rejects meshes that invert or fail crop coverage. Teeth are independent of that mesh; the lower row receives only a rigid translation. Endpoint canvases and the original image remain unchanged outside the edit masks. Crossfading mixes complete cached crop compositions so different mask extents do not leave a previous pose behind.

## Observed quality and boundaries

EI, O, U, MBP and CHSHR passed internal endpoint review as local candidates for owner evaluation. Together with Rest and A, they provide 40 safe directed transitions. Uâ†”CHSHR fails mesh checks and remains static-only. FV is inspection-only: its raw generation shows both dental rows and fails the required labiodental contact; replacing teeth does not make its articulation correct. SZ is also inspection-only: the raw output reads as a sibilant, but substituting the limited A dentition leaves an unconvincing cavity/lower-tooth strip. Neither rejected pose nor an unsafe mesh enters animated playback.

The A reference exposes only limited dentition. Reuse cannot recover unseen side teeth. Source-resolution softness and some soft-tissue interpolation artifacts remain. Do not describe this as production lip-sync or automatic visitor preparation. Seven requests reported 21,976 total tokens and 93.603 seconds of combined provider request time; those figures are not a verified billing charge or total preparation duration.

No blink, LDTN, speech, depth, deployment or public-generation feature was added. The future automatic separator can produce the same reviewed-layer contract, but visitor images must not depend on these sample-specific masks or color thresholds. The agreed 24-hour retention policy still applies when public preparation is implemented; local owned development evidence is separate from visitor uploads.

## Verification completed

Verified on 2026-10-19:

- Python: 58 tests passed, including bounded requests, cache recovery, exact saved prompt bytes, identical original input, protected tongue pixels, mask confinement and accepted mesh coverage. One existing Starlette test-client deprecation warning remains.
- Frontend type checking and production build passed. Development trial images and controls are excluded from the production bundle.
- Browser trial: 36 checks passed across Chromium and WebKit, desktop and mobile viewports, with no retries or skipped checks. These cover endpoints, manual interpolation, playback, fixed upper-tooth samples, fallback behavior and static-only rejected pairs. SZâ†”FV and FVâ†”CHSHR were checked as blocked playback, because their endpoint review failed.
- The original JPEG hash, runtime manifest equality, archived pipeline-source hashes and absence of the configured Gemini key from browser source/build were verified.

These checks cover the local trial. Backend-dependent public upload integration was not rerun for this batch; Go was unchanged. Real-iPhone verification and owner acceptance of the new poses remain pending. Automated pixel checks cannot establish complete phonetic or visual quality.

## Complete dental reference update â€” 2026-10-19

The owner authorized one separate dental plate generation and local recomposition of the existing poses. Exactly one additional Gemini request produced `.tools/dentition-01/generated-dentition.jpg`; no viseme was regenerated. The request included the exact original JPEG bytes as image 1 and the unchanged approved A mouth crop as a clearly labeled supporting enamel/style reference. It did not use an intermediate pose as the portrait input.

The exact prompt is [dentition-v1.txt](../../prompts/dentition-v1.txt). Its UTF-8 byte hash is `a89092e8aaec7c7373cc24e85cdd069b7049d61c28ed54b83546a503f1d52099`. The saved `run.json` contains both input hashes, model, settings, timestamps, usage and raw-output hash. Settings remain `gemini-nano-banana-2.1`, 1K, 3:4 JPEG, `store=false`, 180 seconds and zero automatic retries. The request reported 4,857 total tokens and 14.505 seconds of provider time; billing was not measured. Credentials stay in `.env` and are not copied into any record or browser resource.

The generated plate has two separate enamel-only arches on a black matte. The complete visible frontal arches are a synthetic extension of the A reference, not recovered hidden anatomy or a full anatomical dental scan. [dentition-review.json](../../characters/syntelle/dentition-review.json) records reviewed row regions, black-matte thresholds, scale, center, cutting-edge positions, exposure gains and a smooth cutting-edge envelope that rectifies the generated arch perspective once. Canonical upper/lower textures are then fixed: runtime does not repeat this warp or resize the teeth. Upper teeth remain stationary; only the recorded rigid lower-row translation varies by pose. Some synthetic tooth proportions and limited source softness remain visible.

[viseme-dentition-review.json](../../characters/syntelle/viseme-dentition-review.json) is the rejected candidate configuration, now explicitly marked with the owner decision. Original A soft tissue is retained, but its composed teeth are now replaced too; the original approved A raw image and earlier endpoint remain intact in their original directories. Rest and MBP hide both rows. Existing tongue/cavity tissue and removal masks are reused. No confidently identified real tongue region was added; empty tongue masks and that limitation remain explicit.

The rejected local output remains archived at `.tools/dentition-01/package-current/`, with lossless PNG/WebP rows, mask assets, composed endpoints, a contact sheet, source-edge profiles, calibration/provenance and artifact hashes. The previous limited-A package and both intermediate dental compositions remain available for inspection. Pipeline source, prompt, review configuration and manifest snapshots are archived under `.tools/dentition-01/preparation-history/`.

At restoration, the default command rebuilt the approved-A-reference package without a provider call. To replay it now, pass `--review ../../docs/characters/syntelle/viseme-review.json --output ../../.tools/visemes-01/package-restored` explicitly. The current default command below follows the automatic-selection workflow documented at the end of this document:

```powershell
uv run --frozen --extra preparation python -m app.prepare_viseme_package --install-trial
```

The generation entry point is `python -m app.dentition_trial --input "E:/Carpetas/Documentos/01-MisProyectos/Colors/Syntelle.jpg"`. It now reuses the cached result, verifies prompt/original/supporting-image hashes and refuses missing or modified paid output. `--generate` was used for the one authorized request; it is not a retry or regeneration switch.

SZ passed the initial internal composition review of this later-rejected candidate: the complete rows replace the thin lower-tooth strip and keep a small central gap. With Rest, A, EI, O, U, MBP, SZ and CHSHR accepted internally, that archived candidate offered 54 geometrically safe directed transitions. Uâ†”CHSHR remains blocked by mesh validation. FV remains inspection-only because its generated lip articulation lacks the required upper-tooth/lower-lip contact; adding teeth alone cannot correct that pose. The subsequent owner rejection overrides this internal acceptance; these candidate transitions are no longer in the active preview.

Verification for this update: 63 Python tests passed, including one-request/cache/failure guards, exact original plus supporting-reference bytes, deterministic calibration, shared row pixels and rigid lower-row placement. All 36 Chromium/WebKit desktop/mobile trial checks passed without retries, including EIâ†”SZ and SZâ†”A playback in addition to Aâ†”EI, Oâ†”U and MBPâ†”A. Frontend type checking and production build passed. Final runtime image bytes match the browser-tested package; canonical PNG/WebP pixels match exactly. Original input and all eight original viseme run hashes remain unchanged, source snapshots match their manifest, and the configured key is absent from browser resources and dental evidence. The existing Starlette test-client warning remains. Real-iPhone and public upload integration were not added to this experiment.

## Owner rejection and restoration — 2026-10-19

The owner rejected the dental appearance. Internal review had accepted the candidate too early: fixed coordinates and lossless exports establish consistency, not convincing character dentition. The isolated plate, one-time arch rectification and exposure adjustment produced flat, square, gray rows that do not fit this character. Both dental configuration files now record the owner rejection; preparation refuses that rejected review. The prompt, raw plate, all candidate compositions and source snapshots remain available for inspection. No new provider request was made.

The active preview is restored byte-for-byte from the earlier limited-A package, including its original A endpoint and 40 safe directed transitions. FV and SZ are again inspection-only. That baseline still has the documented missing-side-teeth limitation; restoration is not a claim that the original issue is solved. A future reference should be evaluated inside the character mouth with matching perspective, illumination and lip occlusion before accepting extracted canonical rows. No replacement generation is authorized by this rejection alone.

Restoration verification: 64 Python tests and all 36 Chromium/WebKit trial checks passed; frontend type checking and production build passed. Every installed runtime resource matches the earlier package bytes. A new guard test verifies that an owner-rejected dental review fails before preparation writes output. No paid request was issued during restoration.

## Automatic original-reference selection — 2026-10-19

The current pipeline compares all eight original cached provider images: A, EI, O, U, MBP, SZ, FV and CHSHR. It reads each original response file, checks its recorded SHA-256, replays the cached stable-anchor transform and uses that pose's original landmarks. It does not rank corrected compositions or the rejected synthetic plate. Missing/changed originals stop preparation without regeneration. No credentials, new service, image-generation request or dependency was introduced.

The selection implementation is `services/processor/app/preparation/dental_selection.py`; settings are [dental-selection.json](../../characters/syntelle/dental-selection.json), and composition uses [viseme-auto-review.json](../../characters/syntelle/viseme-auto-review.json). Ranking contains no preferred pose ID or character-specific pixel polygons. The utility score combines the narrower visible dental-row span, enamel area, row separation, row balance and a small capped sharpness term. Coverage, minimum utility, row separation, central gap and best-candidate margin are gates; a larger opening or a high raw score alone is insufficient. Ties or no eligible pose preserve inspection evidence and stop without a fallback or new paid work. Scores are not confidence probabilities or visual approval.

The current result is EI (utility 51.824), with A an eligible alternative (40.198). EI's proposed upper/lower spans cover approximately 42%/52% of its landmark mouth width, versus A's 33%/43%. These are mouth-width ratios, not estimates of anatomical completeness. EI has an approximately 11-pixel central gap that makes the rows separable. FV scores 55.406 before gates but fails separation/gap checks; SZ also fails those checks. O, U, MBP and CHSHR do not expose two confidently extractable rows under these settings. This selection agrees with internal inspection of the original crops; owner acceptance of the recomposed appearance remains pending.

### Automatic mask feasibility and limits

The implemented prototype proposes enamel seeds using color neutrality and brightness inside the landmark mouth aperture, grows them with deterministic OpenCV GrabCut, removes tiny components and searches for a dark valley between the rows. Seed thresholds, GrabCut iterations/seed, minimum component sizes and gates are recorded. A small recorded alpha feather is applied before export. This is an automatic proposal without manual tooth polygons, followed by internal visual inspection for this one character. Some shadowed side teeth are missed; touching rows are rejected instead of being split confidently.

MediaPipe supplies facial landmarks, blendshape scores and transformation matrices; its documented outputs do not include dental/tongue semantic masks. See the [official Face Landmarker task guide](https://developers.google.com/edge/mediapipe/solutions/vision/face_landmarker). Pixel-level dental separation is the additional local step here. General automatic masks are feasible as a preparation feature, but this prototype does not validate them for arbitrary photographs, unusual lighting, reflections, stylized teeth, braces, occlusions or tongue visibility. A broader labeled evaluation and likely a dedicated semantic separator are needed before removing review for visitor uploads. Synthetic checks show red tongue tissue and external white highlights are excluded in controlled cases; they do not establish real-tongue robustness. No clearly visible real tongue is present in this batch, and the existing pose-specific tongue protections remain empty and explicitly recorded.

### Composition, replay and review

Both canonical rows use the selected aligned original's pixels. No new tooth is invented, no dental plate is synthesized, and no extra tooth color correction, scaling or deformation is applied. Upper teeth remain fixed. Each pose's lower inner-lip landmark proposes an integer vertical offset relative to the selected pose. An automatic bite stop derived from the extracted upper/lower edges limits upward motion, preventing the lower row from crossing the fixed upper row during closure. Proposed/applied offsets and the stop are saved in the selection report and resolved review. Original mouth tissue, reviewed tooth-removal/tongue masks and lip occlusion are retained per pose. A now receives the selected reference too; its original raw image and earlier endpoint stay archived.

The first browser run exposed an upper-tooth pixel regression caused by excessive lower-row ascent. Its failure report, traces and manifest remain in `.tools/dental-selection-01/pre-bite-stop-failure/`. The bite stop fixes the composition rather than weakening the fixed-pixel assertion. The rejected synthetic plate remains blocked. FV/SZ remain static-only; U↔CHSHR remains blocked by mesh checks. There are still 40 safe directed transitions, with internal candidate status separate from owner approval.

From `services/processor`, inspect ranking/masks independently of the installed browser package:

```powershell
uv run --frozen --extra preparation python -m app.select_dental_reference
```

Prepare and install the development trial without any provider call:

```powershell
uv run --frozen --extra preparation python -m app.prepare_viseme_package --install-trial
```

Outputs are in `.tools/dental-selection-01/package-current/`: original aligned crops, per-candidate automatic masks, annotated contact sheet, full selection report, canonical rows, corrected endpoints and versioned manifest. Records include original/run/landmark hashes, settings, algorithm/software versions, all candidate metrics and exclusion reasons, row motion, artifact hashes and pipeline-source snapshots. `review.json` retains the input configuration; `resolved-review.json` records computed offsets. Earlier candidate packages and failed intermediate evidence remain separate. The browser receives only explicit runtime resources and compact selection metadata.

Expand **Dental reference: EI** in the [development trial](http://localhost:5173/?trial=viseme-a) to compare the original crop, mask proposal and every candidate decision. Playback, manual interpolation, magnification and crossfade remain available. The public upload workflow and production build do not expose this owner-operated experiment.

Verification completed for the current package: 73 Python tests passed, including pose-ID-independent ranking, greater dental visibility versus greater mouth opening, deterministic masks/ranking under candidate reordering, no eligible candidate/tie handling, synthetic tongue/highlight exclusions, touching-row rejection, corrupted/missing original rejection, actual original replay with networking blocked, no-fallback preparation evidence, original canonical pixels and bite-stop safety across accepted transitions. All 40 Chromium/WebKit desktop/mobile trial checks passed with zero retries or skipped checks, covering the expanded reference inspector, playback, manual morph/crossfade, fixed upper-tooth pixels during closure and A↔EI, exact endpoints, outside-mask preservation and blocked unsafe/review-only pairs. Frontend type checking and production build passed; the development trial remains excluded from production assets.

An independent standalone selection replay matched the installed package's winner, ranking, settings, candidate metrics, input hashes and crop. Original JPEG, all eight raw response/run/landmark records, selected-row PNG/WebP pixels, installed resources, artifact hashes and archived pipeline-source hashes were verified. The configured Gemini key is absent from browser resources and package evidence. The recorded bite stop is -7 pixels relative to the extracted EI lower row; it is derived, not embedded in rendering code. Existing Starlette test-client deprecation remains a warning. No public upload integration or real-iPhone acceptance is established by these local checks.

## Dental occlusion correction — 2026-10-19

The owner reported floating fragments in A to U. Inspection found a bright dental strip over U's lower-lip crease: both dental rows were enabled even though the original rounded pose exposes no separate teeth. MediaPipe's narrow inner-mouth polygon alone did not exclude that crease. The renderer also used the union of endpoint visibility flags, leaving teeth active until the static endpoint replaced the animated frame.

Review version `8-dental-occlusion` disables canonical rows for O and U, preserving their original cavity and highlights. A recorded one-pixel inner dental margin with antialiasing keeps reference pixels inside the lip border in static compositions and interpolation. Open-pose row visibility now changes continuously between the endpoint flags. Rest and MBP continue to hide teeth through geometric lip occlusion. Upper reference coordinates and original tooth textures remain fixed; lower-row motion still uses the recorded rigid translation and bite stop. The manifest records these rules as `oral-layers/4` and `pose-morph/2`, with source snapshots and export settings. The previous package is preserved at `.tools/dental-selection-01/pre-occlusion-fix/`; no raw generation, prompt or provider record was replaced, and no paid request was made.

Verification: 75 Python tests and all 44 Chromium/WebKit desktop/mobile trial checks passed, with zero retries or skips. New checks verify the inner dental margin, original toothless O/U endpoints and a near-U transition against transparent dental layers to detect a last-frame flash. The fixed-upper-pixel test samples enamel two pixels farther inside the opening because the previous sample lies on the newly occluded lip border. A to U was added to representative playback checks. Frontend type checking and production build passed. Manual browser inspection at 50%, 75% and the U endpoint confirmed removal of the bright strip; residual soft-tissue blending remains a visual quality limit. FV/SZ and unsafe pairs remain blocked, and public automatic preparation is still unvalidated.

The existing browser tab retained an older endpoint image during hot updates; reloading the trial loaded the corrected U endpoint. After replacing trial assets, reload before assessing static endpoints. Final artifact, source and archived-source hashes, immutable input/raw/run/landmark records, lossless row exports, installed runtime bytes and unchanged outside-mask pixels were verified against the current manifest.

## Incomplete automatic masks and reviewed contours — 2026-10-19

The owner identified missing side teeth and ragged lower enamel in the automatic EI proposal, plus poor SZ composition. The earlier internal inspection was insufficient. Color/brightness seeds and GrabCut do not establish semantic dental boundaries; this remains an unresolved automatic preparation problem. The original automatic proposals and scores remain in `selection/` and `dental-proposal.png`/`.webp`, while the pre-correction package is archived at `.tools/dental-selection-01/pre-reviewed-masks/`.

For this character only, [dental-masks-reviewed.json](../../characters/syntelle/dental-masks-reviewed.json) records assistant-reviewed upper/lower contours, exact original/base hashes, aligned crop coordinates, annotation version, reviewer and supersampled rasterization. The pipeline verifies those bindings and refuses another source, crop, overlapping rows or an unavailable review. It retains the original enamel colors/textures, including identifiable shadowed side teeth; it does not reconstruct hidden anatomy. The active `dental-mask` preview explicitly labels reviewed contours. Source selection is still automatic across the eight originals; active extraction is now reviewed and must not be described as automatic visitor segmentation or owner-approved appearance. Review version is `9-reviewed-ei-contours`; preparation is `oral-layers/5`.

The bite stop now measures opposing upper/lower edges per shared image column. A global upper-bottom/lower-top comparison incorrectly paired curved side teeth with unrelated front teeth, restricting closure too early as coverage grew. The recorded column profile derives the new -13 px stop; lower motion remains a rigid translation and every opposing column retains its configured minimum gap. Upper coordinates stay fixed. Browser asset URLs include the manifest build ID to avoid mixing cached old frames with new annotations during local preparation updates.

`Review only` means a pose is excluded from animation while its static composition remains inspectable. SZ still needs visual/articulation acceptance; FV still lacks the required upper-tooth/lower-lip contact. Neither was promoted by better extraction or technical checks. The candidate-ranking table has a different purpose: eligibility as a dental reference. A closed MBP pose naturally exposes no teeth and is not an invalid pose merely because its reference score is zero.

Verification: 77 Python checks passed, including reviewed-source/crop binding and column-wise bite-stop checks. Frontend type checking and production build passed. The first browser run retained 40 passing checks but exposed four assertions sampling the new antialiased tooth edge at y=82. The report, traces and manifest are archived at `.tools/dental-selection-01/reviewed-contours-alpha-sample-failure/`. The assertion now explicitly requires fully opaque enamel at y=80 before checking exact fixed RGB across closure; all four targeted browser checks passed. This correction does not establish visual approval. No provider request, new model or public endpoint was added; original raw generations and prompts remain unchanged.

The final full browser run passed all 44 Chromium/WebKit desktop/mobile checks without retries or skips. Final source/archive/artifact hashes, exact original generation/landmark/run records, lossless exports, installed runtime bytes, 40 gated directed transitions and outside-mask pixel preservation were verified. Manual browser inspection confirmed the reviewed-contour labels and improved EI side/lower boundaries. FV/SZ remain static-only; the remaining composition and phonetic limitations require further visual work rather than another claim of completion.

## Gemini contour comparison — 2026-10-19

The owner subsequently authorized a vision-only EI contour experiment. [Gemini contour trial](gemini-contours-trial.md) records two HTTP configuration rejections and one accepted text-analysis response, with zero new viseme generations. The returned upper polygon crosses itself and omits visible enamel; it remains rejected. The saved comparison visualizes the original returned coordinates without manual repair. This experiment did not replace the active reviewed contours or alter runtime assets.

## Owner-selected Argon composition — 2026-10-19

Following the enlarged-input experiment, the owner preferred Argon's visual coverage and authorized replay on existing poses. The active browser now uses `.tools/argon-composition-01/package/`, with original Argon polygons and recorded teeth-over-tongue ownership rather than manual dental contours. Full tongue evidence is retained and EI's visible tongue is protected during tooth removal. Other poses retain their own soft tissue and existing reviewed removal masks. All 94 processor checks and 44 Chromium/WebKit desktop/mobile checks passed; frontend type checking/build passed. No provider requests or image generation occurred during composition. SZ/FV remain static-only, unsafe geometry remains blocked and public automatic preparation remains unvalidated. See [reproduction, records and limitations](argon-composition-trial.md).
