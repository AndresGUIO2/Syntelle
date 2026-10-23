# Registered closed-mouth smile

The owner rejected phase 25's corner deformation: its protection region pinned the center lips while the corners curled upward. Phase 26 disables that smile for every character and replaces Syntelle's smile with an inspected reference. All three characters retain eyebrow controls. Previous assets, bindings and the rejected screenshot remain preserved.

## Authorization and records

The owner authorized one Syntelle reference and at most one visual correction, and requested adding it to the pipeline. The immutable configuration is `docs/characters/smile-reference-proposal.json`; PostgreSQL budget `authorized-smile-reference-2026-10-20` permits two images and zero analyses. **One image was received; no correction, retry, analysis or viseme regeneration was made.** Existing image/speech budgets are unchanged.

Exact original `Syntelle.jpg` bytes are the first image input; an automatic face crop from cached MediaPipe landmarks is second. The source prompt is preserved in `docs/prompts/smile-reference-v1.txt`. The existing `gemini-nano-banana-2.1` uses 1K, 3:4 and the established JPEG provider wire format. Raw bytes are retained, with decoded PNG/lossless WebP exports; these exports do not undo provider JPEG compression.

PostgreSQL stores complete original/crop bytes, prompt, request, settings, timestamps, hashes, raw response/output and usage. Inputs and a transactional reservation precede the POST. The response reported 4,378 total tokens (2,478 input); full modality details remain in `proposal/run.json`. The configuration's initial `requests_made: 0` is frozen authorization input, not a live counter. PostgreSQL and the run record report actual usage.

## Preparation and rendering

`app.workflows.motion.prepare_smile_reference` accepts explicit original/landmark/prompt/budget inputs. It reuses a matching received image; failed, incomplete or rebound requests cannot be retried. Its sole optional correction requires a hash-bound rejected visual review and supplies the immutable original again. No Café/Woodland reference request is authorized by this delivery.

`app.workflows.motion.bind_smile_reference` registers stable facial anchors excluding lips/eyelids. A common automatically derived lower-face domain includes complete lips and cheeks. Piecewise landmark displacement supplies geometry. Reference texture is normalized into neutral coordinates before interpolation: both materials share one moving lip seam. There are no manually edited masks or character placement overrides. Brow geometry reuses the preceding field.

`registered-closed-smile/1` is an optional `local-expressions/2` companion bound to original, motion and oral hashes. The accepted Syntelle field has a 0.85 smile reduction, approximately 0.532 minimum Jacobian and 0.172 source-pixel inverse residual. Quantized topology checks do not imply visual approval: native 0/50/100% images and actual browser composition were inspected separately. Inputs, transforms, landmarks, material alpha, algorithm copies, dependency/model versions and timings are archived. The static field/material upload once; expressions are applied before RGB-D projection without new WebGL contexts or full portrait uploads per frame.

## Behavior and limits

Open `/` → **Customize → Expressions → Smile** for Syntelle. Manual changes ease over 240 ms; reset returns exactly to neutral. Automatic mode retains sparse brow cues and adds a small smile after an utterance completes. Interruption does not trigger that completion smile. Reduced motion disables automatic expressions while retaining static manual control.

**The reference is closed-mouth only.** Smile is suppressed during articulation; cavity, tongue, teeth and cached visemes remain unchanged. A bounded preparation lead lets a manual smile settle before audio begins, outside the audio/viseme clock. The manual amount returns afterward. This does not deliver smiling speech, open-mouth laughter, or smiles at pauses inside an utterance. Café/Woodland retain brows without the rejected smile. Further expression references and articulation-compatible smiling remain future work. Runtime controls make no image request.

## Reproduction and verification

From `services/processor`, reuse the received result without a provider call:

```powershell
.venv/Scripts/python.exe -m app.workflows.motion.prepare_smile_reference --config ../../docs/characters/smile-reference-proposal.json --output ../../.tools/smile-reference-26
.venv/Scripts/python.exe -m app.workflows.motion.bind_smile_reference --config ../../docs/characters/smile-reference-proposal.json --reference ../../.tools/smile-reference-26/proposal --expressions ../../.tools/smile-reference-26/previous-runtime --output ../../.tools/smile-reference-26/prepared
```

`previous-runtime` freezes the rejected input fields; do not bind against the installed replacement. Installation requires a separate hash-bound composition review. Keep `.tools/smile-reference-26/` and phase-25 evidence intact.

Bounded checks passed: six processor cases, frontend type checking, scoped lint/formatting, production build, three Chromium desktop checks and one WebKit desktop check. These cover all three characters' controls/reset, deterministic scheduling/interruption and one intercepted cached Dragon replay. A center-lip regression independently rejects the previous pinned seam. No paid speech, exhaustive pose matrix, new performance benchmark, real-device approval or public-upload validation is claimed.

The complete delivery is archived/restored through `app.audits.archive_preparation`. Final identifiers and file counts are recorded in `.tools/smile-reference-26-archive.json` and `.tools/smile-reference-26-restoration.json`.
