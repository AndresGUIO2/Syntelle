# Laughter lip-contact lighting

The owner observed an excessively white dental strip as cached laughter ends.
The independent reference compositor previously revealed the registered enamel
without the lip-contact shadow already used by speech. Its material bytes were
not whitening; it retained the open-reference lighting while the aperture closed.

The optional `reference-laughter/1` oral `contact_shadow` companion adds
`aperture-contact-shadow/1`. Preparation reuses the existing oral shadow strength
and derives its width from the character's measured reference aperture. At runtime
the shadow follows current lip distance and the fraction of aperture closure.
Only enamel RGB receives illumination, after immutable material sampling. Dental
opacity, coverage, placement, textures and all facial geometry are unchanged.
The reference peak receives no additional shadow, avoiding double shading of the
received image. Historical packages without this metadata retain their behavior.

`prepare_frames` includes the metadata for future reference preparation. Existing
cached bundles can receive it without rebuilding any image:

```powershell
# From services/processor; choose fresh output directories.
.venv/Scripts/python.exe -X utf8 -m app.workflows.motion.prepare_reference_contact_shadow `
  --bundle ../../.tools/repeated-nod-78/prepared-v2/resource-bundle.json `
  --output ../../.tools/laugh-release-79/replay-prepared

.venv/Scripts/python.exe -X utf8 -m app.workflows.rendering.export_native `
  --bundle ../../.tools/laugh-release-79/replay-prepared/resource-bundle.json `
  --output ../../.tools/laugh-release-79/replay-package
```

The active local package is `.tools/laugh-release-79/package/`, hash
`52a2e84a696bab3934f69a42ead562f56a0d0b07ede5f30cee1c68d902296252`,
with private `active-profile/profile.json` and native image
`syntelle-render-native:laugh-release-79`. The phase-78 nod range and repetitions
remain. Original manual smile resources are untouched; all 118 exported texture
hashes match the preceding package. Rollback settings are in `runtime-before.json`.

Twenty release states at manual smile 0/1 match CPU and CUDA byte for byte. Before
and after comparisons show only reduced RGB around covered enamel: no alpha or
geometry changes, no RGB increase, and exact full-laughter/manual resting frames.
Focused Go checks with the actual package and 11 processor tests pass. Frontend
type checking/scoped lint and isolated Chromium/WebKit shader checks pass. Actual
Chromium CUDA/NVENC playback completes two approved cached laugh replays;
its report remains in `live-review.json`. No paid request, voice change or new
reference is included. Visual acceptance remains with the owner.
