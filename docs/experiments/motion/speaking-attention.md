# Conversational attention during speech

The owner found the speaking character unsettling, particularly its fixed stare. Phase 34 deliberately faded both idle posture and glances to zero during directed speech. Neutral replies therefore held a centered head and gaze until speech ended. This correction replaces that behavior in the local demo with optional `conversational-attention/1`.

## Behavior and scope

`features/character/animation/speakingAttention.ts` plans brief sideways or downward looks, returns to the viewer and a slower partial head follow. Small posture changes continue at 30% of the existing idle amplitude. Existing semantic gestures still reserve headroom inside the same projection bounds; no regular left/right sweep is added. The common settings apply to Syntelle, cafe and woodland.

| Setting | Common value |
| --- | --- |
| First viewer-contact period | 850–1,500 ms after speech begins |
| Later contact periods | 2,200–4,300 ms after the preceding head follow settles |
| Eye move / look away / return | 110–170 / 900–1,700 / 200–320 ms |
| Partial head follow | 180 ms delay, 700 ms adjustment, 850 ms return |
| Speech entry / return to idle | 700 / 1,200 ms |
| Short phrase-gap continuity | Preserve the attention episode across gaps up to 900 ms |

These are animation choices, not measurements of human conversational behavior. Both eyes retain one shared direction limited by the existing observed-iris support. Counterrotation, eye materials, eyelid coverage and renderer precision are unchanged. The source's narrow supported gaze envelope still limits how far a character can visibly look away.

Attention uses the existing paused/visibility-aware animation clock. Audio-clock nod/shake timing remains separate. New progressive clip sessions do not restart attention; short inter-clip gaps blend without restarting the same opening glance. A stopped reply settles back to the ongoing idle posture. Existing reduced-motion and movement controls suppress the extension through the renderer's existing lifecycle.

Component boundaries remain unchanged: `DemoCharacterStage` connects controls to `useCharacterMotion`; `useMotionRenderer` samples animation and renders existing resources; the planner contains timing only. `MotionContext.conversationalAttention` opts in. No component or reusable animation module reads URL parameters. The development and generated build catalogs supply that explicit setting.

- `/`: current conversational attention.
- `/?attention=previous`: preceding centered directed-speech behavior, for comparison.
- `/?performance=previous`: preserved pre-direction behavior.
- Existing laboratory contexts without the option retain their behavior.

## Verification and evidence

Bounded checks cover the three source-supported head/eye envelopes, repeated speech-clip session changes, a short phrase gap, paused sampling, stop settling and actual cached progressive playback. The existing preceding-behavior test now selects its comparison explicitly. Browser API and WebSocket routes are intercepted locally; verification makes no provider requests or ledger writes.

Before/final sources, settings, trajectory measurements, screenshots, failed test-fixture setup and corrected results live in `.tools/speaking-attention-39/`. Archive and restore this tree with the generic `app.audits.archive_preparation` command. Do not run a historical motion or performance finalizer against current sources.

The three deterministic 60-second speaking traces each contain 11 gaze breaks, spend approximately 25–28% of time looking away and keep the longest uninterrupted contact at 4.82–5.00 seconds. Head speed stays below 2.24 degrees/second without semantic gestures; existing eye support bounds and binocular direction checks pass. These are functional measurements, not perceptual acceptance thresholds.

Four focused Chromium desktop cases pass against the rebuilt loopback demo on port 5173: speaking trajectories, actual cached progressive playback with eye/head shader updates, the previous centered comparison and existing semantic-direction limits. Two Chromium mobile cases and the WebKit trajectory case pass; Windows WebKit audio is explicitly skipped because Web Audio is unavailable there. Six runtime-manifest/build checks, scoped lint/formatting, frontend type checking and the validation-only production build pass. No new sustained performance benchmark or full mouth/browser matrix was run. The initial playback fixture omitted the required recognized-turn event and failed before speaking; that failed record and the corrected fixture are both retained.

No portraits, prepared masks, oral/eye/foreground/depth resources, source prompts, budgets, model/provider configuration or animation geometry are changed. Facial laughter remains disabled. Passing functional checks does not establish visual naturalness, owner acceptance, physical-device quality, sustained renderer performance or public-upload readiness.
