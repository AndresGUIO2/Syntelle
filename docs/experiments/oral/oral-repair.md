# Shared dental material and FV correction

The subsequent [multi-character development report](automatic-oral-corpus.md) records the newer reusable lip-contact correction, two generated development characters, PostgreSQL request audit and remaining visual failures. The results below describe the preceding single-character repair.

## Result and scope

The local trial now defaults to the corrected semantic package, also available explicitly at `http://localhost:5173/?trial=viseme-a&oral=semantic`. `oral=completed` and `oral=previous` retain the earlier packages. This is an improved single-character development demonstration, not completed public-upload preparation or owner acceptance.

The owner authorized the proposed two-image correction on October 19, 2026. Exactly two Nano Banana requests were made, sequentially, with no retries: one lower dental completion and one FV articulation, using `gemini-nano-banana-2.1` at 1K. Both include the exact immutable original portrait bytes. The provider's supported JPEG response format was used; all working material and browser exports are lossless PNG/WebP. The original portrait, eight preceding visemes, old completion and rejected experiments remain intact.

**The new lower completion was rejected.** It repeated the horizontal strip/second-crown-body defect. Its generated image and visual rejection record remain in `.tools/oral-repair-07/lower/`; it is not installed. The current shared material instead continues the actual selected EI fragments away from their incisal edges into hidden attachment coverage. This deterministic method preserves selected RGB, incisal silhouettes and opacity. It avoids pasting unrelated generated crowns underneath them. It is conservative 2D texture continuation, not a fully reconstructed anatomical denture; large newly exposed crown surfaces remain a quality limitation.

The new FV source visibly conceals the lower teeth and brings the lower lip to the upper incisors. One additional Argon semantic analysis measured its lip opening and upper teeth; the remaining eight analyses were reused unchanged. Across the original semantic batch and this correction there are ten actual analysis requests, not nineteen; copied caches do not represent new calls. Original FV remains preserved in `.tools/visemes-01/fv/`.

## General algorithm changes

After stable face alignment, a generated mouth can still drift relative to the fixed upper arch. The compositor previously removed generated teeth but left lips in that drifting frame, exposing an artificial void beneath the shared incisors. Preparation now registers the entire local mouth patch and all its semantic masks to the observed incisal edge of the shared arch. The shift is derived from reliable overlapping observations, bounded by reference width, and recorded per pose. It does not enlarge, scale or reshape the opening. Clipping and excessive drift are rejected. There are no pose-name placement offsets or manually repaired masks in this path.

The upper arch stays fixed in character coordinates. The lower arch remains one rigid material layer fitted to observed articulation. Both are opaque; occlusion comes from the measured lips, and shadows affect RGB independently of alpha. Source tongue pixels remain protected during tooth removal. The browser uses the same compositor for endpoints, transitions and exports.

The visible preview does not call an image model. AI is used only during resource preparation and contour analysis.

## Evidence and verification

All preparation records are in `.tools/oral-repair-07/`: final prompts, exact request payloads/responses, original outputs, input hashes, usage, timestamps, original fragments, registered material, semantic masks, source-bound manifests and algorithm snapshots. Credentials remain in local secret configuration and are excluded from these records and browser assets.

The new source/material manifests are `docs/characters/syntelle/oral-input-repair.json` and `oral-material-repair.json`. The installed package build is recorded in its manifest; preceding semantic builds remain under `.tools/semantic-oral-06/`, and staged repair rebuilds are archived by build ID.

The nine endpoints and all 72 directed transitions were inspected in six contact sheets with 25%, 35%, 41% and 75% frames. This includes EI/FV at 35%, FV/A at 41%, EI/SZ, A/EI, O/U and MBP/A. The local correction removes the previous angular CHSHR cavity and mismatched lower crown body, and restores FV contact without exposing the lower row. This internal comparative review is not owner phonetic acceptance or proof of quality on unseen characters.

Browser export checks 729 frames across 81 pairs, including self-pairs, with exact per-engine endpoint agreement and zero alpha outside the edit-mask union. Processor tests independently cover preserved observed pixels, connected attachment coverage, dark opaque enamel, bounded dental registration, unchanged aperture shape, rejected excessive drift, cached-output integrity and provider-failure stopping.

The processor suite passes 124 tests. Frontend type checking and production build pass. All 28 Chromium and WebKit desktop/mobile checks pass and are recorded in `browser-test-results.json`. Cross-engine exported fallback comparison permits at most two 8-bit levels in at most 32 channels across all nine endpoints: diagnosis found 16 shaded edge channels in SZ/CHSHR differed by two levels on WebKit. Per-engine transition endpoints remain exact; unshaded opaque tooth texture and observed lip checks retain their tighter limits. No screenshot baselines were updated to approve the changed appearance.

## Reproduction without paid requests

From `services/processor`, reproduce shared hidden coverage:

```powershell
uv run --frozen --extra preparation python -m app.preparation.dental_coverage --upper-fragment ../../.tools/dental-spacing-03/package/upper-teeth.png --lower-fragment ../../.tools/dental-spacing-03/package/lower-teeth.png --output ../../.tools/oral-repair-07/material
uv run --frozen --extra preparation python -m app.prepare_semantic_oral --manifest ../../docs/characters/syntelle/oral-input-repair.json --material ../../docs/characters/syntelle/oral-material-repair.json --semantics ../../.tools/oral-repair-07/semantics --output ../../.tools/oral-repair-07/package
```

From `apps/web`, with Vite running:

```powershell
node scripts/export-oral-review.mjs --oral semantic --package ../../.tools/oral-repair-07/package --output ../../.tools/oral-repair-07/browser-review
```

Then from `services/processor`:

```powershell
uv run --frozen --extra preparation python -m app.prepare_semantic_oral --manifest ../../docs/characters/syntelle/oral-input-repair.json --material ../../docs/characters/syntelle/oral-material-repair.json --semantics ../../.tools/oral-repair-07/semantics --output ../../.tools/oral-repair-07/package --finalize ../../.tools/oral-repair-07/browser-review --install ../../apps/web/src/experiments/viseme-a/semantic
uv run --frozen --extra preparation pytest -q
```

`app.oral_repair` retains the bounded request workflow and refuses retries or changed cached outputs. The authorized two-image allowance is exhausted; do not start another repair output directory to bypass it.

## Remaining boundaries

Five-character preparation and held-out validation remain incomplete. The additional four source portraits are preserved, but their visemes/materials have not been generated; blue also failed pinned MediaPipe detection. No new-character generation, infrastructure, public endpoint, speech, depth or blink work was added here. The current generic operations need multi-character evidence before claiming automatic visitor preparation.
