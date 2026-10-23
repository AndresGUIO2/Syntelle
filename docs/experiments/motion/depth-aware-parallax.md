# Depth-aware head parallax

The optional `head=natural` motion trial now uses `rgbd-perspective/1` for Syntelle, cafe and woodland. The previous phase-17 renderer and movement remain available at `head=vitality-candidate`. The ordinary motion trial without a head flag is unchanged. This is a small-turn local candidate; owner acceptance and physical-device validation remain pending.

## Comparison

Open `http://127.0.0.1:5173/?trial=motion&character=syntelle&head=rgbd-compare` and select **Replay comparison**. The three views share one clock, expression textures, blink state, breathing track and background:

1. Previous renderer with the previous movement amplitude.
2. Previous renderer with the proposed turns.
3. Perspective RGB-D with exactly those proposed turns.

Manual yaw presets cover 0°, ±1°, ±2° and ±3°; pitch inspection covers ±1°. A deterministic 16-second replay visits both directions and returns to neutral once. **Automatic head motion** restores the existing seeded behavior. Autonomous yaw has a larger envelope, while nod timing, nod amplitude, blink planning and asymmetric breathing retain their existing configuration. Movement-off and reduced motion retain essential speech articulation. Hidden tabs and pause freeze the shared clock.

Use `character=cafe|woodland` for the other inputs. `head=rgbd-candidate` selects the same renderer without the three-view layout. The development-only `render=legacy` measurement switch uses identical proposed motion in the preserved renderer; it is not a product or public API setting.

## Preparation and geometry

Run from `services/processor` with the local PostgreSQL preparation profile available:

```powershell
.venv/Scripts/python.exe -m app.workflows.motion.prepare_projection_motion --manifest ../../docs/characters/projection-inputs.json --install-preview
```

The manifest supplies immutable portrait bytes, an existing motion package, cached full-precision depth, MediaPipe XYZ landmarks, semantic confidence maps and foreground. The implementation has no character names, manual masks or placement overrides. Common settings are versioned in `app/preparation/projection_rig.py` and copied into each preparation record. Results live in `.tools/rgbd-motion-18/prepared-current/`. Matching inputs, algorithm, workflow and settings reuse that cache; changed bindings, corrupt outputs and previously failed preparations are rejected instead of overwritten.

Stable skin landmarks exclude eye and mouth regions. Huber scale/offset fitting calibrates relative depth to MediaPipe facial depth. The observed robust facial range bounds extrapolation. An edge-aware low-frequency field retains a bounded dense residual inside the stable facial hull; the face is not replaced entirely by interpolated landmarks. Reliable foreground depth continues through the uncertain alpha boundary, preventing removed-background depth cliffs. A common slope constraint conditions remaining discontinuities.

The 128×128 indexed grid contains 16,641 vertices. Fully transparent cells contribute no triangles; alpha still defines visibility inside boundary cells. Automatically derived anatomy supplies head/neck and torso weights. A smooth chin-relative taper prevents shoulder rotation. No disconnected opaque geometry is created across background. The reference virtual camera uses image center and a focal length of 1.5 times the longer image dimension. Source pixels are unprojected at their calibrated depth; rotation is around the existing automatically derived neck pivot, followed by perspective projection.

Each package adds an optional `projection` extension with camera, geometry hashes, calibration, weights, parent/source hashes, settings binding and validation attempts. Binary geometry contains float32 vertices and uint16 indices. All three final packages pass 81 combined yaw/pitch/roll/breath envelope configurations at ±3° yaw, ±1.2° pitch and ±0.3° roll, with minimum projected area ratios 0.573, 0.592 and 0.604 respectively. No final amplitude reduction is necessary. Automatic reduction remains available; unsafe or imperceptible envelopes fail explicitly.

## Rendering

The new renderer preserves transformed depth and enables depth testing. Opaque fragments write depth first. The uncertain hair boundary uses premultiplied color, a separately sorted index subset and depth testing without depth writes. Geometry is uploaded once; only the small boundary ordering changes with pose. Existing eye and mouth compositors update expression rectangles in the persistent subject texture before the shared deformation. Solo playback keeps three WebGL contexts; comparison adds two persistent body contexts. All resources are disposed with their owning scope.

Homogeneous projective source coordinates preserve texture placement through the perspective transform. Neutral head rendering uses reference-image sampling to avoid grid-dependent raster interpolation drift; the independent flat-source-quad regression permits at most one RGB/alpha quantization step. Neutral facial pixels are exactly unchanged during torso breathing. Backgrounds remain separate and stationary. There is no camera orbit, dolly or lateral character translation.

## Verification and evidence

Processor regressions cover robust calibration with outliers, depth direction, independent projection/texture math, local relative-depth motion, stationary torso, slope convergence, inverted/clipped geometry, cache reuse and corrupt-cache rejection. The processor suite passes 167 tests. Frontend formatting, lint, type checking and production build pass. The installed Chromium/WebKit desktop/mobile-layout suite passes 176 cases, including previous speech, blink, interruption, reduced-motion, hidden-tab and resource-lifecycle regressions.

Independent GPU tests select the nearer of two deliberately overlapping colored surfaces regardless of draw order, and verify transparent-over-opaque blending. Browser tests verify source registration, geometry checksums, background independence and synchronized comparison controls. These checks do not substitute for visual inspection.

Runtime exports retain all 1,215 mouth/eye combinations per character (81 pairs × five interpolation positions × three eye closures). Eye-mask edits, exposed-iris changes, closed-source differences and endpoint jumps remain zero. A further 99 combined head/expression frames per character cover all nine endpoints and EI→FV at 35% / FV→A at 41%, three eye closures and yaw −3°/0°/+3°. The native envelope exports cover seven yaw positions at three pitches. Six existing English/Spanish Dragon HD recordings replay through the real UI with synthesis routes intercepted; no provider request is made.

Development inspection includes three-view comparisons, head/neck crops at envelope limits, all nine mouth endpoints and the reported transitions, eyes, hair/ears/accessories, all supplied backgrounds, and blur off/on. No new holes, doubled contours or detached expression layers were observed in the accepted small-turn envelope. It remains a single visible surface, with inherited foreground/oral quality limits. It does not reconstruct hidden cheeks or ears, prove wide-angle quality, or complete public-upload validation.

Performance uses serial, amplitude-matched 4.5-second mouth/blink/20 px blur workloads during deterministic turning. Chromium measures approximately 60 fps for all three characters and both viewport sizes. Syntelle desktop WebKit measures 55.8 fps versus 56.4 fps for the matched legacy renderer (about 1.2% lower; both have 30 ms p95). The other WebKit cases measure approximately 62.6–64.9 fps. There is no material regression in these short local samples, but the strict Syntelle/WebKit 60 fps target remains unmet. Mobile viewports run on desktop hardware and establish no real-phone performance claim. Three-view comparison is intentionally heavier and is not the solo playback target.

Earlier preparations and failed browser assertions remain in `.tools/rgbd-motion-18/`. Early depth extrapolation produced silhouette/neck inversions; the common foreground-depth continuation and slope constraint corrected the algorithm for all three characters. Source registration is checked against an independent flat image quad rather than accepting the old grid's sampling error. No manual character repair, regenerated asset or provider call was used. The first installed browser run passed 175/176 cases; an existing WebKit mobile checkbox action timed out waiting for element stability. Its isolated rerun passed without code or assertion changes. The complete acceptance run executes serially to avoid browser-resource contention; the failed trace is preserved.

## Archive and restoration

```powershell
.venv/Scripts/python.exe -m app.audits.finalize_projection_motion
```

The finalizer verifies installed package identity, source/geometry bindings, unchanged eye/depth/foreground/oral inputs, test reports, cached speech and visual review records. It archives complete inputs, outputs, rejected attempts, source snapshots, comparison evidence and performance results in the existing PostgreSQL archive, then restores and hashes every file. It does not rerun historical finalizers or reset any provider allowance. The optional candidate is updated only after internal review; the previous renderer/package remains selectable. No database schema, public endpoint, model, paid call or deployment is added.
