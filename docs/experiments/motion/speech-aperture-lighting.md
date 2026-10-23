# Speech aperture lighting

The owner rejected bright, thin enamel strips in partially closed speech and the
changed resting expression while audio was being prepared. Phase 81 addresses
both, preserving phase 80 for rollback. No provider requests are made.

## Prepared measurements

`app.preparation.aperture_lighting` adds optional `visible-aperture-lighting/1`.
Each articulation's mask B channel records its measured exposed column coverage
divided by the registered laughing reference's central opening. Mask R (edit
coverage), G (aperture signed distance), and alpha remain unchanged. Enamel,
surface, cavity, facial appearance and original manual-smile files are unchanged.

The reference dental preparation command includes this measurement before it
computes the new oral identity. The cache-only bundle command adds it to existing
characters and verifies portrait/package bindings, then rebinds dependent
resources. There are no character-specific coordinates or placement settings.
Missing or invalid references block that command. Older packages without the
extension retain their previous lighting.

CPU, CUDA and development WebGL use the actual aperture SDF for dental contact
shadow and the prepared local opening for ambient illumination. Narrow openings
receive less light; larger openings recover the reference illumination. This is
runtime RGB illumination only: canonical enamel RGB/alpha, crown geometry, lip
coverage and oral occlusion are preserved. Joint resting articulation uses its
analytic opening rather than the closed base-pose mask.

For packages with prepared smiling speech, waiting no longer selects the speech
mouth or halves the manual baseline before PCM playback starts. Historical
packages keep their archived waiting behavior. Active articulation, pause,
interruption and conversational gesture limits retain their existing clocks.

## Reproduction

From the repository root with `PYTHONPATH=services/processor`:

```powershell
services/processor/.venv/Scripts/python.exe -X utf8 -m app.workflows.motion.prepare_speech_lighting --bundle .tools/head-gestures-80/prepared/resource-bundle.json --output <fresh-prepared-directory>
services/processor/.venv/Scripts/python.exe -X utf8 -m app.workflows.rendering.export_native --bundle <fresh-prepared-directory>/resource-bundle.json --output <fresh-native-package>
```

Current evidence is retained in `.tools/speech-lighting-81/`, including the first
binding candidate and the corrected `prepared-v2/` / `package-v2/` identities.
The initial native build failure records historical fixture expectations; the
waiting correction now applies only to prepared smiling-speech packages.

Numerical invariants and internal visual inspection are separate from owner
visual acceptance. This delivery does not establish arbitrary-upload readiness.

## Verification and activation

- Twelve focused processor checks pass, alongside the native rendering,
  animation, adapter and media tests and the native Docker build.
- All 34 offline CPU/CUDA frames are byte-identical. Manual smile at 0/50/100%
  and laughter at 20/60/100% are also byte-identical to phase 80.
- Chromium and WebKit each render 24 speech samples plus seven reference/eye
  samples without WebGL errors. Internal inspection covers thin openings, FV
  contact, closure, wide articulation and smiling speech.
- The actual Vue player, isolated PostgreSQL fixture API and native server
  replay cached speech to completion with audible decoded PCM (peak 0.6513).
  The private fixture records one fixture model operation and one cached speech
  operation, zero recognition operations and zero paid provider calls. These
  counters are fixture invocations, not real model or speech requests.
- The selected local bundle is `prepared-v2/resource-bundle.json`; the selected
  native package is `package-v2/`. Its hash is
  `1c1d907472b689497321ddc503b21dd379aa6519ab00b4b3aa7d3411b5812771`.
  `activation.json` retains the preceding package/profile selections.
- The installed GPU worker also passes the public player's cached laughter
  playback and existing repeated-shake, tilt and nod previews (159 sampled
  gesture frames, no browser errors). The voice remains Ava. This correction
  does not alter gesture amplitudes, repetition or timing.

The first fixture HTTP command lacked its stream ticket and was rejected with
404. Its evidence is retained; the successful run uses the player's existing
authenticated WebSocket. No access policy was changed. The broader unchanged
reference-preparation suite was stopped during import to keep this correction's
verification bounded; focused processor checks pass separately.
