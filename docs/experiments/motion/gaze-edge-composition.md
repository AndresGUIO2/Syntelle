# Iris boundary composition correction

The owner reported intermittent pupil-mask defects. Phase 28 corrects two reproducible faults in the shared compositor for Syntelle, cafe and woodland. This is a rendering correction; source portraits, iris masks, gaze bounds, blink assets, movement planning and provider budgets are unchanged.

## Defects and correction

The previous compositor restored the original eye wherever a shifted sample fell outside the source aperture, even when that sample was background rather than iris. This could preserve a stationary crescent in a vacated iris region. Unsupported original material is now retained only where the translated iris actually needs it. A continuous coverage equation replaces the sharp 99%-support threshold.

The soft limbal edge was also matted twice: the photographed iris already included its antialiased sclera boundary, then that color was multiplied by the iris mask again. Neutral gaze bypassed this path, so movement could introduce a pale rim immediately. Covered foreground color is now recovered against the prepared sclera before bilinear filtering. The iris core remains opaque, with the original pupil/iris texture translated rigidly; no brightness-derived opacity, pupil scaling, hidden-iris synthesis or manual mask repair is introduced.

Four source texels are filtered as covered color/alpha using the existing textures. Signed covered color is retained until final composition, avoiding a new clipping seam during unmatting. The CPU reference in `app.preparation.gaze.render` follows the same equation, identified as `supported-iris-over/2`. Existing `observed-iris/1` packages remain compatible, with no rewritten package IDs or new runtime assets. Six eye texture units and the existing persistent WebGL context/partial eye uploads remain in use.

## Evidence and bounded checks

The old CPU compositor altered 175 Syntelle eye pixels, 52 cafe pixels and 26 woodland pixels at a 0.0001-pixel offset (which rounds to zero in the CPU sampler), with maximum RGB differences of 9–11. The corrected CPU compositor changes zero pixels in that case. Native source comparisons and earlier code are preserved under `.tools/gaze-edge-28/`; this verifies the onset defect rather than asserting every reported visual defect is solved.

Eight processor tests pass. Independent synthetic cases cover vacated pixels, red-skin contamination, subpixel continuity and double-matting at the soft edge. Six focused Chromium/WebKit desktop cases pass: the same synthetic behavior on the GPU, the three characters at axis/diagonal bounds and partial/full blinks, and neutral/closed/pupil/outside-aperture invariants. Native eye sheets were inspected. Scoped formatting/lint, frontend type checking and the production build pass. The initial test-only TypeScript errors were corrected before delivery.

Verification remains bounded as the owner requested. No new speech, image, contour or inference request was made. No exhaustive mouth matrix, mobile-device approval or fresh rendering benchmark is claimed; the existing sustained WebKit performance limitation remains open. The small unsupported iris rim permitted by preparation still exists; wide gaze remains unsupported. Owner visual acceptance is pending.

Evidence, current/previous source, unchanged prepared inputs and checks are archived/restored through the generic PostgreSQL preparation archive. Identifiers and file counts are in `.tools/gaze-edge-28-archive.json`.
