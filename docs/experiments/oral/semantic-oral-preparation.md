# Automatic semantic oral preparation

## Current status

**Superseded local trial:** the owner authorized two image corrections, and the current implementation/default trial is documented in [Shared dental material and FV correction](oral-repair.md). The sections below preserve the original semantic experiment and its failures. Use the repair manifests for the current browser candidate; do not reinstall this historical FV/material combination as the latest result.

The development candidate is available at `http://localhost:5173/?trial=viseme-a&oral=semantic`. The preceding completed package remains at `oral=completed`; it has not been replaced. The candidate is **not visually accepted or ready for publication**.

Nine source-bound Argon analyses are cached in `.tools/semantic-oral-06/`: Rest, A, EI, O, U, MBP, SZ, FV and CHSHR. Exactly nine requests succeeded, consuming 37,108 total tokens (25,318 input, 11,242 output and 548 thought tokens). The estimated analysis cost is $0.168536 at the [published introductory Argon rates](https://ai.google.dev/gemini-api/docs/pricing), checked October 19, 2026; this is a usage estimate, not a billing statement. No viseme or dental image generation was performed in this experiment. Replaying analysis without `--request` verified cache reuse. The batch limit is eighteen attempts, including failures, with at most one semantic correction per pose and no transport retries.

CHSHR's observed opening differs substantially from MediaPipe's estimated inner contour. The new composition uses the observed lip boundary and detects both dental rows without the old fixed 14-pixel lower-row displacement. This removes the previous enlarged angular cavity, but does not establish owner acceptance of the character or all motion.

FV's original generation exposes both dental rows and lacks upper-incisor/lower-lip contact. The pipeline records both failures and the browser displays an explicit preparation failure. It does not relabel this as a correct FV or retarget the lips to conceal the source failure. The existing lower completion also has a visible selected/generated material seam when raised into narrow poses. Its visual quality remains unresolved; technical checks must not promote it automatically.

## Inputs and implementation

The preparation command accepts an immutable-original/cached-generation manifest and a separate registered-material manifest. Character coordinates and asset paths are data; the implementation contains no Syntelle-specific paths, removal polygons, lip-opening targets or pose-specific jaw offsets.

MediaPipe provides localization and stable alignment. Argon receives enlarged clean original crops and coordinate grids, and returns outer lips, the actual opening, visible upper/lower teeth and tongue. Empty regions are valid. Contours are checked for topology and ownership, supersampled, and optionally snapped at most one native pixel to a stronger local image edge. Raw model polygons are retained unchanged.

The compositor uses semantic signed-distance visibility rather than MediaPipe's inner lip polygon. Closed-pose geometry has a finite-area scaffold for mesh validation but no visible opening. A single shared upper registration is fitted from broad incisal observations; the lower arch translates rigidly from observed lower incisal edges, with a bite stop. When no lower teeth are visible, hidden coverage is derived from the lower lip boundary. No individual teeth move to fill gaps.

Tooth removal stays inside the measured opening. Photographic cavity donors supply hidden background, while visible tongue pixels and all pixels outside removal remain unchanged. Dental opacity comes from the supplied material coverage; luminance is used only for cavity donor selection, never dental alpha. Shadows affect color independently of opacity.

Raw generations, analysis requests/responses, rejected results and preceding packages remain separate from corrected compositions. The package records original hashes, source transforms, semantic provenance, rigid placements, material hashes, dependency versions, export settings and a snapshot/hash of preparation and rendering code. The old A generation predates per-request input hashes; its existing original-source hash and unchanged-source verification are preserved rather than inventing a missing request record.

This command requires existing registered dental assets and cached pose generations. It does not implement the complete visitor-upload generation workflow or automatically complete a new character's dentition from its portrait.

## Reproduction

From `services/processor`, replay all cached analyses without paid calls:

```powershell
uv run --frozen --extra preparation python -m app.oral_semantics --manifest ../../docs/characters/syntelle/oral-input.json --output ../../.tools/semantic-oral-06 --poses Rest A EI O U MBP SZ FV CHSHR
```

Stage the candidate using the generic manifests:

```powershell
uv run --frozen --extra preparation python -m app.prepare_semantic_oral --manifest ../../docs/characters/syntelle/oral-input.json --material ../../docs/characters/syntelle/oral-material.json --semantics ../../.tools/semantic-oral-06 --output ../../.tools/semantic-oral-06/package
```

With Vite running, from `apps/web`:

```powershell
node scripts/export-oral-review.mjs --oral semantic --package ../../.tools/semantic-oral-06/package --output ../../.tools/semantic-oral-06/browser-review
```

Then from `services/processor`:

```powershell
uv run --frozen --extra preparation python -m app.prepare_semantic_oral --manifest ../../docs/characters/syntelle/oral-input.json --material ../../docs/characters/syntelle/oral-material.json --semantics ../../.tools/semantic-oral-06 --output ../../.tools/semantic-oral-06/package --finalize ../../.tools/semantic-oral-06/browser-review --install ../../apps/web/src/experiments/viseme-a/semantic
uv run --frozen --extra preparation pytest -q
uv run --frozen --extra preparation python -m app.oral_benchmark --corpus ../../docs/characters/oral-corpus.json --output ../../.tools/semantic-oral-06/corpus-report.json
```

Finalization uses straight RGBA read from the live compositor, checked against saved hashes, to avoid an extra browser PNG-encoding round trip at nearly transparent pixels. Lossless PNG and WebP exports are retained; endpoint fallbacks use PNG. The same renderer supplies endpoints, intermediate frames and authoritative exports. The exporter can read a staged package directly without replacing the active trial. Installation rejects unfinalized endpoints, so preliminary Pillow inspection images never become runtime fallbacks.

From `apps/web`, run `npm run build` and `npx playwright test tests/e2e/semantic-oral.spec.ts tests/e2e/completed-oral.spec.ts`. No visual baseline updates are needed for the independent semantic invariants. The pre-existing completed-package snapshots must remain unchanged.

## Corpus and remaining work

The owner supplied four additional portraits. Original bytes are preserved under `.tools/semantic-oral-06/corpus/`, with hashes and local preflight records. The corpus manifest assigns Syntelle, cafe and woodland to development, and blue and window to held-out evaluation. Do not tune the algorithm against held-out results and continue calling them unseen inputs.

Local preflight detected 478 landmarks on cafe, woodland and window. The pinned MediaPipe model did not detect a face on blue. That is a recorded failure, not a successful preparation or an excuse to count a rejected image as a positive visual result. No new characters' visemes or dental completions have been generated. The five-character benchmark therefore remains incomplete.

Before promotion: resolve lower-material seams and the incompatible FV source within an explicitly authorized generation budget; rerun all endpoints and directed transitions in slow playback; pass independent processor/browser checks; obtain visual acceptance; and complete the multi-character preparation and held-out evaluation. Public preparation, speech and real-device launch verification remain unfinished.
