# Stronger shake and shoulder tilt

The owner accepted the phase-78 nod but found shake and shoulder tilt too weak.
The new shake is one complete center-left-right-center primitive over 0.92 seconds,
with eased direction changes and full prepared amplitude on both sides. Previews
and new phrase direction accept 1–8 repetitions. Each cycle spends one movement
from the shared eight-per-reply allowance, reserves its entire interval, and
preserves the existing spacing and interruption rules. Speech semantics still
choose intended disagreement; audio beats never request a shake.

The previous windowed shake remains readable for historical direction without a
repetition field. New resolved shake sequences expand into private `shake_cycle`
instances; the model and controls use only `shake`. Their normalized intensity
uses the selected character's prepared yaw bound, matching its preview. The nod
curve, 0.62-second cycle and existing pitch range remain unchanged.

Tilt uses the same single rise-and-return curve with a four-times-larger prepared
roll range: 1.2 degrees rather than 0.3. Its maximum is 1.14 degrees at 95% of the
prepared bound. New direction uses that character's roll limit. Tilt remains a
single gesture and retains its descriptive shoulder label.

## Preparation and limits

The generic `prepare_gesture_envelope` now accepts optional yaw/roll bounds in
addition to pitch. It validates the full combined box with existing geometry,
using the same minimum-area threshold and 81 angle/breath configurations.
The selected box is yaw 3, pitch 4, roll 1.2 degrees, with minimum area ratio
0.2593 and no clipped vertices for Syntelle. The same box passes cafe/woodland
cached geometry. Wider failed candidates remain in `envelope-candidates*.json`;
their thresholds were not relaxed and they were not installed.

From `services/processor`, choose fresh output directories:

```powershell
.venv/Scripts/python.exe -X utf8 -m app.workflows.motion.prepare_gesture_envelope `
  --bundle ../../.tools/laugh-release-79/prepared/resource-bundle.json `
  --roll 1.2 --output ../../.tools/head-gestures-80/replay-prepared

.venv/Scripts/python.exe -X utf8 -m app.workflows.rendering.export_native `
  --bundle ../../.tools/head-gestures-80/replay-prepared/resource-bundle.json `
  --output ../../.tools/head-gestures-80/replay-package
```

Active resources are `.tools/head-gestures-80/package/`, with hash
`9bce8e072ef249942f4940fc8556b306cccba6d2371232957e43444addaafe5a`,
private `active-profile/profile.json`, and native image
`syntelle-render-native:head-gestures-80-final`. Rollback settings and phase-79
resources/image remain. All 118 texture hashes match phase 79: original manual
smile, full-face laughter, lip-contact shadow and dental materials are preserved.
No character coordinates, masks, paid calls or public endpoint are added.

## Bounded verification

Focused processor/Go checks cover bounds, intensity, complete bilateral travel,
repetitions, shared policy and archived shake interpretation. Existing nod,
pause/interruption and audio-clock checks pass. Type checking and scoped lint
pass. Isolated WebKit controls emit the correct shake count/intensity and hide
repetitions for tilt. Native/API/Web images build successfully.

Actual Chromium CUDA/NVENC playback exercises three shake cycles, shoulder tilt,
the preserved nod and cached laughter without model or speech requests. Received
video measurements identify three peaks in each shake direction, approximately
17.9 pixels of horizontal dark-feature travel and 2 pixels of bilateral eye-line
height variation during tilt at the 720-pixel native output width. These are
feature measurements, not anatomical rotation estimates. Normal-size frames,
video and source-bound preparation reports remain in `.tools/head-gestures-80/`.
The initial receiver timeout happened before the worker connected and is retained
separately. Owner visual acceptance and physical-device review remain pending.
