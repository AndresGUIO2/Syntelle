# Full-face reference laughter and shared dentition

The latest optional release correction is [lip-contact lighting](laughter-contact-lighting.md).
It preserves the phase-78 nod behavior, manual smile and all source texture bytes.

The owner rejected the phase-74 restrained chuckle because it reused the manual
smile instead of reproducing the supplied laughing reference. This preparation
keeps that rejected package and introduces an independent `reference-laughter/1`
extension. The existing registered manual smile is preserved byte for byte.

## Preparation

The reference stage accepts explicit immutable portrait, reference, cached
landmark, oral, motion, foreground and manual-expression inputs. Its algorithms
contain MediaPipe topology identifiers, never character-specific pixel coordinates
or placement overrides. Syntelle's reproducible inputs are recorded in
`docs/characters/reference-laughter-inputs.json`. Other characters use the same
workflow with their own source-bound references.

Stable facial anchors register the received laughing reference once. A common
facial mesh carries cheeks, eyelids, eye corners, nasolabial tissue and lip
contours at full measured amplitude. Fixed connectivity is validated across 25
steps and used when rasterizing both appearances into the same intermediate
geometry. An automatically derived inward facial boundary protects existing
foreground pixels. No amplitude reduction is used to disguise invalid topology.

The face sequence has its own clock parameter, independent of manual smile.
Reference iris pixels and enamel are excluded from its skin appearance. Original
rigid iris/sclera samples remain behind expression-aware eyelids; cached closed
eyelid material supplies live blink closure. The eye overlay owns only its
aperture and closure sweep, preserving the received creases and cheek tissue.

Dental preparation uses a curved occlusal seam and separate seeded row regions
inside the observed mouth. Shadowed enamel is segmented as material, never made
transparent using brightness. Extracted enamel RGB remains unchanged after the
single recorded reference registration. Teeth are reused opaquely for speech,
smiling speech and laughter, with rigid placements and geometric lip occlusion.
No dental color-normalization, per-pose scaling or alternative tooth identity is
introduced. The existing manual resting smile retains its original rendering.

Hidden attachment continuation is recorded separately from observed crowns.
Preparation audits all endpoints and directed transitions for unsupported exposed
attachment pixels, row overlap, closed-lip visibility and FV contact. Bounded
shared arch calibration applies to every articulation; it is not a collection
of hand-positioned pose corrections. A naturally occluded lower row in a rounded
vowel is reported as a visibility change rather than replaced with old teeth.

## Reproduction and contracts

Run the following from `services/processor`, choosing fresh output directories:

```powershell
.venv/Scripts/python.exe -X utf8 -m app.workflows.oral.prepare_reference_dentition `
  --manifest ../../docs/characters/reference-laughter-inputs.json `
  --cached-landmarks ../../.tools/expressive-performance-67/joint-candidate-v12/laugh/landmarks.json `
  --output ../../.tools/reference-laughter-76/replay-dentition

.venv/Scripts/python.exe -X utf8 -m app.workflows.motion.prepare_reference_laughter `
  --manifest ../../docs/characters/reference-laughter-inputs.json `
  --expressions ../../.tools/expression-controls-75/runtime-expressions `
  --cached-landmarks ../../.tools/expressive-performance-67/joint-candidate-v12/laugh/landmarks.json `
  --dentition ../../.tools/reference-laughter-76/replay-dentition `
  --output ../../.tools/reference-laughter-76/replay-expressions
```

Native export accepts a `character-resource-bundle/1` JSON using explicit `oral`,
`motion`, `motion_assets`, `gaze`, `expressions` and `backgrounds` resources plus
the immutable portrait hash. These remove Syntelle-specific asset discovery from
the new export path; historical arguments retain compatibility. Updated oral
bindings require a motion metadata companion referencing the new oral build,
without regenerating existing projection geometry, eyes or foreground assets.

The optional expression extension provides 25 face textures, eye profiles, oral
contours, a separate cavity texture and measured jaw limits. CPU, CUDA and local
development rendering consume the same resources. New resources join existing
immutable GPU sharing; character operations remain private in the active server
renderer. The public player receives no preparation resources.

The private performance profile enables laughter only with matching package
bindings, valid dental coverage, composition/playback review and approved cached
voice audio. Laughter capability is independent of smile capability. The existing
phrase protocol, direct preview, cached Ava audio, interruption and conversational
allowances remain in place. Packages without the extension keep historical
interpretation.

## Active local package and jaw correction

The selected assets are `.tools/reference-laughter-76/package-final/`, prepared
from `dental-v7/` and `expressions-jaw-v2/`. The explicit source bundle is
`resource-bundle-final.json`; private audio/capabilities are in `active-profile/`.
The running image is `syntelle-render-native:reference-laughter-76-final`, also
tagged as the local launcher default. Previous phase-74 resources remain intact.

The first full-face candidate incorrectly fixed the complete face oval, including
the chin. The corrected mesh retains the reference's measured 30.43-pixel chin
descent and 36.05-pixel lower-lip descent on the immutable source canvas. Static
outer support and an automatically measured neck transition attach this moving
jaw to the remaining portrait. Audio jaw releases move the lower lip, lower
dental row and chin together; the smile slider does not drive that motion.

The incorrect landmark-based lower dental translation was removed: the reference
already supplies its observed arch placement. Lip geometry reveals that arch;
additional audio jaw movement opens the lower row. Reference-specific crown views
use the same canonical enamel RGB with measured gingiva/lip ownership. This keeps
hidden attachment proposals from painting over observed gum tissue. Full shared
materials remain available to speech articulation. The old uniform dark mouth
placeholder and padded enamel removal are removed; registered contact tissue
outside the actual enamel masks remains byte exact.

The manual smile is pixel identical to the preceding installed package at 0%,
50% and 100%, including restoration after laughter. Nine raw CPU/CUDA fixtures
are byte identical. Focused processor tests, native animation/renderer/adapter/media
checks, type checking and scoped lint pass. The full player build is blocked by
the separately added public `characters/syntelle-poster.jpg`, which is absent from
the current public-output allowlist. Both failed build logs remain preserved;
this change does not loosen that boundary or alter the poster. Chromium and WebKit
development captures pass, including expression-aware blinking. Actual installed
CUDA/NVENC playback completes two cached audio previews with no chat/synthesis
commands and measured received audio energy; the recorded sequence was inspected
at normal size. The first startup harness timeout is retained: the current player
requires an explicit Start action, which the corrected harness now performs.

The requested further nod amplitude adjustment remains pending and is not part
of this selection.

## Evidence and acceptance

Development evidence is preserved in `.tools/reference-laughter-76/`, including
source snapshots, rejected extractions, intermediate exports and rendered review
frames. This change makes zero image, analysis or speech provider requests and
does not alter existing generation budgets. Future reference generation still
requires its own authorized, PostgreSQL-recorded budget.

Numerical validation and internal playback inspection are separate from owner
visual acceptance. Generic synthetic/cached-character checks establish reusable
implementation behavior, not successful arbitrary-upload preparation. New café
and woodland laughing references, physical-device approval and cloud deployment
are outside this delivery. Final selected resources, check results and verified
PostgreSQL restoration are recorded in the run's selection/archive records.
