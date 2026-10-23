# Manual and speech-driven expressions

**Superseded smile:** the owner rejected this corner-only smile. Follow [Registered closed-mouth smile](registered-smile.md) for the current Syntelle reference and articulation limits. Café/Woodland retain brows without this rejected smile. The remaining text preserves the phase-25 experiment.

The local user demo now supports a soft smile and bilateral eyebrow raise for Syntelle, cafe and woodland. The owner selected both manual controls and automatic expressions during speech. This first version uses the existing portraits and cached preparation; it makes no provider request and adds no generated expression pose.

## Controls and behavior

Open **Customize → Expressions**. Smile and Raise eyebrows independently range from 0–100%. Reset expression returns both manual values to neutral. Automatic expressions during speech is enabled by default and can be disabled independently. Its preference is stored alongside the existing local appearance settings; manual expression amounts are not persisted.

Manual amounts ease over 240 ms. Speech expressions use the existing acoustic onset, pause, emphasis and ending cues on the audio playback clock: a restrained opening expression, occasional brow emphasis, and a soft ending smile. Optional emphasis/pause events use seeded selection with a 3.2-second gap. These are presentation cues, not inferred sentiment or understanding of the text. Automatic amounts stay below the manual maximum and combine continuously with the owner's manual baseline. Interruption releases the automatic contribution over 900 ms without clearing that baseline.

Reduced motion disables automatic expressions but permits immediate static manual expressions. Paused playback and hidden tabs keep the existing clock suspension. Natural movement still controls head/gaze separately; an owner can stop head movement and retain speech expressions. Mouth articulation, blink/double-blink scheduling, breathing and gaze planning are unchanged.

- `/` and `/?expressions=preview`: current development demo with expressions.
- `/?expressions=off`: same demo without the expression extension for comparison.
- Historical laboratory contexts omit the extension and keep their previous behavior.

## Shared automatic preparation

`app.workflows.motion.prepare_expressions` accepts the explicit `docs/characters/expression-inputs.json` manifest: original portrait, frozen motion package, cached refined MediaPipe landmarks and existing oral package. Dental assets resolve relative to the supplied oral package. No character-specific coordinates, manual mask editing, new model inference or image generation is used.

`local-expressions/1` derives compact smooth displacement fields from mouth corners, cheek support and both eyebrow contours. The upper/lower dental alpha materials and every recorded rigid row translation form a protected dental region. Eye openings/lid margins and the central mouth cavity/contact seam are also protected. Sampling-footprint padding prevents lower-resolution field interpolation from leaking deformation into these regions. This keeps original pupil/iris material, shared teeth, tongue and central FV/MBP articulation stable while the corners and nearby cheeks move.

The field stores smile horizontal/vertical displacement and brow vertical displacement in a source-bound PNG, with an explicit byte-to-source-pixel scale. Its neutral value is exact. A five-step inverse field lookup samples the already composed mouth/eye texture before the existing RGB-D projection is displayed. Head geometry, source images and frozen build IDs are not rewritten. The current implementation is a small image deformation, not a facial-muscle simulation or reconstruction of new wrinkles/hidden surfaces.

Preparation validates the actual quantized field at bilinear cell corners across 25 expression combinations. It rejects inverted local mappings and excessive inverse residual, and reduces the common requested amplitudes automatically. Independent smile/brow reductions avoid making one action unnecessarily weak because of the other's constraints. Final current reductions are 0.7225 for Syntelle and 0.85 for cafe/woodland; minimum Jacobians are approximately 0.70–0.74 and maximum inverse residuals 0.09–0.18 source pixels. These are numerical safety measures, not visual approval.

From `services/processor`:

```powershell
.venv/Scripts/python.exe -m app.workflows.motion.prepare_expressions --cases syntelle --install-preview
```

Inspect its native comparison before preparing the other characters:

```powershell
.venv/Scripts/python.exe -m app.workflows.motion.prepare_expressions --cases cafe woodland --install-preview
```

The command saves immutable originals, landmarks, motion/oral inputs, dental materials, source copies, dependency versions, settings, hashes, fields, protection masks, native previews, reductions, timings and failures under `.tools/expressions-25/prepared/<id>/<binding>/` and in PostgreSQL. Matching caches are hash-verified; failed matching preparations stop explicitly. Browser exports live in `experiments/motion/expressions/<id>/`. Earlier experimental bindings remain preserved, including the overly restricted first draft and the field-interpolation boundary correction.

## Vue and rendering boundaries

`MotionContext.expressions` is an optional source/package-bound companion. Asset loading validates its motion/oral identities and field checksum. `animation/expressions.ts` owns the deterministic cue planner/director; `useExpressions` owns manual/automatic controls. `ExpressionControls.vue` presents the existing drawer section with typed models, and the workspace/stage pass values explicitly. The existing perspective renderer samples the field; zero intensity bypasses the expression lookup.

No WebGL context is added. The field is uploaded once. Changing intensity updates uniforms; mouth and eye patches keep their partial texture uploads and original resource cleanup. Production builds continue to exclude the development demo and prepared resources.

## Bounded verification and limits

Four processor cases pass: the three real preparations plus rejection of an intentionally inverted synthetic field. They verify exact neutral pixels, unchanged protected pixels and pixels outside the field, field hashes, Jacobians/inverse residuals, and protected material in representative A, FV and MBP endpoints. Cache replay reuses the same immutable PostgreSQL snapshots.

Focused Chromium desktop/mobile and WebKit desktop checks verify all three characters' manual actions, exact reset to neutral, deterministic speech cue/release behavior, mobile layout and reduced-motion controls. One actual cached Ava Dragon recording confirms nonzero automatic expression uniforms during playback and simultaneous access to manual controls, followed by interruption. The recording is intercepted locally before the API; zero Azure calls and no speech-ledger fixtures are involved. Two initial failures were test-harness issues (a missing Vue-internal test lookup and a malformed range value); their evidence is retained and the test now reads the actual shader uniforms.

Scoped frontend formatting/lint, type checking and production build pass. No exhaustive pose-pair matrix or new sustained performance benchmark was run, as requested by the owner. Existing strict WebKit performance, physical-device and public-upload validation remain pending. Internal native/browser inspection covers the soft smile and brow raise; owner visual acceptance remains pending. Large grins, new exposed dental coverage and detailed expression wrinkles are outside this source-only version.

Use the generic `app.audits.archive_preparation` command to archive/restore this evidence separately. Historical motion finalizers must not overwrite current or preceding snapshots. Provider budgets, credentials, public APIs, database schema and deployment are unchanged.
