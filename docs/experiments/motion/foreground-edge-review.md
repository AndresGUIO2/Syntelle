# Native hair boundary correction

The owner correctly identified original-background remnants between Syntelle's hair strands after `semantic-closed-form-matting/4`. The initial suggestion that the complaint concerned highlights was incorrect. Coarse semantic foreground seeds locked contaminated boundary pixels opaque. Changing the alpha estimator while keeping those seeds could not remove the reported remnants.

## Common preparation correction

`semantic-native-hair-matting/5` retains MediaPipe class confidence maps and PyMatting closed-form estimation. It automatically unlocks foreground certainty in a hair-dominant boundary band, within 5% of the smaller source dimension from known background. Face, clothing and deep interior seeds retain existing protection; known background holes remain background. There are no character names, hand-edited contours or character-specific coordinates in this algorithm.

Pinned ViTMatte Small Composition-1k estimates alpha on overlapping **native-resolution** 512-pixel tiles, with 320-pixel stride and smooth weighted overlap. Its result replaces only uncertain hair coverage. Other regions retain the closed-form result. PyMatting estimates foreground color after alpha refinement; fully opaque exported RGB is restored exactly from the immutable source. No brightness key removes pale hair, highlights or white clothing.

The Apache-2.0 model is pinned to `hustvl/vitmatte-small-composition-1k` revision `6a58ad7646403c1df626fbd746900aec7361ea1d`. Original safetensors, configuration, model card and SHA-256 checksums are retained separately. Inference runs locally on CPU using existing pinned Torch/Transformers dependencies, with no remote code or portrait upload. Model identity, algorithm source and settings participate in the cache key.

Earlier offline candidates remain under `.tools/foreground-11/`: shared matting, local color projection, low-resolution ViTMatte and native inference with the old trimap. They retained contamination or introduced seams and were rejected. Diagnostic crop coordinates serve inspection only. An initial comparison-export variable-shadowing failure was fixed without rerunning model inference; its failure, recovery record and PostgreSQL snapshot remain preserved.

## Inspection and verification

Syntelle passed development inspection first, followed by cafe and woodland through the identical algorithm. Native black/white/checkerboard compositions, all three supplied backgrounds with/without blur, staged browser captures and deterministic motion samples are retained in `.tools/foreground-11/prepared/`. The reported pale hair gaps are visibly reduced without the brown banding seen in rejected color-projection candidates. Woodland's silver hair, ears, braids and clothing remain present. Previous foregrounds remain untouched in `.tools/foreground-10/`.

All three reviewed foreground hashes were installed in the selectable motion trial. Existing oral dependencies, eye layers and depth remain unchanged. This is internal development review, not owner approval or completion of public-upload validation. Exact ground-truth hair opacity is unavailable for these portraits; difficult boundary colors remain a limitation.

- Processor: 151 tests pass, including regressions for erroneous coarse hair certainty and native tile coverage, plus existing known-alpha/foreground-color fixtures.
- Frontend: lint, formatting, type checking and production build pass.
- Chromium/WebKit desktop and mobile emulation: 40 motion scenarios pass, covering all three characters, background-only blur, iris stability, endpoint consistency, context reuse and visibility suspension.
- Preparation times: approximately 44.7 seconds for Syntelle, 10.9 for cafe and 6.6 for woodland. Process-lifetime peak working sets: approximately 2.21 GB and 1.11 GB respectively; these are not incremental image allocations.

Local motion stress measurements remain approximately 60 fps in Chromium. WebKit measured approximately 55.5 fps for desktop Syntelle and 61.5–62.1 for the other two; its existing strict 60 fps Syntelle target remains unmet. Mobile emulation measured approximately 60 fps Chromium and 64.6–64.9 WebKit on desktop hardware, not a real phone. This change adds preparation work only; it does not change the browser renderer. Records are in `.tools/foreground-11/performance/`.

## Reproduction and records

From `services/processor`, with local PostgreSQL available:

```powershell
uv run --frozen --extra preparation python -m app.refine_foreground --cases syntelle
uv run --frozen --extra preparation python -m app.refine_foreground --cases cafe woodland
```

Defaults now preserve `.tools/foreground-10/` and prepare `.tools/foreground-11/prepared/`. For a new experiment, supply distinct `--previous`, `--output` and `--experiment` values. Inspect Syntelle before preparing others. From `apps/web`, `node scripts/review-foreground.mjs` previews staged image bytes without installation. All three actual hashes must pass development and browser review before `app.refine_foreground --phase install` accepts installation.

`app.finalize_foreground_edges` audits cache bindings, original hashes, opaque RGB, runtime hashes and unchanged eye/depth/oral dependencies. It archives complete outputs, rejected diagnostics, automatic masks, model weights, sources, configuration and verification records as PostgreSQL bytes, restores each snapshot, and verifies hashes. The earlier `app.finalize_refinement` is the historical v4/refactor finalizer; use the new finalizer for this method. No image generation or paid analysis occurs, and existing request budgets remain unchanged.

References: [PyMatting foreground estimation](https://pymatting.github.io/foreground.html), [pinned ViTMatte model card](https://huggingface.co/hustvl/vitmatte-small-composition-1k/blob/6a58ad7646403c1df626fbd746900aec7361ea1d/README.md).
