# Disgust readability and lip correction

The owner reported that disgust looked neutral in Syntelle's face, then rejected an odd lip appearance in the stronger candidate. Phase 93's final local review candidate is `.tools/disgust-readability-93/prepared-v6/` and `package-v6/`. Previous variants remain preserved and explicitly rejected or unqualified. This changes only the disgust calibration from phase 87. Distinct emotional interpretation and owner acceptance remain unqualified.

The preceding lip action was reduced to about 1.16 source pixels by a thin corner triangle. Increasing an isolated outer-lip curl made it stronger but stretched vermilion against stationary skin. An intermediate candidate also failed an explicit corner/contact check and was rejected. Geometry checks alone did not resolve the owner's visual objection.

The final preparation removes independent disgust lip displacement. It also omits the all-zero oral state entry: a present zero entry still enabled joint UV resampling. Nasal displacement now has a smooth anatomical falloff above the highest registered lip contour across all cached poses, reserving the lip surface from that second warp. This changes displacement support, without adding or blending compositing masks. Disgust uses stronger nasal/lid cues and a small existing Rest/A articulation at rest. Spoken visemes retain closure/contact ownership. Canonical teeth, source skin, masks, mouth textures and prior coverage corrections remain unchanged.

The optional preparation flag `--refine-state disgusted` freezes all nine other field pages and oral actions. It requires explicit source-bound calibration metadata and fails on implicit legacy calibration instead of changing its defaults. Geometry gates may reduce only the requested state. There are no new model requests, expression images, emotional viseme sets or manual character masks.

## Reproduction

From `services/processor`, choose fresh output folders:

```powershell
.venv/Scripts/python.exe -m app.workflows.motion.prepare_emotions --bundle ../../.tools/emotion-readability-87/prepared-v3/resource-bundle.json --landmarks ../../.tools/emotion-readability-87/prepared-v3/landmarks.json --refine-state disgusted --output ../../.tools/disgust-readability-93/reproduction
.venv/Scripts/python.exe -m app.workflows.rendering.export_native --bundle ../../.tools/disgust-readability-93/reproduction/resource-bundle.json --output ../../.tools/disgust-readability-93/reproduction-package
```

The recorded final incremental command, `prepare-canonical-mouth.py`, reuses the byte-identical, fully validated v5 field and protection images, then recomputes the generic oral companion and exports a fresh v6 package. Source snapshots, hashes and the explicit reuse record are preserved; no frozen output is overwritten.

## Verification and limits

Fifteen focused processor checks pass. All ten quantized fields, 45 transition midpoints and existing articulation gates remain valid. The nine other field pages, their eyelid/rest settings, other oral actions and all 118 non-field textures are unchanged. Eight neutral/anger frames remain byte-identical to phase 87. At equal MBP, FV and open-A articulation, the inspected mouth region contains zero changed RGB bytes relative to neutral. Thirty-eight CPU/CUDA frame pairs match exactly, including same-frame fallback. Existing focused native tests pass. The renderer binary/image is unchanged.

An isolated PostgreSQL CUDA/NVENC player fixture completes five cached replies for neutral, anger and disgust at 35/50/100 percent. Final counts are zero model calls, zero recognition calls and five cached speech preparations. The earlier v3/v5 runs are separate evidence, with their own five deliveries each; they are not real provider synthesis. A fixture-startup failure is preserved too. Samples record playback status and include some completion frames. This is a bounded visual/articulation check, without new cadence or device qualification.

The final comparisons remove the localized lip stretch in internal inspection. Disgust's semantic readability, naturalness across the full intensity range and owner acceptance remain pending. Prepared resources and screenshots are not a claim that these requirements are complete.

## Activation and rollback

Only private package/profile selection changes. The UI, voice synthesis, gesture code, controller definitions and saved preferences are unchanged. The profile copies existing authenticated capabilities and changes only package binding. Stable image/container names and `.\dev start` remain in use. `activation.json` records three nonsecret settings for rollback to phase 87; no binary rollback is needed.

Complete sources, resources, rejected variants, owner feedback and cached evidence use the generic PostgreSQL archive with byte-verified restoration in `.tools/disgust-readability-93-archive.json`, parent snapshot 645. Do not run historical finalizers. There are no paid requests, image generations, new models/dependencies, schema migrations or cloud deployments.
