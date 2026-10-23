# Jaw spacing calibration

Date: 2026-10-19. The owner reported unnatural tooth spacing, illustrating the SZ endpoint. This offline adjustment reuses the current Argon masks and every cached pose without image generation or provider calls.

## Finding

The previous lower-row motion followed only the lower inner-lip landmark relative to EI. Whole-mouth vertical movement could therefore be mistaken for jaw opening. Its collision stop used just 0.4% of mouth width, allowing SZ's opposing enamel edges to approach within one aligned-crop pixel. This prevented intersection but did not provide useful visual clearance.

The original/current contact sheet and candidate offsets are retained in `.tools/dental-spacing-03/original-current.png` and `sz-candidates.png`. They support this diagnosis of inter-row placement, not a claim that all tooth appearance or phonetic issues are resolved.

## Change

`preparation/jaw.py` version `jaw-clearance/1` derives rigid lower translation from the difference between target and reference lip aperture: `(lower - upper)target - (lower - upper)reference`. Whole-mouth vertical translation does not affect that measurement. It then applies the existing per-opposing-column enamel stop with a separate recorded visual-clearance setting. Reference EI remains at zero translation; incompatible settings that would force the reference to move are rejected.

[Calibration](../../characters/syntelle/dental-jaw-calibration.json) selects minimum clearance of 1.75% of the reference mouth width. For this character that is 4.2115 crop pixels; integer translation leaves at least 5 pixels between the opposing enamel rows. The 2.5% candidate was also rendered and retained in preparation history, but hid too much of the narrow pose's lower row during visual comparison. The chosen setting is a local visual calibration, not validated universal anatomy.

| Pose | Previous lower translation Y | Current lower translation Y |
| --- | ---: | ---: |
| EI | 0 | 0 |
| A | 4 | 2 |
| SZ | -11 | -7 |
| CHSHR | -9 | -7 |

Upper teeth stay fixed. Canonical tooth PNG/WebP exports, their colors and shape remain identical to the prior package. Only rigid lower-row placement changes. The opaque geometric exposure renderer, pose-specific cavity/tongue protection, outside-mask preservation and geometry-only experimental playback policy remain intact. No review-only blocking is reintroduced.

## Reproduce and inspect

The active package is `.tools/dental-spacing-03/package/`, using [pose settings](../../characters/syntelle/viseme-spacing-review.json). The previous installed runtime is backed up alongside it. From `services/processor`:

```powershell
uv run --frozen --extra preparation python -m app.prepare_viseme_package --review ../../docs/characters/syntelle/viseme-spacing-review.json --output ../../.tools/dental-spacing-03/package --install-trial
```

Package pipeline is `oral-layers/7-jaw`, renderer remains `pose-morph/3`. The manifest records the reference aperture, each pose's aperture and proposed/applied offsets, per-column reference clearance, calibration hash and source snapshots. `spacing-comparison.png` shows previous/current A, SZ and CHSHR crops; browser screenshots and the final audit are stored separately from the hashed package. All original generation/analysis prompts and settings are unchanged.

## Verification and limits

All 96 processor tests passed, with the existing Starlette test-client deprecation warning. New tests verify independence from whole-mouth translation, exact reference identity, interpolated clearance and rejection of nonfinite/incompatible settings. Vue type checking and production build passed. All 48 Chromium/WebKit desktop/mobile checks passed on the final calibration, covering exact endpoints, fixed upper RGB, opaque transitions, closed/rounded occlusion, experimental SZ/FV playback and invalid-mesh blocking. Browser mobile emulation does not replace real-iPhone launch verification.

The final audit verifies 75 protected source/model-evidence files unchanged, source/artifact/runtime hashes, identical base and canonical tooth exports, an unchanged EI endpoint, identical cleaned surfaces in all nine poses and retained tongue protection. No configured secret is present in evidence or runtime. `browser-review.png` records the updated SZ endpoint and `browser-test-results.json` preserves the final report. Manual in-app inspection used EI-to-SZ and the SZ endpoint; the comparison remains available without review-only approval gating.

This improves inter-row spacing for the inspected character; it does not validate arbitrary uploaded dentitions or fix the previously noted FV phonetic articulation. The owner can inspect all geometrically valid experimental poses in motion. U/CHSHR's two invalid direct meshes remain blocked.
