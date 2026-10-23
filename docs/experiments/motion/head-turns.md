# Brief head rotations

This phase-14 experiment is preserved for comparison. The owner subsequently rejected its noticeable turn routine; [Speech-context motion and blinking](contextual-motion.md) documents the current `head=natural` behavior. Use `head=rotation-candidate` to inspect the phase-14 turns. Do not rerun this historical finalizer against the updated runtime.

The owner clarified that lateral movement meant turning the head. The translation experiment in `.tools/head-motion-13/` is rejected and preserved. Its numeric geometry checks did not establish visual suitability. The current `head=natural` candidate uses `.tools/head-motion-14/`, `anatomical-head-rig/3` and `head-posture/3`. The original renderer is restored unchanged: it rotates the existing depth/anatomical mesh around the neck pivot, without an added lateral-position parameter.

The common director alternates brief seeded left/right yaw turns, with a smooth approach, a short hold and a return to neutral. Turns last 2.6 seconds; starts are separated by seeded 3–7 second intervals. Yaw targets use 65–100% of the idle bound. Pitch and roll remain small. Speech adds bounded gestures on the existing audio clock and preserves continuity when starting or stopping. Existing blink, reduced-motion, hidden-tab, texture-upload and resource-lifecycle behavior remains intact.

Prepared bounds are:

| Character | Idle yaw/pitch/roll | Speaking yaw/pitch/roll | Minimum sampled triangle area ratio |
| --- | --- | --- | --- |
| Syntelle | 6° / 0.9° / 0.6° | 8° / 2° / 1.2° | 0.3433 |
| cafe | 5.1° / 0.765° / 0.51° | 6.8° / 1.7° / 1.02° | 0.3261 |
| woodland | 6° / 0.9° / 0.6° | 8° / 2° / 1.2° | 0.2091 |

The automatic envelope check reduces cafe's amplitude to 85%; no per-character override is present. It checks 375 combined rotation/breathing configurations per character and rejects non-finite or insufficient positive triangle areas. These limits describe a restrained 2.5D approximation; hidden facial surfaces are not reconstructed.

## Reproduction and comparison

From `services/processor`, run `.venv/Scripts/python.exe -m app.prepare_head_motion` to reproduce version 3 from the cached foreground-11 resources. It writes `.tools/head-motion-14/` and preserves different-version output directories. Runtime manifests live in `apps/web/src/experiments/motion/natural/`; `natural-rotation/` retains the separately inspected candidate. Existing foreground, eye, depth and oral files are reused byte-for-byte.

- Current: `http://127.0.0.1:5173/?trial=motion&character=syntelle&head=natural`
- Preserved earlier rotation: `http://127.0.0.1:5173/?trial=motion&character=syntelle&head=natural-previous`
- Original motion trial remains available without a `head` parameter.

Use the same URLs with `character=cafe|woodland`. The visible comparison link preserves the selected character. The Docker development web image contains a source snapshot; rebuild it after source or manifest changes.

From `apps/web`, `node scripts/export-motion-review.mjs <character> --rotation-candidate` exports actual runtime compositions, neutral/open-eye frames, pure yaw bounds, combined bounds, timed turn frames and the reported mouth-transition frames. Head comparisons now use the same Rest mouth and open eyes. The script also checks all 81 mouth pairs at five blend positions and three eyelid positions (1,215 combinations per character).

## Verification and persistence

The processor suite passes 154 tests. Frontend lint, formatting, type checking and build pass. The initial browser run passes 68 Chromium/WebKit desktop/mobile-layout cases, including a new rotation-only trajectory regression for all three rigs. After installation, all 28 head/speech scenarios pass again against the active natural candidate. No synthesis or image-generation request is made by these tests.

Runtime exports have zero endpoint jumps, outside-eye changes, exposed-iris changes and closed-eye source errors. Development inspection compares left/neutral/right yaw and combined extrema against all three supplied backgrounds. No new face-layer separation or neck/silhouette tear was found in those inspected frames. The previous translation is not treated as an approved baseline.

With mouth playback, blinking and 20 px background blur active, the installed candidate measures approximately 60 fps in all six Chromium desktop/mobile-viewport cases. WebKit measures 58 fps for Syntelle desktop and 63–64 fps for the other cases, comparable to the earlier Syntelle shortfall. These 4.5-second samples run on local desktop hardware; mobile viewport emulation is not a mobile-device benchmark. Complete results remain in `.tools/head-motion-14/performance-*.json`.

`app.finalize_head_turns` archives the rejected experiment separately, the current source and manifests, review images, settings, observed budget usage and complete preparation outputs. It restores the new snapshot into a separate directory and verifies every file hash. It does not invoke providers or change request budgets. Do not rerun the historical `app.finalize_head_motion`, which targets the frozen phase-12 evidence.

This updates an explicitly selected experimental candidate; it does not promote it to the default motion trial or establish owner acceptance. Local frame measurements do not validate real mobile hardware. The earlier strict Syntelle/WebKit 60 fps shortfall, real-iPhone testing and held-out/public-upload validation remain pending. No character-specific mask repair, image regeneration, speech synthesis, schema migration or deployment is added.
