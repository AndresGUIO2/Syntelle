# Resting smile and restrained laughter correction

The owner rejected the phase-67 laughing face as frightening and identified an
unwanted replacement of the existing manual smile. The cached Ava laughter audio
was explicitly retained. The new local candidate restores the original registered
closed smile at rest and prepares a separate, restrained boundary chuckle.

## Selected resources

- Native assets: `.tools/laughter-face-74/package-v6/`.
- Expression preparation: `.tools/laughter-face-74/candidate-v6/`.
- Private cached capabilities/audio: `.tools/laughter-face-74/active-profile/`.
- Native image: `syntelle-render-native:laughter-face-74-v6`; local launcher
  alias `syntelle-render-native:local` points to the same image.
- Previous phase-67 package and all phase-74 candidates remain available.

The original `registered-closed-smile/1` field and material are preserved byte for
byte. The renderer selects this original branch when resting, including manual
slider changes. Joint smiling speech stays separate. Existing speech geometry,
smile skin frames, original foreground and canonical dental textures are retained.

Laughter uses the registered resting smile's cheek/corner appearance with a small
oral opening measured from the cached laughing reference. The common restrained
profile uses 35% of that observed aperture. Lip appearance is normalized in the
same neutral geometry as the existing registered smile. It does not introduce the
generated laughing reference's teeth or dissolve between two independent seams.
Opaque canonical dentition remains sampled independently of lip deformation.

The `boundary-laugh/1` companion prepares only `Rest-A`, limited to articulation
0–0.06. Its geometry is checked over 405 combinations of jaw release, expression
strength and smile/laughter blending. Runtime rejects other poses or larger jaw
releases. A 20 ms smoothed audio-energy envelope drives small jaw releases on the
existing emitted-audio clock, replacing speech phonemes during laughter. Natural
blinks retain their own clock; laughter lifts the lower eyelid within a measured
bound rather than imitating a partial blink. Pause, interruption, reduced motion,
the automatic-expression toggle and existing conversational cooldowns remain.
Direct preview reuses the accepted cached audio and bypasses the LLM.

The stronger reference-derived candidates were internally rejected: v1 failed the
inverse envelope, v2 introduced a chin contour and sleepy eyes, v3 hid too much
upper dentition, v4 exaggerated neutral lips, and v5's received lip material
remained visually incoherent. Their outputs, diagnostic renders and sources stay
in `.tools/laughter-face-74/`. Numerical validity was not treated as visual
acceptance. No image, analysis or speech request was made, and no budget was reset.

## Offline preparation

From `services/processor`, using the existing virtual environment:

```powershell
.venv/Scripts/python.exe -m app.workflows.motion.prepare_laughter `
  --manifest ../../docs/characters/joint-expression-inputs.json `
  --expressions ../../.tools/expression-controls-75/runtime-expressions `
  --cached-landmarks ../../.tools/expressive-performance-67/joint-candidate-v12/laugh/landmarks.json `
  --output ../../.tools/laughter-face-74/replay-new

.venv/Scripts/python.exe -m app.workflows.rendering.export_native `
  --expressions ../../.tools/laughter-face-74/replay-new `
  --output ../../.tools/laughter-face-74/replay-native-new
```

Output directories must be new. Inputs remain source-bound; there are no manual
character coordinates, mask edits or placement overrides. This is a reusable
preparation method exercised with Syntelle, not proof of arbitrary-upload readiness.

## Controls and verification

The customization panel groups continuous smile/brow sliders with percentages,
cached laughter playback and a separate head-gesture selector/intensity/play
control. Plain labels replace `Tilt`, `Nod` and `Shake`. Head previews use a
0.72-second single nod or 1.12-second double nod, with intensity bounded by the
existing prepared rotation range. Version-2 conversational nods use the same
shorter duration. Existing minimum spacing and gesture allowance are unchanged.
The panel leaves the character visible during inspection; mobile height is bounded.

Focused processor, animation, media, adapter, renderer and API tests pass. A pixel
comparison at 0%, 50%, 100% and return transitions matches the previous registered
resting-smile renderer exactly. Chromium desktop and WebKit mobile control tests
pass. The first failed browser checks are retained; corrections fix range accessible
names and a test-text encoding issue. Type checking, scoped lint and the player
build pass. Actual CUDA/NVENC playback completed two cached laughter previews with
audio RMS above 0.23, no errors and no chat or synthesis command. Normal-size
screenshots and a sampled recorded sequence were internally inspected.

Evidence includes `live.json`, `laugh-live.png`, `restored-smile-live.png`, the
recorded browser video and `live-sequence.jpg`. PostgreSQL snapshots and byte-checked
restoration are recorded separately in `.tools/laughter-face-74-archive.json` and
`.tools/expression-controls-75-archive.json`. Owner visual acceptance remains
separate; this change does not establish physical-device, strict performance or
public-upload approval.
