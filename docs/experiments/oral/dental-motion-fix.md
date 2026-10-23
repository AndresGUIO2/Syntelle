# Opaque dental motion and experimental pose playback

Date: 2026-10-19. The owner reported dissolving teeth and objected to the manual-review playback restriction. This local correction reuses all existing images, Argon masks and protected tongue tissue; it makes no provider requests and generates no new images.

## Root cause and correction

The previous renderer linearly interpolated `globalAlpha` between visible and toothless open-pose flags. In transitions such as EI-to-O or A-to-U, exposed teeth therefore darkened into the cavity rather than remaining opaque until covered. That earlier choice prevented a late floating dental strip but introduced the reported dissolve.

Renderer `pose-morph/3` keeps tooth opacity unchanged. `dentalOcclusion.ts` derives each row's vertical support from its actual alpha channel once per renderer. An advancing geometric cover hides the upper row from above and the lower row from below as the pose becomes toothless. The cover extends two crop pixels beyond the row support so no residual strip survives near the endpoint. Closed Rest/MBP continue to rely on the interpolated lip aperture. Target lip clipping, the existing lip inset, fixed upper coordinates and rigid lower translation remain in effect. No enamel RGB, texture or shape is recolored or stretched.

This cover is an explicit 2D visibility approximation, not a new anatomical lip model. Antialiased boundaries remain partial coverage; the opaque enamel core no longer dissolves. The optional crossfade comparison still intentionally blends entire frames. The default landmark method uses the opaque dental composition.

## Playback eligibility

The new [motion configuration](../../characters/syntelle/viseme-motion-review.json) explicitly selects `geometry-only-experiment` for this owner-operated trial. SZ and FV retain `experimental` quality metadata and their unresolved phonetic notes, but those quality notes no longer disable playback. They are not relabeled as phonetically accepted. Pose selectors show plain identifiers without `review only`, and repeated review warnings are removed.

There are 70 enabled directed transitions across nine endpoints. U-to-CHSHR and its inverse remain disabled because their direct meshes invert; the interface reports the actual geometry failure and keeps endpoint inspection available. Removing the approval gate does not bypass mesh validation or establish public automatic preparation readiness. Earlier configurations retain their original accepted-only policy.

The browser component responsibilities are unchanged: `VisemeTrial` selects and coordinates the pair, `VisemeStage` paints the portrait/detail, the playback composable manages timing, and `poseMorph` plus the small dental drawing utility render the patch. No new product UI flow, dependency or service is introduced.

## Reproduction and records

From `services/processor`:

```powershell
uv run --frozen --extra preparation python -m app.prepare_viseme_package --review ../../docs/characters/syntelle/viseme-motion-review.json --output ../../.tools/dental-occlusion-02/package --install-trial
```

The active local trial remains `http://localhost:5173/?trial=viseme-a`. The output package, resolved settings, source snapshots, prior runtime backup and final `audit.json` live in `.tools/dental-occlusion-02/`. Original analysis/generation records and their prompts are unchanged. The existing default preparation command still refers to an older experiment; use the explicit motion-review command above for this variant.

## Verification and remaining limits

All 94 processor tests passed, including 70 geometrically valid pair checks sampled through their transitions; the existing Starlette test-client deprecation warning remains. Vue type checking and production build passed. All 48 Chromium/WebKit desktop/mobile tests passed without retries. Coverage includes SZ-to-FV and FV-to-CHSHR playback, no review-only labels, retained invalid-mesh blocking, exact endpoint rendering, fixed upper-tooth pixels, outside-mask pixel preservation, reduced motion and resource failure.

A regression test samples a fully opaque real upper-tooth pixel during EI-to-O at 25% and 50%: RGBA stays byte-identical to the canonical tooth texture, which failed under the old opacity interpolation. At A-to-U 99%, replacing both reference rows with transparent fixtures now makes exactly zero pixel difference, rather than tolerating residual faded enamel.

The audit verifies 75 protected original/model-evidence files unchanged, package/source hashes, exact installed runtime bytes, canonical tooth exports byte-identical to the previous version, protected tongue checks and zero edits outside pose masks. No configured secret occurs in the new evidence or runtime resources. `browser-test-results.json`, `browser-review.png` (EI-to-O 50%), `sz-fv-midpoint.png` and `fv-chshr-midpoint.png` preserve inspection evidence.

Manual browser inspection confirms the opaque EI-to-O midpoint and plain pose selectors. SZ/FV are now available for motion comparison; their previously identified articulation limitations remain quality work, not a reason to prevent the owner from testing them. No public-upload preparation, speech synchronization or final phonetic acceptance is claimed.

The owner subsequently reported unnatural row spacing. [Jaw spacing calibration](dental-spacing-trial.md) supersedes this package in the active browser, retaining its opaque renderer and playback policy while improving lower-row placement and clearance.
