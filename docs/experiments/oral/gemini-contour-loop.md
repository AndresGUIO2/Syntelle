# EI contour review with enlarged inputs and visual feedback

Date: 2026-10-19. Local preparation research only; no public generation endpoint or automatic visitor preparation acceptance.

## Flash experiment

The owner authorized an enlarged image, coordinate grid and one correction round looking at the rendered proposal. The recorded experiment is `.tools/gemini-contour-loop-ei-01/`. It uses the exact native EI crop from the cached original generation, without regenerating a viseme. The 170 x 116 crop is enlarged to 680 x 464 with Lanczos. Clean, normalized-grid and MediaPipe-lip views share precisely the same framing. The second request includes the actual first proposal overlay and validation report. No manually reviewed dental contours are supplied.

The configured model is `gemini-3.8-flash`, medium thinking, high image resolution, temperature 1, output limit 8192, no storage, redirects, fallback or retries. [Proposal prompt](../../prompts/dental-contours-grid-v1.txt), [correction template](../../prompts/dental-contours-correction-v1.txt) and [settings](../../characters/syntelle/gemini-contour-loop.json) are preserved with the exact expanded prompts, requests, raw responses, usage, source snapshots, masks and transforms.

Exactly two HTTP requests returned 200:

| Stage | Duration (s) | Input tokens | Output tokens | Thought tokens | Total tokens | Actual outcome |
| --- | ---: | ---: | ---: | ---: | ---: | --- |
| Proposal | 41.689 | 3726 | 761 | 3935 | 8422 | Complete JSON, simple nonoverlapping polygons, visually incomplete enamel coverage. |
| Correction | 68.407 | 5623 | 315 | 7860 | 13798 | Provider status `incomplete`; truncated JSON, no usable corrected mask. |

The first proposal has 23 upper vertices and 24 lower vertices. Its masks cover 349 upper and 195 lower native pixels at the validation threshold. Both lie inside the MediaPipe inner-lip aperture. The tongue is marked not visible. Visual inspection still finds omitted lateral enamel; geometric success is not semantic approval. See `proposal/review.png` for original versus mask.

The correction exhausted the configured output allowance, which includes reasoning as well as returned text. This is a budget/configuration failure, not evidence that the model cannot segment teeth. Its partial output is retained verbatim and must not be closed, repaired or presented as a corrected mask. The root experiment remains `stopped`; no third Flash request was made. Future captures explicitly reject incomplete provider statuses even when HTTP succeeds, after saving raw output and usage.

## MediaPipe's role

The cached Face Landmarker points provide outer and inner lip contours, already used elsewhere in preparation. The orange inner boundary in `mediapipe-lips.png` describes the opening; the cyan outer boundary describes lips. Neither identifies upper enamel, lower enamel or tongue. The experiment uses these as context and a diagnostic aperture check, without clipping or correcting Gemini's prediction. Landmark IDs, input/display/original transforms and the source pixels remain recorded.

References: [Face Landmarker](https://developers.google.com/edge/mediapipe/solutions/vision/face_landmarker), [official lip connections](https://github.com/google-ai-edge/mediapipe/blob/master/mediapipe/python/solutions/face_mesh_connections.py), [thinking and output limits](https://ai.google.dev/gemini-api/docs/thinking).

## Separate Argon review

The owner suggested consulting Argon after these experiments. Nano Banana remains the image-generation model; these contour experiments already use text-output vision models. Gemini 4 Argon supports image inputs and structured text outputs, but that does not establish more accurate dental boundaries. It is tested as a separate reviewer, not an image generator or runtime dependency.

The [review prompt](../../prompts/dental-contours-argon-review-v1.txt) and [configuration](../../characters/syntelle/gemini-contour-argon-review.json) authorize one recorded `gemini-4-argon` request with the same three enlarged views and the exact Flash proposal overlay. The prompt supplies Flash's coordinates and deterministic validation, asking for specific error analysis in region notes and complete corrected contours. This is an assisted review rather than an independent blind model benchmark.

The request uses medium thinking and a 24576-token cap to allow room for reasoning and complete JSON, with no retries, model fallback, manual mask input, viseme generation or active installation. Evidence goes to `.tools/gemini-contour-argon-ei-01/`. Completed caches are hash-verified and reused; failed/incomplete reviews stop without another call. Source experiment evidence is copied and hashed without overwriting the Flash records. Actual results are recorded below after the request completes.

The actual Argon request returned HTTP 200 with provider status `completed` in 34.012 seconds. Usage was 5888 input tokens (4320 image, 1568 text), 1327 output tokens, zero reported thought tokens and 7215 total tokens. Medium thinking was requested; the record does not imply that internal reasoning occurred when the provider reports zero thought tokens. No follow-up call was made.

Argon expanded upper coverage, including lateral enamel, and expanded the lower row. It also classified the dark reddish inter-row area as visible tongue, with stronger certainty in its notes than the image warrants. Its tongue and lower-tooth polygons overlap, so local geometry validation rejected the candidate. Upper/lower/tongue masks cover 483/319/445 native pixels. Upper and lower masks have 91/18 pixels outside MediaPipe's inner-lip aperture; this is a diagnostic discrepancy rather than independent anatomical ground truth. Visual inspection also finds coarse lower-row coverage. The masks and notes remain unchanged for inspection, not corrected or installed. No accepted automatic dental separator results from this test.

`comparison.png` shows original EI, Flash's initial proposal and Argon's returned mask side by side. Orange identifies Argon's proposed tongue region. It is a visualization of a rejected segmentation, not evidence of a correct tongue boundary. The complete record preserves prompt, settings, exact four image inputs, original provider text, transforms, raw contours, mask exports, validation, usage and implementation snapshots. Flash's incomplete correction remains a separate untouched experiment.

All 92 processor tests passed, with the existing Starlette test-client deprecation warning. Tests cover matched view coordinates, two-stage budgeting, exact visual feedback, incomplete HTTP-success output preservation, one-call Argon review, cache reuse, tamper detection and failure recovery without retries. No frontend files changed; no browser animation acceptance or frontend rebuild is claimed. The final audit is stored separately as `.tools/gemini-contour-review-audit.json` so the completed experiment cache remains immutable.

Reference: [Argon input types and capabilities](https://ai.google.dev/gemini-api/docs/models/gemini-4-argon).

The owner subsequently preferred Argon's visual result and authorized resolving layer overlap for composition. See [the offline composition trial](argon-composition-trial.md). The original rejected diagnostic and all analysis records above remain unchanged; deterministic occlusion is recorded as a separate preparation step.
