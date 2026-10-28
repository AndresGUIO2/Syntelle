# Laughter mouth and mandibular regression correction

**Owner visually rejected phase 110 on 2026-10-28.** Its technical checks did not establish acceptable laughter or dental appearance. The active local runtime has been restored to exact phase 107, before the mandibular/within-viseme and laughter-composition experiments. Use `.tools/laughter-restore-111/restore.ps1`; preserve phase110/snapshot770 as rejected evidence. The new jaw is temporarily withdrawn with that rollback; its implementation is not visually accepted.

Phase 110 repairs the owner's rejected phase 109 result. The screenshot is preserved in `.tools/laughter-correction-110/owner-rejection.png`; phase 109 and PostgreSQL snapshot 769 remain unchanged.

## Reproduced failures

The thin stretched grin is reproduced with full laughter expression and zero acoustic jaw opening. It also appears in the previous phase's own review frames. The reference oral path compressed open laughing lips into the neutral resting aperture while retaining the full open-reference smile deformation. Tests for endpoint continuity and CPU/CUDA agreement passed because both implementations shared the mistake. They did not establish a coherent closed-jaw smile.

The mandibular weights extended to chin plus 65 percent of face height. A jaw-only test measured 2.329 px of lower-neck movement despite fixed head and breathing. The cached 478-point MediaPipe record was used, but the preparation algorithm assigned this overly broad support; MediaPipe does not supply a calibrated jaw rig or validate these weights.

## Correction

The closed-jaw oral expression reuses existing calibrated joint-smile geometry and skin. Acoustic opening transitions once to the complete reference laughing mouth, with canonical opaque enamel drawn once. Both lip materials are transported into the same interpolated contour before sampling; a rejected intermediate RGB blend produced a double lip edge. Full amused cheeks/lids keep their independent lifecycle. The oral opening is monotonic, preserves its source-calibrated peak, and recovers to the live selected baseline. No smile amplitude reduction or new image conceals the failure.

The existing speech topology needs a deterministic shared internal-diagonal correction for the wider laughter lip band. Source coordinates and the fixed boundary do not move. Both endpoint orientations, opposite-side diagonal membership and the exact quadratic signed-area minimum across the full interpolation must pass; no valid shared topology means rejection rather than a flipped/hidden triangle. The chosen topology is independent of instantaneous jaw opening. CPU and CUDA sample immutable mouth snapshots through the same maps, preventing frame or parallel-pixel feedback. New private CUDA allocations are included in resource accounting.

A focused mouth-only comparison exposed an additional CUDA error hidden by whole-frame metrics: the default-stream snapshot could run before oral preparation on the nonblocking renderer stream. The copy now uses that same renderer stream. Mouth RGB agreement increased from 25.53 dB to 84.96 dB in the reported quiet-gap state, with 76.79 dB minimum across eight isolated mouth states. Whole-frame/alpha agreement is retained separately. Legacy lip-overlay parameters now also use the same suppression policy as CPU.

Optional `mandibular-rig/2` bounds influence using the cached lower facial oval and a short adjoining under-chin taper. The lower neck and clothing receive zero jaw influence. Original projected geometry, articulation timings, oral materials, accepted head bounds and audio remain unchanged; version 1 packages remain loadable. This is an estimated source-bound rig, not anatomical reconstruction.

## Verification and use

Evidence lives in `.tools/laughter-correction-110/`: failing/passing regressions, rejected intermediate opening trajectories, fixed-head comparisons, CPU/CUDA frames and actual decoded-video captures of cached speech and laughter. Closed-jaw gaps, intermediate openings, peaks, settled manual/Playful combinations and direct recovery are inspected independently. Original recordings and listening approvals are retained without renewed synthesis. The review is not a new sustained-performance or future-character qualification.

Reproduce with the existing preparation environment:

```powershell
$env:PYTHONPATH = 'services/processor'
services/processor/.venv/Scripts/python.exe -m app.workflows.motion.prepare_mandible --source .tools/jaw-articulation-108/prepared-final/package --landmarks .tools/foreground-11/prepared/syntelle/base-landmarks.json --output .tools/laughter-correction-110/reproduction-mandible
services/processor/.venv/Scripts/python.exe -m app.workflows.motion.prepare_laughter_presentation --source .tools/laughter-correction-110/reproduction-mandible/package --joint-field .tools/emotion-skin-84/prepared-v2/reconstructed-field/field.npy --output .tools/laughter-correction-110/reproduction
```

Preparation requires new output directories and makes zero image, analysis, synthesis or model calls. It uses no manual character masks or placement overrides.

Local activation uses `.tools/laughter-correction-110/install.ps1`; rollback uses its `rollback.ps1`. Activation preserves stable `syntelle-render-native:local` / `syntelle-render-native-local` naming and updates the same resource bindings used by `dev start`. API/player code remains unchanged. `app.audits.finalize_laughter_correction` archives complete sources, resources, approved audio, failed results and comparisons to PostgreSQL and verifies byte-exact restoration. Final owner naturalness acceptance remains open.
