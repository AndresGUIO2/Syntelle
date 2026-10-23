# Longer, visible idle posture

The owner found the default user demo's idle too brief and difficult to notice. Its phase-18 contextual director changed posture only every 12–27 seconds, moved for 4–6.5 seconds at the start, and had a 50% chance of repeating the same target. Repeated holds could leave an apparently frozen character.

The user demo now supplies the common optional `idle-posture/1` runtime profile through `MotionContext.idle`. It does not edit the frozen motion packages, their build IDs, geometry or image resources. Existing laboratory contexts omit this option and retain their preceding timing. The phase-22 UI source/evidence remains preserved.

## Shared settings

The canonical settings and sampler are in `apps/web/src/features/character/animation/idlePosture.ts`:

| Setting | Value |
| --- | --- |
| Eased posture movement | 4.8–6.8 seconds |
| Quiet hold after movement | 0.8–1.8 seconds |
| Maximum angle | Existing package's proposed idle bounds; yaw remains within ±3° |
| Minimum yaw target change | 65% of the yaw limit (1.95° for current packages) |
| Interpolation | Cubic easing with zero velocity at each end |
| Randomness | Existing character seed; deterministic, irregular targets and durations |

The sampler chooses a new yaw outside the previous posture's exclusion interval, without retries or a fixed left/right sequence. It starts moving immediately instead of sometimes choosing an unchanged neutral target. Pitch and roll retain their small existing limits. The slow posture continues under the existing speech gesture layer, so speech/idle transitions do not reset the head. Nod timing, gesture amplitudes, blinking and asymmetric breathing are unchanged.

All three characters use the same profile and algorithm. There are no manual character parameters, new geometry, image inference, paid calls, public endpoint changes or budget adjustments. Reduced motion, pause, hidden-tab suspension and the Natural movement switch keep their existing behavior.

## Bounded verification

Two Chromium desktop checks pass. The deterministic trajectory check samples the three existing seeds for two simulated minutes, without waiting in real time. At a yaw-speed threshold of 0.05°/s, movement occupies approximately 80–81% of the sampled time, versus 10–13% previously. Maximum observed near-still intervals are 1.85–2.05 seconds; maximum yaw remains below 3° and speed below 1.62°/s. This measures scheduling, not physiological realism or owner acceptance.

The live demo check observes facial pixels with blinking disabled over four seconds, then verifies that disabling natural movement freezes the character exactly. Its mean facial RGB difference is approximately 0.77/255, with zero browser errors and zero POST requests. A preliminary softer profile failed the initial three-second facial-change assertion (0.116/255); that evidence remains preserved. The current profile uses the already validated full turn envelope and a longer observation window instead of weakening the assertion.

Scoped formatting/lint, type checking and production build pass. No exhaustive mouth/browser matrix, new performance benchmark or real-device validation was performed. Live browser frames are retained for owner review; numerical movement alone does not establish visual acceptance.

Evidence, settings, source copies and the initial failed check are in `.tools/idle-motion-23/`. Archive and restore them separately with the generic PostgreSQL archive command; do not run a historical motion finalizer against current sources. This change does not promote the neck candidate or complete public-upload preparation.
