# Gemini EI contour experiment

Date: 2026-10-19. This is an owner-operated local vision experiment, separate from image generation and public uploads.

## Actual result

Gemini returned upper/lower tooth coordinates and marked the tongue as not visible. The upper polygon crosses itself and omits visible enamel. The lower contour also misses portions of the visible row. The geometry validator rejected the proposal. It was not repaired manually, installed as character masks or used for animation. This single input/configuration does not establish that Gemini segmentation is unusable generally, nor that it is suitable for unattended preparation.

Inspect `.tools/gemini-contours-ei-03/inspection/review.png`: original crop on the left, the returned polygon fill on the right, explicitly labeled rejected. This fill is a diagnostic visualization of invalid coordinates, not a valid segmentation mask. Blue identifies the predicted upper row, pink the lower row; the tongue mask is empty.

## Source and request

The exact cached original provider response is `.tools/visemes-01/ei/generated-ei.jpg`, SHA-256 `159cd003310d4abea52fe0d4326b47f4b982b4274ec04a83f0940468228d59d0`, with native dimensions 896 x 1200. Existing MediaPipe landmarks determine a native crop at x=364, y=489, width=170, height=116. The input is a lossless PNG encoding of those original decoded pixels, without resizing, alignment resampling, markup or manual mask information. Its SHA-256 is `1c0cc4d8aced7c0058d6a6bdd17509354737c5611f50e09cf75d4029a52d2a9a`.

The model is `gemini-3.8-flash`, which returns text. It receives an image and the [saved prompt](../../prompts/dental-contours-v1.txt), and is asked for normalized full-input-image [x,y] contours for visible upper teeth, lower teeth and tongue. Absence requires an empty polygon list. [Settings](../../characters/syntelle/gemini-contours-trial.json) version `gemini-contours/1.2` uses low thinking, temperature 1, an 8192-token output cap, JSON text output, `store=false` and no redirects or transport retries. A simple inline provider schema is followed by strict local Pydantic, coordinate, visibility, intersection and overlap checks. Gemini's reported coordinates are preserved verbatim; no tooth color, outline or placement was adjusted.

There were **three HTTP requests, one accepted analysis, and zero viseme/image generations**:

| Evidence folder | HTTP | Outcome |
| --- | --- | --- |
| `.tools/gemini-contours-ei-01/` | 400 | The image-understanding example suggested minimal thinking, but this model rejects that level. |
| `.tools/gemini-contours-ei-02/` | 400 | Low thinking with the original strict Pydantic schema was rejected with a generic invalid-argument response. The provider did not identify which field was invalid. |
| `.tools/gemini-contours-ei-03/` | 200 | A simplified inline schema was accepted. Returned upper contour failed local geometry validation. |

Each failed request was preserved before the next explicit configuration correction. There is no automatic retry loop, model fallback or regeneration. The accepted request took 5.184 seconds and reported 1,451 input tokens (1,080 image; 371 text), 505 output tokens, zero thought tokens and 1,956 total tokens. Rejected responses reported no usage. No monetary cost is inferred from these records.

## Records and replay

Each attempt retains its prompt, settings, exact request JSON, input PNG, raw provider response, request ledger and preparation-source snapshot. The accepted-response folder also contains output text, parsed contours, usage and cached-file hashes. The offline `inspection/` directory stores masks, overlay, comparison sheet, coordinates in normalized/input/native-original/aligned-character spaces, affine transforms, the rendering source and an inspection report. `audit.json` records final checks. Credentials stay in local secret configuration and are excluded from evidence.

From `services/processor`, replay the rejected proposal for inspection without contacting Gemini:

```powershell
uv run --frozen --extra preparation python -m app.gemini_contour_trial --inspect-invalid
```

The ordinary command reuses a valid cache or refuses failed/invalid evidence; it never retries a previous request. `--request` is reserved for explicitly authorized provider work. It cannot be combined with `--inspect-invalid`. Each output ledger is created exclusively before its one request, and existing inputs/settings/cache hashes are checked before reuse. A future experiment needs its own authorized configuration and evidence directory. Do not issue another request merely to improve this result.

## Verification and boundaries

All 85 processor tests passed, with the existing Starlette test-client deprecation warning. New checks cover exact input pixels, text-only output configuration, bounded calls/cache reuse, provider failure recovery, credential redaction, coordinate transforms, invalid polygon/overlap rejection, unsupported thinking preflight and offline rejected inspection without repair or promotion.

Final audit verified exact source-crop pixels; request/response/source snapshot hashes; unchanged immutable base, all eight original viseme raw/run records and active package/runtime assets; and absence of the configured secret from all three experiment folders. Frontend files and public endpoints were not changed. No frontend rebuild or browser animation acceptance is claimed for this experiment.

Relevant provider references: [image understanding](https://ai.google.dev/gemini-api/docs/image-understanding), [structured outputs](https://ai.google.dev/gemini-api/docs/structured-output), and [model-specific supported thinking levels](https://ai.google.dev/gemini-api/docs/models/gemini-3.8-flash). The model-specific low/medium/high list takes precedence over the image-understanding example's minimal setting.

## Method audit after owner challenge

The owner's challenge is valid: the assistant's reviewed contours and this API result were produced under different protocols. The assistant used repeated enlarged observations, a coordinate grid and manual edits. Gemini received one unannotated 170 x 116 native-pixel crop, low thinking and instructions to serialize free-form closed polygons, without a subsequent visual feedback step. This is not a controlled comparison of model capability. Enlargement can aid coordinate work but cannot recover missing source detail; the small crop is a possible factor, not an established cause.

An offline audit verified that the image bytes transmitted in the request exactly match the saved input and that rendering uses x*170/1000, y*116/1000 without swapping axes. The superior-row crossing is present in the returned 14-point polygon before rasterization. The lower polygon has 11 points and is geometrically simple, which does not establish semantic accuracy. Evidence is in `.tools/gemini-contours-ei-03/method-audit.json`. No new provider request was made during this audit.

A fairer follow-up would give the automatic method comparable viewing aids, make contour ordering easier to express, and evaluate a bounded proposal/render/visual-feedback correction loop. Model, reasoning level, image-resolution controls and input scaling must be recorded as experimental factors. Do not infer that upscaling, a grid, more reasoning or a feedback round will improve quality until tested. The previous failed setup and rejected contour remain preserved; no follow-up request is implied by this note.

The owner subsequently authorized that bounded follow-up and suggested Argon as another reviewer. See [the enlarged-input and feedback experiment](gemini-contour-loop.md) for its separate records and actual results. The original one-shot evidence above remains unchanged.
