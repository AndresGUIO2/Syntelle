# Asymmetric breathing and occasional double blinks

The optional `?trial=motion&character=syntelle|cafe|woodland&head=natural` candidate now uses `speech-context/2` and `asymmetric-breath/1`. The same configuration applies to all three characters. The preserved phase-15 behavior remains at `head=contextual-candidate`; the new preparation is also directly selectable at `head=vitality-candidate`. The motion trial without a head flag remains unchanged.

## Breathing

`features/character/animation/breathing.ts` replaces the candidate's sine with a deterministic sequence of asymmetric cycles. Each cycle lasts 4,600–6,000 ms; inhalation occupies 32–38%, followed by the slower exhalation. Amplitude varies from 82–100% of the existing 0.006-face-height displacement bound. Quintic easing joins cycles at neutral with zero velocity and acceleration. Sampling an earlier time reproduces the same cycle; no per-frame random noise is added.

The existing anatomical torso weights and shader are retained. Inhalation raises the torso slightly; exhalation returns it to neutral. Neither the face nor the background breathes. Mouth and eyelids are still composed before deformation. Movement off restores exact neutral rendering; the original head-animation seed is recorded explicitly, preserving the prior head-posture trajectory. Historical packages without the new optional configuration retain their sine behavior.

## Double blinks

The existing shared spontaneous/speech blink planner now optionally reserves a second closure. After the first complete blink, eyes remain fully open for a seeded 90–160 ms before the second complete blink. Both closures use the existing geometric eyelid renderer, with small duration variation and no generated intermediate eye pose.

Automatic doubles have a 20% chance **when eligible**, require two available credits, and are separated by at least 18 seconds. This is not a 20% chance on every rendered frame or every speech cue. All spontaneous, speech-triggered and double closures consume the same 16-closures/minute token allowance. The bucket holds at most two credits, initially full; two closures are debited in advance, rather than charging one for the pair. An unshown second closure is refunded on cancellation. Actual second starts reset the ordinary 800 ms spacing rule; the brief within-pair interval is its explicit exception.

`Blink twice` is a manual preview, enabled only when two credits are available. It ignores the automatic probability/cooldown while spending both credits. The existing unrestricted `Blink once` inspection control remains available and spends its normal credit. Pausing or hiding the tab freezes the same animation clock, including the brief open interval. Manual eyelid inspection, reduced motion and disposal cancel pending second closures. Disabling automatic blinking cancels an automatic second closure; an explicitly requested manual pair can finish while automatic blinking is off.

The deterministic one-hour idle simulation produces 772 total closures, including 29 double groups. Both closures count in that total. It stays below the shared refill allowance at every simulated step and never produces a triple. Frequency is not reset by toggling automatic blinking.

## Preparation and verification

Common settings live in `docs/characters/vitality-motion.json`. From `services/processor`:

```powershell
.venv/Scripts/python.exe -m app.workflows.motion.prepare_vitality_motion
```

The command reads the frozen phase-15 packages, preserves all existing geometry and image references, records source/configuration hashes and the prior animation seed, archives preparation manifests in PostgreSQL, and writes separate `vitality/` preview manifests. It performs no provider request and does not install into `natural/`. After review, copy only the three approved manifests to `natural/`; retain the `contextual/` manifests. The Docker web image must be rebuilt to serve source changes.

New browser tests independently check cycle asymmetry/variation, deterministic seeking, displacement bounds, exact face pixels, neutral restoration, one full texture upload, shared frequency accounting, cancellation/refunds, no triples, a fully open interval, real second closures, pause/resume, hidden tabs and reduced motion. All 32 new Chromium/WebKit desktop/mobile-layout cases pass. The processor suite passes 160 tests. Frontend formatting, lint, type checking and production build pass.

All 136 installed browser/API regression cases pass, including the existing speech, resource-lifecycle and historical-module checks. Initial development failures remain preserved: the first assertions distinguished negative zero from zero and sampled too narrow a second-blink interval while the test clock still advanced during screenshots. Neutral output is now explicitly zero, and screenshot tests freeze browser time between controlled steps and capture the actual fully closed second frame. No assertion was relaxed to accept an incomplete closure.

All three runtime export reports cover 1,215 mouth/eye combinations each, including all nine mouth endpoints and the reported intermediate mouth frames. They report zero endpoint jumps, edits outside eye masks, exposed-iris changes and closed-source differences. Development inspection covers open-between-closures and both fully closed states on all three characters. Six archived English/Spanish Dragon HD recordings play through the real candidate UI with every speech route intercepted: zero new synthesis requests. Existing foreground, depth, eye and oral resources are reused unchanged.

Complete evidence is kept in `.tools/vitality-motion-17/`. After verification, freeze, archive and hash-verify restoration with:

```powershell
.venv/Scripts/python.exe -m app.audits.finalize_vitality_motion
```

Use this finalizer for this experiment; historical finalizers target frozen sources. Credentials, provider allowances and existing request ledgers are untouched. This updates an explicitly selected local candidate, not public-upload preparation or real-device approval. Owner acceptance, real-iPhone validation and the outstanding Syntelle desktop WebKit performance target remain separate.

The serial 4.5-second mouth/blink/20 px blur stress samples measure approximately 60 fps in all Chromium cases. WebKit measures 55.3 fps for Syntelle desktop and 61.8–63.6 fps for the remaining desktop/mobile-viewport cases. The preceding phase-15 Syntelle sample was 56.1 fps: these short samples do not establish a statistically significant change, and the strict Syntelle desktop target remains unmet. Full measurements are retained in `performance-*.json`; no physical mobile performance claim is made.
