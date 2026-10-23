# One repeated nod primitive

The [shake and shoulder-tilt correction](shake-and-tilt.md) preserves this nod
while adding complete repeated shake cycles and a wider validated roll range.

The owner rejected the separate single/double-nod controls and the weak motion.
New direction and previews now use `nod` with an integer `repetitions` count from
1 to 8 and the existing normalized intensity. Each cycle lowers and returns the
head using the same curve, lasting 0.62 seconds. `double_nod` remains readable for
historical recordings but is absent from the new prompt, schema, capability
advertisement and controls.

Each repetition spends one movement from the existing eight-per-reply allowance.
A sequence reserves its entire interval. Separate head cues retain the 2.2-second
minimum start spacing and cannot overlap a running sequence. Audio-derived beats
spend the same allowance. A sequence that cannot fit its phrase is omitted while
preserving speech. The model chooses contextual expression independently of
chat commands; direct inspection stays in the preview controls.

## Prepared range

`app.workflows.motion.prepare_gesture_envelope` accepts an explicit character
resource bundle and checks its immutable portrait/resource bindings and existing
projection geometry checksum. It validates the requested pitch across 81 angle
and breathing configurations, preserves failed evidence, and refuses automatic
fallback or overwrite. It adds no character coordinates or masks and makes no
provider calls. Linked gaze, manual and reference-laughter metadata are rebound
to the new motion identity without changing their image resources.

The active Syntelle candidate uses a 4-degree prepared pitch bound and a
3.8-degree maximum preview/directed nod, replacing the previous 1.14-degree
preview. Idle posture, yaw/roll bounds, breathing, manual smile, laughter and
dental textures are unchanged. All 118 exported texture hashes match phase 76.
Geometry has minimum projected triangle ratio 0.3034 and zero clipped vertices.
The same cached 4-degree geometry check also passes cafe and woodland; these
checks do not constitute playback or visual acceptance for those characters.
This remains a bounded portrait projection, not reconstructed hidden anatomy.

From `services/processor`, choose fresh output directories:

```powershell
.venv/Scripts/python.exe -X utf8 -m app.workflows.motion.prepare_gesture_envelope `
  --bundle ../../.tools/reference-laughter-76/resource-bundle-final.json `
  --output ../../.tools/repeated-nod-78/replay-prepared

.venv/Scripts/python.exe -X utf8 -m app.workflows.rendering.export_native `
  --bundle ../../.tools/repeated-nod-78/replay-prepared/resource-bundle.json `
  --output ../../.tools/repeated-nod-78/replay-package
```

## Activation and verification

Active resources: `.tools/repeated-nod-78/package/`, prepared from
`prepared-v2/resource-bundle.json`, with private cached voice profile
`active-profile/profile.json`. Package hash:
`364a4345d6c71954a8a86c5078a027c2904ccf77adb3ed3064b953ada74af54c`.
Native image: `syntelle-render-native:repeated-nod-78`.
Rollback settings are in `runtime-before.json`; phase-76 resources/image remain.

Focused Go tests cover phrase decoding, malformed optional counts, shared reply
allowances, sequence overlap, native single-curve expansion, preview intensity,
baseline return and interruption. Processor checks cover bounded preparation,
metadata binding and preserved material references. Type checking and scoped
frontend lint pass. Native/API image builds run their existing focused checks.

Actual CUDA/NVENC Chromium playback successfully previews 7, 4 and 1 nods using
application controls without model or speech requests. Received video measurements
identify seven complete peaks in the seven-nod sequence; cached laughter preview
still completes. Evidence is in `live-review.json`, `video-motion-review.json`,
normal-size screenshots and `live-video/`. The preliminary companion without
nested reference metadata rebinding stays in `prepared/`; only `prepared-v2/`
is exported and active. Owner visual acceptance and physical-device validation
remain separate from these checks. No new paid requests are included.

Windows WebKit loads and verifies the unified controls. Its separate 20-second
live receiver attempt did not produce a video frame and is retained in
`webkit-review.log`; no WebKit playback pass is claimed for this change.
