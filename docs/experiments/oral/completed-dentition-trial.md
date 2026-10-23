# Completed selected dentition

Current follow-up: [Dental edge and commissure artifacts](oral-artifact-fix.md). The records below describe the first completed-material package; current offline defaults preserve it and write the correction to `oral-artifacts-05/`.

The default development trial (`/?trial=viseme-a`) uses `completed-oral-rig/1`, renderer `completed-oral/1`. The comparison link opens the preserved previous cutout package at `/?trial=viseme-a&oral=previous`. This is a single-character experiment, not public-upload preparation or validated speech.

## Generation and material

The source is the original EI image and its existing Argon contours. The immutable portrait, eight cached visemes, original Argon polygons and rejected dental plate remain unchanged. `.tools/dentition-completion-04/baseline.json` records their hashes; `previous-runtime/` retains the old browser assets.

`complete_dentition.py` creates a registered 1024×1024 canvas with actual upper/lower EI fragments at uniform 4× scale in recorded slots. The exact portrait, original mouth crop and protection guide accompany `docs/prompts/dentition-completion-v2.txt`. The prompt requests extension of those crowns and lateral/attachment coverage, not a newly designed denture.

Exactly **two HTTP attempts** were used, without transport retries:

1. `initial/`: HTTP 400. This provider/model rejected PNG output and required JPEG. No image was returned.
2. `correction/`: HTTP 200, `gemini-nano-banana-2.1`, square 1K. This was a format correction, not a second generated visual variant. The original 1024×1024 JPEG is retained. Working layers/exports are PNG and lossless WebP; these cannot undo JPEG source compression.

The successful response took 25.598 seconds and reported 7,898 total tokens: 4,984 input, 2,056 output and 858 thinking tokens. Raw image SHA-256: `de4d78717f79078cb19c07cfe9448687cbc77bf81dd76b748ecf72fcd384a7d2`. Billing was not measured. No viseme was regenerated.

Incisal-profile registration recovered upper displacement (-20, 101) and lower displacement (0, -2) atlas pixels. Both arches retain unit scale in character coordinates. Original selected RGB is restored on 1,916 upper and 1,273 lower pixels before illumination. Hidden coverage adds 13,299 upper and 11,514 lower pixels. Early registration diagnostics remain separately available.

Chroma-background components and enclosed key-colored apertures determine opacity. Dark enamel/attachment material remain opaque. Edge despilling and premultiplied resampling remove matte contamination. Enamel-only color matching avoids pink tissue as the illumination reference; protected source pixels remain unchanged. Generated hidden crowns and roots are estimates, not recovered anatomy.

## Oral package

`package-v2/` contains completed arches, protected/completed masks, original and retargeted soft tissue, pose-specific cavity/tongue sources, lip occluders, landmark meshes, placement settings and hashes. `pipeline-snapshots/` preserves the source code and settings. Raw requests, responses and credentials are not installed in the browser. Credentials remain in local secret configuration.

The upper arch stays anchored to the face. The lower arch translates rigidly as one layer. No individual teeth move or stretch. Placement and lighting parameters are recorded in `docs/characters/syntelle/completed-oral-rig.json`.

FV's lower lip is retargeted to the upper-incisor edge. O's upper lip and CHSHR's local aperture are retargeted inside the existing patch. O/U derive lower-arch occlusion from the completed incisal profile. Rest/MBP expose no cavity. SZ uses close-row placement. CHSHR retains a contracted aperture with the lower row behind its lower lip. These are recorded character calibrations, not an automatic phonetic solver.

Lips and cavity have separate textures. This prevents reconstructed cavity borders from painting dark rectangles on the lip surface. Recorded removal regions are augmented with local enamel proposals, and photographed cavity samples extend behind removed teeth. No constant dark plate or artificial dental shutter is used. Visible tongue material is preserved behind the dentition; lips determine visibility.

One WebGL compositor renders endpoints, intermediate frames and exported previews. Contact/depth shadows change color in linear light, independently of alpha. Crossfade is an intentionally separate diagnostic comparison; landmark morph preserves dental material.

The stage explicitly tracks the playback amount because the renderer mutates one canvas without changing its identity. An additional end-to-end regression caught stale displayed frames despite an updated pose label. The corrected live FV endpoint matches its exported, WebGL-free fallback within one 8-bit level.

Mesh validation remains mandatory. Correcting lip-quad diagonal winding resolved U↔CHSHR without bypassing inversion checks. All 72 directed transitions and nine identity meshes pass signed-area/coverage checks.

## Verification

- 101 processor tests passed, covering chroma opacity, enclosed key holes, premultiplied filtering, cache changes, bounded failure records and restored selected material.
- Frontend type checking and production build passed. The experiment stays development-only.
- 60 Chromium/WebKit desktop/mobile checks passed: 12 exercise the completed package, while 48 retain previous-package coverage. Mobile configurations are emulations.
- Completed-package checks render 648 frames per configuration: all 81 pairs at 0%, 1%, 25%, 35%, 50%, 75%, 99% and 100%. Endpoint differences across pairs are exactly zero. Pixels outside both edit masks remain untouched.
- Representative exposed unlit upper enamel matches the canonical texture within one 8-bit level. Lighting changes no alpha values and darkens the material without overshoot.
- Visual regression images cover nine endpoints and representative pairs, including EI↔FV at 35%, EI↔SZ, A→EI, O→U, MBP→A and U→CHSHR. Slow playback and EI→FV at 35% were inspected in the in-app browser.
- Internal inspection used endpoint sheets and every directed pair at 25%, 35%, 50% and 75%. `browser-review/before-after.png` compares the reported EI→FV frame, FV contact, EI→SZ and CHSHR. All rendered frames remain available.

These checks establish local material/compositing behavior, not universal visual quality. Source resolution limits enlarged tooth detail. This is a frontal 2D approximation, and the broad CHSHR grouping is a phonetic approximation. Owner motion acceptance, physical iPhone testing and automatic visitor teeth/tongue separation remain pending.

## Offline reproduction

From `services/processor`:

```powershell
uv run --frozen --extra preparation python -m app.complete_dentition --replay --correct
uv run --frozen --extra preparation python -m app.prepare_completed_oral --install-candidate
```

With Vite running, from `apps/web`:

```powershell
node scripts/export-oral-review.mjs
node scripts/export-oral-review.mjs --previous
```

Then, from `services/processor`:

```powershell
uv run --frozen --extra preparation python -m app.prepare_completed_oral --finalize-browser-exports
uv run --frozen --extra preparation python -m app.oral_review_sheet
uv run --frozen --extra preparation pytest -q
```

Finalization checks the exact prepared build, all frame records, endpoint equality and outside-mask preservation, then installs lossless exports from the browser compositor. It does not generate images. Both attempted request stages are locked against reuse. From `apps/web`, run `npm run build` and `npx playwright test tests/e2e/completed-oral.spec.ts tests/e2e/viseme-trial.spec.ts`; compare existing screenshots without updating them.
