# Speech-context motion and blinking

The owner rejected the noticeable alternation of left/right head turns. `head=natural` now uses the optional `speech-context/1` extension in `.tools/contextual-motion-15/`. The preceding phase-14 head turns remain at `head=rotation-candidate`; phase-12 remains at `head=natural-previous`. The base motion trial without a head parameter is unchanged.

## Behavior and data flow

`features/character/animation/speechBehavior.ts` contains pure acoustic analysis, gesture planning, quiet posture and blink scheduling. The Vue speech composable decodes the already available PCM once and prepares an immutable plan. Rendering samples the plan on the audio output clock used for native Azure visemes; callback arrival times are not used. Existing mouth and eyelid layers remain independent, and both are composed before the unchanged shared character deformation.

- Idle posture changes are bounded to 0.22° yaw, 0.10° pitch and 0.08° roll. Seeded 12–27 second posture holds may keep the previous pose. There is no alternating yaw turn, forced return-to-center cycle or idle nod routine. Existing torso breathing remains independent of facial articulation.
- Audio RMS is sampled in 20 ms windows. Short gaps below 160 ms are bridged; pauses require at least 320 ms. The analysis proposes onset, longer pause, prominent energy peak and utterance-ending cues. Acoustic pauses are not claimed to identify semantic ideas, intentions or emotion.
- The planner favors pause/end cues and selects only some candidates. Nods last 1.2–1.5 seconds, reach at most 1.1°, and are spaced at least 2.4 seconds apart. Most have zero yaw; an occasional additional turn is limited to 0.22°. The downward motion and slower return are eased. Stop or a new speech session discards obsolete events and eases the current gesture contribution out over 600 ms inside the existing validated geometry envelope.
- Spontaneous blinks use an accumulated random hazard, rather than a recurring fixed timer. Onset, pause, ending and selected-nod cues can favor a blink. All automatic blinks spend the same replenishing allowance of 16/minute with limited accumulated credit and an 800 ms minimum gap; contextual blinks do not add an independent rate. This is an experimental tuning setting, not a physiological claim or guarantee of a particular count in a short sample.
- Initial cue probabilities are onset 20%, longer pause 45%, ending 55% and nod 25%, subject to the common allowance and minimum gap. Blink duration varies around the existing asymmetric 80/40/140 ms close/hold/open geometry, totaling approximately 229–291 ms. The original iris stays behind the eyelids.

The common settings are in `docs/characters/contextual-motion.json`; no character-specific masks, placements or behavior overrides are introduced. The same settings are applied to Syntelle, cafe and woodland. Listening/thinking states, gaze changes and emotional behavior are not inferred or added. The current product slice supplies idle and short text-to-speech playback, not a conversation-state engine or streaming audio.

## Preparation, playback and evidence

From `services/processor`, run `.venv/Scripts/python.exe -m app.prepare_contextual_motion`. It reads the three frozen phase-14 packages, adds the common configuration, binds the behavior source/configuration to new build hashes, archives preparation records in PostgreSQL, and writes the separate `contextual/` preview manifests. It never invokes a provider or changes request budgets. The existing geometry, MediaPipe coordinates, Depth Anything V2 Small depth, foreground, eyes and dentition are reused.

The preview is `http://127.0.0.1:5173/?trial=motion&character=syntelle&head=contextual-candidate`. After development inspection, its three manifests were copied to `natural/` and the local Docker web image was rebuilt. The owner's current `head=natural` URL therefore uses this behavior. The comparison link opens the preserved earlier head turns.

From `apps/web`:

```powershell
node scripts/export-motion-review.mjs syntelle --contextual-candidate
node scripts/export-motion-review.mjs cafe --contextual-candidate
node scripts/export-motion-review.mjs woodland --contextual-candidate
node scripts/review-contextual-speech.mjs
```

The first command group exports actual runtime compositions for all 81 mouth pairs at five blends and three eyelid positions, reported mouth-transition frames, neutral posture, conservative behavior bounds and a diagnostic nod/blink sequence. The diagnostic nod sequence is an explicit inspection fixture, not a promise to blink on every nod. The second script intercepts every speech route and replays the existing four archived Ava/Andrew English/Spanish responses across six character/language cases. It saves the actual UI frames, prepared plans and head-uniform trajectories; it cannot send a synthesis request to Azure.

The cached clips select one gesture per case, except cafe English with two. Development inspection covered all three characters against the supplied backgrounds and actual cached speech in both languages. No new facial-layer detachment, tooth fade or silhouette tear was found in these sampled frames. Existing phonetic/owner acceptance is not inferred from this review.

## Verification and persistence

New independent tests check long-pause versus consonant-gap detection, sparse and deterministic gesture selection, quiet idle without a nod routine, bounded nods, interrupted-gesture continuity, shared blink-rate limits, irregular intervals, stale-cue rejection and preserved comparison navigation. An initial velocity assertion failed; slowing the nod and its approach fixed the failure. The initial failed report/traces remain archived separately.

The processor suite passes 154 tests. Frontend formatting, lint, type checking and production build pass. The separate behavior suite passes all 16 Chromium/WebKit desktop/mobile-layout cases. All 84 combined installed browser cases pass, covering existing speech, blink, mouth, hidden-tab, reduced-motion and persistent-context regressions. Windows WebKit's lack of Web Audio remains explicit in its playback tests; actual cached-audio review uses Chromium.

All three runtime export reports cover 1,215 mouth/eye combinations with zero endpoint jumps, outside-eye pixel edits, exposed-iris changes and closed-eye source errors. The renderer and its partial texture-upload path are unchanged. Local performance measurements are retained separately; desktop viewport measurements do not validate real mobile hardware.

The 4.5-second mouth/blink/20 px blur stress samples measure approximately 60 fps in all Chromium cases. WebKit measures 56.1 fps for Syntelle desktop and 62–64 fps for the remaining desktop/mobile-viewport cases. The Syntelle desktop target remains unmet; the earlier phase-14 sample measured 58 fps, and these short local samples do not establish a statistically significant regression or improvement. Full measurements are preserved in `performance-*.json`.

`app.finalize_contextual_motion` freezes the source, settings, manifests, test reports, cached-speech dependency, images and sampled traces in PostgreSQL, then restores and verifies all archived file hashes. No paid image, analysis or synthesis request is made by this change; all existing allowances remain intact. Historical phase-12 and phase-14 finalizers target frozen experiments and must not be used to overwrite their evidence.

This remains an explicitly selected local experiment. Owner acceptance, strict Syntelle/WebKit 60 fps, real-iPhone validation and held-out/public-upload validation remain pending. No gaze model, intermediate eye generation, viseme regeneration, database migration, public endpoint or deployment is added.
