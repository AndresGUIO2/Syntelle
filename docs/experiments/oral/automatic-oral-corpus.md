# Automatic oral preparation: development evidence

## Current result

The reusable path now prepares all nine poses for the cafe photograph and woodland render, and applies the same geometric compositor to the existing Syntelle resources. No manually drawn masks or character-specific placement coordinates were added. These are development candidates, not accepted public-upload preparation. The preceding Syntelle package remains the default trial.

Inspect the candidates locally:

- [Syntelle](http://localhost:5173/?trial=viseme-a&oral=semantic&character=syntelle)
- [Cafe](http://localhost:5173/?trial=viseme-a&oral=semantic&character=cafe)
- [Woodland](http://localhost:5173/?trial=viseme-a&oral=semantic&character=woodland)

These character routes and resources are development-only. The source/composition comparison uses the registered source generation, not the retargeted result mislabeled as an original. Existing playback, manual blend, magnification and comparison controls remain available.

## FV correction without another image request

The first FV sources for both new characters expose both dental rows. Their single authorized visual corrections also failed the articulation requirement. All four outputs and exact requests remain preserved; the failed corrections are not installed as successful generations.

`preparation/labiodental.py` instead derives a bounded local lower-lip warp from the measured inner/outer contours and the shared upper incisal silhouette. The upper arch stays fixed. The lower lip moves upward to that edge, occluding the lower row. The resulting opening must be a subset of the observed opening; excessive displacement fails preparation. The unchanged tongue/cavity remains behind the new lip occlusion. This is a general algorithm change to the preceding strict observed-shape policy, not a claim that Gemini generated a valid FV pose.

The method records the displacement for every image column, maximum movement, opening area before/after, fixed upper-arch status and algorithm version. Human inspection evaluates the result but does not repair masks or per-character placement.

## Shared material and remaining visual limitations

The new characters select a single observed upper/lower pair automatically: SZ for cafe and EI for woodland. Semantic subpixel contours are retained during extraction, rather than thresholding every tooth boundary into an opaque staircase. Dark enamel stays opaque; fractional alpha is confined to the geometric edge.

A bounded color gain estimated from the two selected rows reduces differently baked illumination before pose-dependent shadows. It preserves alpha and texture ordering; it is an illumination approximation, not recovered physical albedo. Raw fragments and gain parameters are preserved. Hidden attachment coverage is continued from observed enamel, without inventing a second crown body. Syntelle reuses its existing completed EI material as requested.

**Visual acceptance remains incomplete.** Enlarged development comparisons still show unnatural dental edge/texture behavior in some poses. A successful mask fit, contact check or endpoint invariant does not establish a usable character. The `ready` field in these experimental manifests reports the current numerical preparation gates, not owner acceptance or production readiness. Do not freeze or promote the pipeline solely from that field. The two held-out characters have not been prepared with this candidate, and no five-character success is claimed.

## Calls and persistence

The four-portrait budget remains 32 image requests and 36 analyses. Current actual usage is **18 image requests and 21 analyses**, all received with no transport retries. This includes the two FV corrections; cafe's corrected semantic proposal required one additional contour correction, so that correction phase consumed three analyses, not two. There are 14 image requests and 15 analyses left. No further paid call was made for the deterministic lip/contact and edge changes described here.

All generation requests used the exact original bytes for their character as the first image. PostgreSQL stores actual source/generated files, prompts, request/response configuration, masks, hashes, usage and immutable snapshots. `.tools/corpus-08/` is the working cache. Failed visual results remain separate from selected runtime resources. Credentials remain in ignored local configuration, outside evidence and browser assets.

Completing eight generated poses plus nine analyses for each of the two held-out characters would require 16 images and at least 18 analyses, exceeding what remains. Do not silently reset the budget, substitute missing poses or start another budget identity. Resolve development quality before requesting any additional allowance or freezing the pipeline.

## Verification

The processor suite passes 136 tests, including PostgreSQL-backed archive restoration and request limits, no aperture enlargement in FV, fixed upper material, dark enamel opacity, tongue protection and retained subpixel extraction. Frontend type checking and production build pass.

For each of the three development candidates, browser export covers 729 frames: nine endpoints and all 72 directed transitions at nine positions including 35% and 41%. Each export reports exact endpoint agreement and zero alpha outside the edit-mask union. `app.corpus_visual_review` derives character-independent close-ups from measured lips; its sheets explicitly begin with visual acceptance unset. They are inspection evidence, not self-approving screenshot baselines.

All 40 oral browser tests pass, including all three candidates in Chromium and WebKit at desktop and mobile sizes, exact per-engine endpoints, stable unshaded exposed enamel and the preceding package regressions. The first combined run had one WebKit-mobile screenshot failure during a concurrent frontend edit; its isolated rerun and the complete run on stable sources passed without baseline changes. See `.tools/corpus-08/browser-test-results.json`. Desktop/mobile emulation does not replace real-iPhone launch verification.

The independent offline request audit reconciles all 39 real request payloads with PostgreSQL bindings and checks that every image request contains the exact immutable portrait bytes first. It also verifies saved prompts, raw generation hashes, single attempts and the authorized caps. Its report is `.tools/corpus-08/request-audit.json`.

## Reproduce from cached resources

From `services/processor`:

```powershell
uv run --frozen --extra preparation python -m app.corpus_prepare --config ../../docs/characters/oral-validation-batch.json --output ../../.tools/corpus-08 --cases cafe woodland --phase compose
```

From `apps/web`, with Vite running, export each case separately:

```powershell
node scripts/export-oral-review.mjs --oral semantic --package ../../.tools/corpus-08/cafe/package --output ../../.tools/corpus-08/cafe/browser-review
```

Then finalize and archive through the processor runner:

```powershell
uv run --frozen --extra preparation python -m app.corpus_prepare --config ../../docs/characters/oral-validation-batch.json --output ../../.tools/corpus-08 --cases cafe woodland --phase finalize
uv run --frozen --extra preparation python -m app.corpus_visual_review --package ../../.tools/corpus-08/cafe/package --browser ../../.tools/corpus-08/cafe/browser-review
```

These commands make no paid requests. Finalizing exports does not grant visual approval or replace the default demo.
