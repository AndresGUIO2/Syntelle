# Argon masks on the cached viseme set

Date: 2026-10-19. The owner preferred Argon's visual coverage and authorized an offline composition and browser trial. This supersedes the earlier decision to keep that candidate out of composition; it does not alter the original model response or erase its overlap rejection.

## Inputs and reproducible preparation

The contour source is `.tools/gemini-contour-argon-ei-01/review/contours.json`. The exact cached EI generation, original image hash, prior analysis inputs, affine transforms and all experiment hashes are checked before replay. There are **zero provider requests and zero new image generations** in this milestone. Argon's prompt and request settings remain in the original analysis folder and the repository [prompt](../../prompts/dental-contours-argon-review-v1.txt) and [configuration](../../characters/syntelle/gemini-contour-argon-review.json).

Preparation uses [Argon replay settings](../../characters/syntelle/argon-composition.json), [pose review settings](../../characters/syntelle/viseme-argon-review.json), `preparation/argon_masks.py` version `argon-mask-replay/1`, package pipeline `oral-layers/6-argon` and existing renderer `pose-morph/2`. There are no hand-edited Argon contour coordinates. Original model vertices map from normalized crop coordinates through the saved native-to-aligned-character transform into the package crop; rasterization uses 4x supersampling and Lanczos. Canonical rows retain original EI RGB pixels, with fixed upper coordinates and rigid lower translation. Extraction retains the full enamel contours rather than clipping them to MediaPipe's approximate aperture; target lip apertures still perform rendering occlusion.

From `services/processor`, reproduce this variant without contacting a provider:

```powershell
uv run --frozen --extra preparation python -m app.prepare_viseme_package --review ../../docs/characters/syntelle/viseme-argon-review.json --output ../../.tools/argon-composition-01/package --install-trial
```

The previous runtime is backed up in `.tools/argon-composition-01/previous-runtime/`. The old manually reviewed package at `.tools/dental-selection-01/package-current/` remains available. The generic preparation command's default review still targets that older experiment; use the explicit command above to retain the Argon browser variant.

## Resolving overlap without replacing the tongue

The compositor assigns visible foreground ownership to enamel. The full Argon tongue mask remains saved as `package/argon-masks/tongue.png`; a separate `tongue_visible.png` excludes support already owned by upper/lower enamel. The original contours are not moved or repaired. Overlap between the two enamel rows would remain an error because their ownership cannot be resolved by this rule.

At aligned-crop resolution, the full tongue has 2201 nonzero support pixels, including antialiasing. Visible tongue has 1435 support pixels; 766 lie behind enamel support. Visible semantic overlap is zero. These are mask-support counts, not a claim that 766 anatomical tongue pixels were deleted. The full original source and full predicted tongue mask remain unchanged.

EI uses the visible tongue as inpainting protection and Argon's enamel masks for tooth removal. All 1435 protected source pixels remain byte-identical in its cleaned surface. Existing masks and pose-specific soft tissue are retained in the other poses: the reference tongue/cavity is not pasted across them. Lips and soft tissue deform as before; canonical teeth are composited in front and clipped by target lips. This is not a new independent tongue animation system. The existing per-pose removal masks remain internally reviewed character annotations; this milestone does not solve arbitrary visitor uploads.

## Browser trial and quality limits

The existing local trial at `http://localhost:5173/?trial=viseme-a` loads this variant. Expand the dental reference to see the original EI crop alongside Argon's upper teeth in blue, lower teeth in pink and visible tongue in orange. Source/target selection, playback, manual interpolation, magnification and crossfade comparison remain available.

There are nine endpoints and 40 enabled directed transitions. Source pose status names such as `accepted` are retained from the earlier motion experiment; they do not establish final owner approval of this new composition. SZ and FV remain `review_only`; U-to-CHSHR and its inverse remain blocked by mesh validation. The existing phonetic defects of SZ/FV were not repaired by substituting extraction masks.

Visual inspection found the EI endpoint coherent and the A/EI midpoint usable for owner comparison; there was no obvious crop jump in those inspected views. This is a limited visual assessment, not proof that every interpolated frame is artifact-free. The full mask overlay, tooth rows, preserved tongue masks, cleaned surfaces, endpoints, contact sheet, source snapshots and export settings remain in `.tools/argon-composition-01/package/` and its preparation history. Motion and phonetic approval remain with the owner.

## Verification

Processor tests: 94 passed, with the existing Starlette test-client deprecation warning. Added checks exercise overlapping antialiased regions, ambiguous opposing enamel, unchanged protected tongue pixels, exact-source binding and offline replay. Vue type checking and production build passed. All 44 Chromium/WebKit desktop/mobile checks passed without retries for masks, fixed tooth texture, exact endpoints, outside-mask pixel preservation, representative transitions, reduced motion, resource failure and unsafe-pose blocking. Mobile browser emulation does not replace required real-iPhone launch verification.

The final `audit.json` verifies 75 protected source/model-evidence files unchanged, source/artifact hashes, runtime equality with the prepared package, byte-identical tooth RGB and lossless exports, zero edits outside each pose mask, retained tongue protection and no configured secret in evidence or runtime. `browser-test-results.json` preserves the browser report and `browser-review.png` captures A-to-EI at 50%. Manual in-app inspection covered A/EI endpoints and midpoint, then left the A/EI loop playing with a 300 ms transition and 1200 ms holds. No cloud resources, credentials, new generation endpoints or speech integration are introduced.

The owner subsequently reported dissolving teeth and requested removal of review-only restrictions. [Opaque dental motion](dental-motion-fix.md) supersedes this package in the active browser, preserving its Argon source masks and all records. SZ/FV can now be compared in motion without implying phonetic acceptance.
