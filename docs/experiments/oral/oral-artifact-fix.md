# Dental edge and commissure artifacts

The owner's FV→A screenshots exposed defects in the previous completed-dentition trial: thin angular remnants beside the lower teeth and rough enamel edges. This correction is `completed-oral-rig/2`, renderer `completed-oral/2`, staged in `.tools/oral-artifacts-05/package/`. The previous completed package is preserved in `before-package/`; all original generation records remain in their original experiments. No Gemini requests or viseme regeneration were made.

## Changes

- Rectangular dental removal annotations no longer define lip cleanup. Lip reconstruction uses the aperture, nearby enamel proposals and residual dark cavity pixels. Removing teeth from the cavity cannot repaint a rectangular area of the lower lip. Tongue pixels remain protected in the cavity layer.
- Generated coverage matches the nearest protected EI fragment along a six-pixel seam band. Protected selected RGB stays exact; the generated side of the boundary changes. Small generated green remnants are despilled without using brightness to determine opacity.
- Dental textures are uploaded with premultiplied color, filtered, then unpremultiplied before color-only lighting. Fractional lower-jaw movement no longer mixes transparent black into enamel RGB and then applies coverage a second time.
- The lip boundary uses signed-distance subpixel coverage, independent of cavity-triangle rasterization. A hardware-MSAA candidate differed between Chromium and WebKit and was replaced with this deterministic shader coverage. Inversion/coverage validation remains mandatory for the lip mesh.
- Endpoint sampling selects the exact endpoint texture instead of relying on floating-point interpolation at zero/one. All incoming/outgoing pairs retain exactly equal endpoint pixels.

## Verification

Processor checks cover original-material preservation, generated seam continuity, lip pixels outside the cleanup region, tongue preservation, chroma opacity, immutable sources and request bounds. Browser checks cover all nine endpoints, FV→A at the reported 41%, A→FV, other representative pairs and all 648 pair/sample combinations. A new synthetic lower-row regression verifies 30% coverage after a 0.7-pixel translation: its edge color is proportional to coverage, without an extra black fringe. Exported fallback and live FV are compared in each browser configuration.

Visual inspection still matters: these checks establish material and boundary behavior, not final phonetic approval. The original source resolution limits enlarged detail. This remains a single-character experiment, separate from automatic visitor preparation.

Final checks: **103 processor tests passed**, frontend type checking/production build passed, and **16 Chromium/WebKit desktop/mobile checks passed without snapshot updates** after inspecting and recording the new references. The normal browser report is retained in `oral-artifacts-05/browser-test-results.json`. All 459 protected source files and the prior completed package are unchanged. The exported pair matrix has zero endpoint differences and zero outside-mask alpha. The in-app browser was also inspected at FV→A 41% and A 100%.

## Offline replay

From `services/processor`:

```powershell
uv run --frozen --extra preparation python -m app.prepare_completed_oral --install-candidate
```

With Vite running, from `apps/web`:

```powershell
node scripts/export-oral-review.mjs
```

Then, from `services/processor`:

```powershell
uv run --frozen --extra preparation python -m app.prepare_completed_oral --finalize-browser-exports
uv run --frozen --extra preparation python -m app.oral_review_sheet --experiment E:/Carpetas/Documentos/01-MisProyectos/ImageAssistant/.tools/oral-artifacts-05
uv run --frozen --extra preparation pytest -q
```

From `apps/web`, run `npm run build` and `npx playwright test tests/e2e/completed-oral.spec.ts`. Compare snapshots without updating them. `--output` on preparation/finalization and browser export allows a separate inspection folder; no provider is contacted.
