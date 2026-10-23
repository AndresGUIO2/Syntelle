# Coordinated diagonal posture and viewer-focused gaze

The default demo now adds [conversational speaking attention](speaking-attention.md) during speech. The idle timeline and prepared eye resources described here remain unchanged.

The current eye compositor includes the [iris boundary correction](gaze-edge-composition.md). Its continuous supported-iris composition and edge unmatting supersede the earlier support-threshold rendering below. Prepared masks, gaze envelopes and behavior remain unchanged.

The user demo's preceding idle profile forced large horizontal target changes while retaining very small pitch and roll. It looked like a left/right sweep. The current local demo uses `viewer-attention/1`: one seeded timeline supplies diagonal head postures, short glances and eye counterrotation. All three characters use the same algorithm and settings. Existing laboratory contexts retain their previous behavior.

## Behavior and interfaces

| Behavior | Common setting |
| --- | --- |
| Head posture | Up to ±2.4° yaw, ±0.8° pitch, ±0.25° roll, within each existing projection envelope |
| Posture adjustment / hold | 3–6 seconds / 1–3 seconds, quintic easing |
| First adjustment | A diagonal target within the first six seconds |
| Glance interval | Seeded 8–14 seconds; most time remains viewer-focused |
| Eye move / hold / return | 80–140 / 350–800 / 120–220 ms |
| Occasional partial head follow | 35% of glance events; starts 150 ms after the eyes and eases over 1.8 seconds |
| Gaze envelope | At most 5% of eye width horizontally, 2% vertically; automatically reduced by source coverage |
| Rendering update precision | Eye offsets quantized toward zero in 0.05-source-pixel steps |

There is no forced minimum horizontal step in the new planner. Pitch and roll vary independently. Both eyes share a normalized gaze direction; the stricter eye's available coverage limits both. Head counterrotation maintains approximate eye contact rather than allowing the irises to remain rigidly painted onto a turning face. This is a conservative screen-space approximation, not measured binocular tracking.

Existing acoustic cue selection, blink frequency accounting, double blinks and breathing are unchanged. The attention director samples the existing speech gestures on the audio clock and smoothly reserves headroom for them inside the projection envelope. Starting/stopping speech blends the gesture residual over 600 ms without restarting posture. Natural movement controls head and gaze together. Reduced motion, pause, hidden-tab suspension and character disposal reuse the existing lifecycle.

`MotionContext.attention` is an optional, source-bound companion extension. It references the original image hash and frozen motion build; no frozen package IDs or geometry are rewritten. The catalog loads its resources only for the selected character. Trial components still own layout and controls; planning is in `features/character/animation/attention.ts`, eye composition in `rendering/eyeMorph.ts`, and resource/loading lifecycle in the existing composables.

Local entries:

- `/` and `/?idle=attention`: current user demo with coordinated attention.
- `/?idle=previous`: preserved phase-23 user-demo movement without gaze.
- Existing `?trial=motion` URLs retain their own movement contexts.

## Automatic source-only eye preparation

`app.preparation.gaze` uses the immutable portrait, cached refined MediaPipe landmarks and existing eye crop. It locates the two iris ellipses and lip-independent eye openings from fixed MediaPipe topology, refines visible side arcs against local image edges, and derives visibility masks automatically. There are no character coordinates, manual masks, provider calls or new inference.

The vacated iris region is filled by a sparse harmonic solve constrained exclusively by observed sclera inside the eye opening. Skin, eyelashes and tear-corner material outside that domain cannot become interpolation seeds. Known pixels remain untouched. Iris and pupil color come from rigidly translated original samples; opacity is geometric and does not depend on brightness.

The source does not show the upper iris hidden behind the lid. That material is **not** invented. At unsupported edge samples, the compositor preserves the original narrow lash-adjacent rim. Automatic bounds keep this protected, stationary sliver below 2% of visible iris material throughout the combined elliptical motion envelope. This trades a small stationary boundary for avoiding stretched pupils or fabricated texture. Vertical limits are smaller where the source is more occluded; wide gaze and eye rolling are outside this implementation.

The initial strictly unoccluded-mask experiment rejected Syntelle. Its failed input snapshot and response remain in `.tools/attention-motion-24/prepared/`; subsequent common algorithm revisions preserve those directories rather than overwriting them. Native contact sheets were inspected for Syntelle before processing the other two characters. The final pipeline uses one set of settings for all three.

Prepared assets are a sclera plate, opaque RGBA ownership map and versioned `gaze.json` with ellipses, source-supported bounds, reductions, asset hashes and validation. Source files, original portrait bytes, cached landmarks, settings, dependency versions, timing, input bindings, failures and inspection outputs are retained in PostgreSQL. Matching caches are hash-verified; a failed matching preparation stops explicitly, and no failed extraction is installed.

From `services/processor`, prepare and inspect Syntelle first:

```powershell
.venv/Scripts/python.exe -m app.workflows.motion.prepare_gaze --cases syntelle --install-preview
```

After inspection, apply the identical preparation to the other two:

```powershell
.venv/Scripts/python.exe -m app.workflows.motion.prepare_gaze --cases cafe woodland --install-preview
```

Defaults use `docs/characters/gaze-inputs.json` and `.tools/attention-motion-24/prepared/<id>/<binding>/`. New bindings retain earlier outputs; repeat runs verify all recorded hashes. Browser exports are in `apps/web/src/experiments/motion/gaze/`. Image, analysis and speech allowances are unchanged.

## Composition and bounded checks

Eyes and mouth are composed into the same source texture before the unchanged RGB-D deformation. The eye renderer retains its WebGL context and uses six texture units. It only uploads the eye patch when closure or the quantized gaze offset changes; fully closed eyes ignore gaze. Neutral eye frames and fully closed frames match the preceding compositor exactly, and pixels outside the measured opening remain unchanged. No pupil compression or opacity dissolve is used.

Five focused processor tests pass: masked sclera reconstruction, rigid pupil translation, protected outside pixels, source/package binding and the three installed preparations. The three browser scenarios pass in Chromium and WebKit desktop/mobile layouts: deterministic bounded trajectories, eye-shader invariants, and character loading/movement control/reduced-motion behavior. Over two simulated minutes, the three seeds produce 10, 10 and 11 brief glances; all include the initial diagonal adjustment and retain bounded speech gestures. These are functional checks, not owner visual approval.

One Chromium replay uses the actual archived 4.25-second Ava Dragon HD recording and its 52 native viseme events. The API route is intercepted locally before the service: zero Azure calls, no ledger writes, cache reuse and interruption on character change verified. Windows WebKit audio remains unsupported and was not represented as real audio validation.

The bounded serial Syntelle WebKit sample measures animation-frame cadence over four seconds after a half-second warmup: approximately 64.46 callbacks/s for the preceding profile and 64.99 for attention. This short local scheduling sample shows no material regression; it is **not** a sustained GPU benchmark or proof of the strict 60 fps rendering goal. The earlier approximately 56 fps Syntelle WebKit limitation and physical-iPhone verification remain open.

Scoped formatting/lint, frontend type checking and production build pass. Production still excludes development characters and trial assets. Native eye overlays, proportionally scaled GPU contact sheets, desktop/mobile screenshots and test reports are retained in `.tools/attention-motion-24/`. The first GPU contact-sheet layout incorrectly compressed the previews vertically; the corrected sheet preserves their proportions, and both records remain available. No exhaustive mouth-pair matrix was rerun.

Archive and restore the entire evidence tree separately using `app.audits.archive_preparation`; do not run a historical motion finalizer. Owner visual acceptance, held-out/public-upload preparation and physical-device approval remain pending.
