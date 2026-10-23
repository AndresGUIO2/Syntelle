# Articulated neck and depth breathing

The local `head=neck-candidate` / `head=neck-compare` candidate adds `neck-volume/3` to the cached RGB-D packages for Syntelle, cafe and woodland. The owner requested lightweight verification and will judge the final appearance. The accepted preceding RGB-D resources remain at `head=rgbd-candidate|rgbd-compare`; no historical preparation is overwritten.

## Behavior

The neck blends three rigid transforms: the full head transform, an upper-neck transform receiving 85% of the rotation, and a lower-neck transform receiving 5%. The lower pivot anchors to the automatically estimated clothing/skin transition. Continuous normalized weights connect the regions, with a small shoulder follow. The face uses one rigid transform. Long hair and accessories outside the central neck retain the previous attachment influence rather than being dragged by a rectangular neck band.

The owner noted that the upper neck should rotate more than its base. Phase 19 already had nominal 65%/25% joint values, but they were insufficient as an explicit visual constraint. Phase 20 strengthens jaw coupling and anchors the lower neck more firmly. Preparation now measures and records the effective rotation influence at five heights through the actual interpolated weight field, requiring an ordered upper-to-lower decrease and at least a 70 percentage-point separation. These values are animation settings, not physiological measurements. Breathing is unchanged. The complete phase-19 candidate remains in PostgreSQL snapshot 353, `.tools/neck-motion-19/`, and the preserved runtime `neck-previous/` resources.

The common preparation uses cached facial landmarks, semantic confidence maps and source-bound geometry. Neck-base detection searches the central body-skin/clothing transition in a face-relative range; absent observations use a recorded conservative estimate. This is an estimated animation rig, not a measured cervical skeleton. There are no manually painted masks, individual placement corrections or per-character configuration values.

Breathing retains the existing asymmetric timing, seeded duration/amplitude variations and blink/speech scheduling. Its visible torso displacement now happens in world coordinates before perspective projection: a small lift, lateral expansion and forward depth displacement around a chest-centered viewing ray. Shoulder support starts earlier laterally so a tightly framed portrait still has a breathing region when most of its chest is outside the image. The head and face have zero breathing weights. There is no new generated material, gaze behavior or physical clothing simulation.

The optional `projection.articulation` extension stores pivots, common rotation fractions, chest center, localization evidence, validation and six additional per-vertex attributes. These are appended to the existing indexed binary with an explicit aligned offset. The decoder validates the offset, stride, finite weights and normalized joint influences. Historical binaries retain their original layout. Vertex data is uploaded once; expression updates remain local texture uploads. Transparent-boundary depth sorting accounts for the new transforms and breathing depth.

## Review

Open `http://127.0.0.1:5173/?trial=motion&character=syntelle&head=neck-compare`. The three synchronized views show:

1. Previous RGB-D, using its original deformation and breathing.
2. Articulated neck with the previous screen-space breathing.
3. Articulated neck with depth breathing.

Use **Replay comparison** for the existing deterministic head trajectory, or the manual yaw/pitch controls. Breathing continues on the same clock. The same three poses, mouth patches, eyelids and backgrounds feed all views. Pause, manual articulation, reduced motion and hidden-tab handling reuse the existing controls. `character=cafe|woodland` selects the other characters.

## Reproduction and bounded checks

From `services/processor`:

```powershell
.venv/Scripts/python.exe -m app.workflows.motion.prepare_neck_motion --output ../../.tools/neck-motion-21/prepared-current --experiment neck-motion-21
.venv/Scripts/python.exe -m pytest tests/test_neck_rig.py -q
```

`docs/characters/neck-inputs.json` supplies the previous prepared packages. Current results and exact parent inputs are preserved in `.tools/neck-motion-21/prepared-current/`; phase-19 drafts and reviewed resources remain separate. Source/workflow/settings hashes bind cache reuse. Every preparation archives inputs in PostgreSQL before processing and archives complete outputs afterward. No provider call or new inference is made. Historical command defaults target phase 19; use the explicit phase-21 arguments above for the current preparation.

All three source rigs pass the mathematical envelope gate at existing bounds: 54 yaw/pitch/roll/breath configurations per character. Three focused processor tests check neutral registration and geometry, rigid facial distances/distributed neck weights, and genuine depth breathing with zero face displacement. These are small local checks, not the previous exhaustive browser/pose matrix.

From `apps/web`, `node scripts/review-neck-motion.mjs` performs one brief Chromium review for each character. `--webkit` checks Syntelle only. The review captures representative turns, one combined closed-eye/FV-to-A frame, and native previous/neck/full frames. It reports exact neutral registration, unchanged facial pixels during breathing, nonzero neck/torso changes, one vertex upload, no page errors and no provider requests. Frontend format, lint, type checking and build are checked. The full processor and browser suites, every mouth pair, all backgrounds and performance benchmarks are deliberately not rerun in this iteration, following the owner's instruction. No renewed 60 fps or real-device claim is made.

The owner evaluates naturalness and preferred amplitude. Internal inspection is limited to obvious composition faults in the representative views. The preceding Syntelle/WebKit performance shortfall, physical-device approval and public-upload validation remain pending.

## Evidence

Run `app.audits.finalize_neck_motion --experiment neck-motion-21` after the bounded checks. It verifies source/package identity and immutable parent geometry, preserves the new sources and reports, archives `.tools/neck-motion-21/` in the existing PostgreSQL preparation archive, restores it and verifies file hashes. For the focused tests, set `SYNTELLE_NECK_PREPARED` to the absolute `prepared-current/` path; for browser output, set `SYNTELLE_NECK_REVIEW_OUTPUT` to the absolute phase-21 `browser-review/` path. Historical finalizers remain frozen. Credentials, image budgets, speech ledgers and database schemas are unchanged.

## Surface-axis correction and screen-space scale

The owner could not see the phase-20 correction. Measuring the actual projected central-neck vertices at 3 degrees yaw and a 362 px comparison-pane width showed a maximum phase-19/20 difference of only 0.32 px for Syntelle, 0.09 px for cafe and 0.15 px for woodland. The nominal rotation profile was not evidence of a perceptible change.

Phase 21 leaves the requested angle, rotation fractions, breathing and original surface geometry unchanged. It fixes neck-axis placement: source depth describes visible front skin, but the previous pivots were sampled on that surface. Preparation now estimates a skin-section width from semantic confidence and alpha, bounds it relative to face width, and places an inner axis using a recorded 0.45-diameter depth prior. This is an approximate geometric prior, not a measured anatomical axis or reconstructed hidden surface. Observed width, bounded diameter, inset and evidence/fallback status are recorded per section.

At the same 3-degree turn, the maximum phase-20/21 neck-vertex changes at 362 px pane width are 1.10 px (Syntelle), 0.81 px (cafe) and 0.54 px (woodland). Full-portrait differences remain small. The new **Magnify neck** control applies the same automatically derived CSS crop to all views; it changes neither pose nor rendering geometry and adds no graphics contexts or animation clock. At that detail scale, these changes are approximately 2.85, 3.14 and 2.86 px. Exact projected-geometry measurements and build IDs are in `screen-displacement.json`; the owner evaluates their visual usefulness. Earlier resources remain in their frozen phase-19/20 archives.
