# Automatic foreground refinement

Historical v4 record: remaining background between hair strands is addressed by [Native hair boundary correction](foreground-edge-review.md). Current CLI defaults and finalization commands are documented there; preserve this experiment for comparison.

The motion candidate's previous foregrounds showed pale hair fringes and hard clipped tips. The replacement uses MediaPipe for semantic localization and **PyMatting 1.1.16** for source-resolution closed-form alpha estimation and multilevel foreground-color estimation. No new image generation, contour analysis or paid provider request is involved.

## Preparation and evidence

`app.refine_foreground` accepts the existing manifest, a previous preparation root and a separate output root. It reuses immutable originals, cached eye resources, existing mouth packages, landmarks and depth. Existing browser manifests retain the same schema; only the foreground and its build binding change. Original motion evidence remains in `.tools/motion-09/`; the new preparation lives in `.tools/foreground-10/`.

All six class confidence maps are retained, along with the combined subject confidence. Confidence values are not treated as alpha. The face-owning connected component seeds an automatically expanded uncertain boundary. Conservative opaque seeds and enclosed interior seeds preserve shadowed hair/clothing; confident true background holes remain background. Closed-form matting estimates fractional coverage in the remaining region. Foreground-color estimation removes old-background contamination. Exported fully opaque RGB is restored byte-for-byte from the source.

The first candidate, `semantic-closed-form-matting/3`, reduced Syntelle's pale fringe but failed enlarged browser review: woodland's interior braid became translucent. Its exact source, outputs, masks, records and rejection reason remain in `rejected-v3/` and PostgreSQL. The general interior-seed correction became `semantic-closed-form-matting/4`; all three characters were rerun through that same algorithm. No character-specific mask, hand-drawn contour or placement override was introduced.

Syntelle was inspected first, then cafe and woodland. Native dark/white composites, checkerboard images, all three supplied backgrounds with and without blur, staged browser screenshots and motion samples are retained. Development inspection is recorded separately from owner acceptance. A single photograph cannot establish ground-truth subpixel hair opacity; difficult matching foreground/background colors remain a limitation. This does not complete public-upload or held-out validation.

Cache identity includes source RGB, semantic model checksum, matting version/settings and implementation hashes. Every cached output is hashed. Complete foreground colors, class maps, trimap, alpha, exports, preparation source and records are stored as PostgreSQL bytes. Dependencies are pinned in `uv.lock` and remain in the offline preparation extra.

## Reproduction

From `services/processor`, with local PostgreSQL available:

```powershell
uv run --frozen --extra preparation python -m app.refine_foreground --cases syntelle
```

Inspect Syntelle and record its actual foreground hash in `foreground-review.json`. This is a review gate, not a manually repaired mask. After acceptance, prepare `--cases cafe woodland`. Matching runs reuse segmentation/matting caches; no command in this module calls a provider. Use a new output root for a changed experiment instead of overwriting frozen historical evidence.

From `apps/web`, `node scripts/review-foreground.mjs` previews staged foreground image bytes without installation, including background and motion samples. Review all three characters and record `browser_review: true`. Then use `app.refine_foreground --phase install`; the installer verifies every character's current review before replacing any runtime foreground.

After verification, `uv run --frozen --extra preparation python -m app.finalize_refinement` audits immutable inputs, unchanged eye/depth assets, opaque RGB, installed hashes, request counts and recorded rendering comparisons. It archives new evidence and the source snapshot in separate PostgreSQL experiments and restores all files into `.tools/refinement-10-restored/`, verifying every byte. It does not modify the previous motion experiment or reset request allowances.

## Verification

Synthetic fixtures independently measure known translucent strands, edge-color contamination over black/white destinations, opaque texture preservation, disconnected furniture, enclosed shadowed hair and genuine background holes. Processor tests pass with PostgreSQL integration enabled. Browser and performance results are recorded under `.tools/organization-10/`; final counts and measurements are summarized in `docs/development/code-organization.md`.

Recorded source-resolution matting time was approximately 11.7 seconds for the 1792×2400 Syntelle source and 1.5–1.8 seconds for the 1024×572 sources. Peak process working sets were approximately 2.09 GB and 0.50 GB respectively; these are process-lifetime peaks, not incremental per-image allocations. Replaying all three matching preparations reused their caches and retained identical foreground hashes. This computation happens during local preparation, not browser animation.

References: [PyMatting implementation](https://github.com/pymatting/pymatting), [foreground estimation](https://pymatting.github.io/foreground.html), [MediaPipe segmentation](https://ai.google.dev/edge/mediapipe/solutions/vision/image_segmenter).
