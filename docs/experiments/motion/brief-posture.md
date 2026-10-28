# Brief conversational posture adjustments

Date: 2026-10-27. Owner correction to coordinated performance (REQ-5): the head moved slowly from side to side and resembled an ongoing negation. This correction changes background posture, not semantic nod/shake speeds or mouth/audio synchronization.

`brief-posture/1` lives inside the existing attention director and uses its pause-aware animation clock. The optional conversational-motion companion selects it; packages without that companion retain their previous behavior. Phrase boundaries do not reset the posture path.

The previous background continuously interpolated random lateral targets for 1.8-3.6 seconds during speech. A second, slow head-follow movement accompanied spontaneous speaking glances. Together, those motions could resemble repeated disagreement even without an admitted shake cue.

The new common posture uses a bounded random walk with brief transitions followed by real holds. Speaking adjustments use 350-600 ms transitions and 1.9-3.3-second holds; idle uses 550-900 ms transitions and 2.6-4.6-second holds. Quiet near-identical targets extend the hold rather than forcing movement. These are candidate animation parameters, not physiological measurements. A wide change between opposite sides must first settle near center. Speaking glances retain their existing eye timing but no longer add a second automatic lateral head excursion.

Speaking background reserves at most 3.6 degrees yaw, 1.7 pitch and 0.45 roll, further limited by the character's qualified bounds. Idle is smaller. Semantic head cues retain their own trajectories, speeds, admission allowance and priority. Existing purpose/semantic suppression, final-head gaze, breathing, blinks, emotion policy, articulation, paired textures and phase-102 laughter recovery remain unchanged. No character-name condition, manual mask, image/analysis request, speech/model call, dependency or deployment change is involved.

## Focused verification

The same 30-second deterministic speaking trajectory moves in 672/871 sampled frame steps (77.2%) before correction and 139/871 (16.0%) afterward; the longest uninterrupted background adjustment is 0.533 seconds. This measures animation state, not decoded frame cadence or realism. The regression fails against the preserved attention source and passes afterward. Additional checks cover opposite-side targets, qualified bounds and continuity across short phrases. Native animation, adapter and media race suites pass. The first bounds assertion encountered floating-point roundoff (0.45000000000000007); its corrected numerical tolerance is 1e-9, without changing geometry gates.

Matched Chromium recordings use the same cached Serena text/PCM, effective facial directions, framing and resources. The first phrase uses plain Serena and the second its existing reassuring recording in both variants; no provider is called. Before/after videos and decision/audio evidence are in `.tools/brief-posture-105/`. Owner naturalness acceptance remains pending. No new performance benchmark, WebKit/device qualification or mouth synchronization fix is claimed.

## Runtime and evidence

The local review worker image is `syntelle-render-native:brief-posture-105`. The exact preceding worker is preserved as `syntelle-render-native:before-brief-posture-105`; API, web image, Serena capability/profile and original prepared package stay selected. `.tools/brief-posture-105/install.ps1` and `rollback.ps1` switch only the managed local worker while retaining the existing phase-103 API/player selection.

Run `app.audits.finalize_brief_posture` from `services/processor` to archive current/before source, binaries, tests and recordings in PostgreSQL and verify byte restoration. The receipt is `.tools/brief-posture-105-archive.json`, with immutable parent snapshot 744 supplying unchanged preparation/source closure. Mouth presentation diagnosis remains separate in snapshot 745 and is not resolved by this timing correction.
