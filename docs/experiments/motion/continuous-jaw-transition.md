# Continuous lower-face jaw transition

Phase 113 corrects the owner's phase 112 observation: chin and lateral jaw regions move while an intervening skin band stays fixed. Preserve phase 112 head/idle improvements, the restored phase 107 laughter, Emma recordings and existing A/V timing.

## Cause and revised protection

The original body-only jaw protected a convenience oral edit rectangle, then every intersecting source triangle. That rectangle includes approximately 49 source pixels / 18 framed 720p pixels of skin below the actual all-pose lip envelope. Together with its initial rectangular distance clamp, it created three disconnected moving regions and a zero-to-full displacement step at the chin.

Optional `speech-jaw/2` protects the actual calibrated outer-lip support instead. A convex envelope includes native-precision base pose endpoints, joint-smile and normalized-emotion extrema, including smile/emotion combinations. A measured sampling guard and intersecting-triangle protection retain actual lip/enamel pixels. The loader re-derives this source-bound envelope; invalid bindings reject preparation/loading. Version 1 retains its old strict rectangle semantics for compatibility.

Outside that protected triangle ring, a source-scaled smooth distance transition joins central skin to the lateral jaw and chin. Skin inside the old edit rectangle may now move: preserving every pixel of that rectangle was the wrong acceptance criterion. Actual lip/contact/enamel pixels, lower neck/clothing and matched laughter remain protected. No oral material, dental placement, speech timeline, head/idle controller or laughter algorithm changes.

## Bounded measurements

The maximum remains 0.75 percent of measured face height with the same estimated hinge and angle. Existing combined head/breath triangle gates remain unchanged. Before/after field inspection shows 3→1 connected moving regions; central bridge weights 0/0→0.493/0.835; global local-displacement gradient 0.349→0.150 and central gradient 0.349→0.126. These are continuity diagnostics, not proof of anatomical reconstruction or owner naturalness.

All inputs are existing cached source geometry, 478-point landmarks, oral calibration and original textures. No new image, analysis, speech or model requests, dependencies, character masks or placement overrides are used. The first raw-float support binding was rejected; preparation now matches the native loader's float32 base-coordinate precision before adding expression deltas. Rejected inputs/results remain preserved.

## Review and reproduction

Use the existing environment and a new output directory from the repository root:

```powershell
$env:PYTHONPATH = 'services/processor'
services/processor/.venv/Scripts/python.exe -m app.workflows.motion.prepare_speech_jaw --source .tools/paired-parallax-100/prepared-v2/package --landmarks .tools/foreground-11/prepared/syntelle/base-landmarks.json --output .tools/jaw-transition-113/reproduction
```

The selected package is `prepared-source-bound/package`, hash `70bf36a1d067c92c72d2418a7a77ae74faa9bedb6dd4f1498d40bc428afe4f0b`. The new resource is version 2; do not rerun historical finalizers or overwrite phase 112 resources/snapshot 772. The [preceding speech-jaw workflow](presence-continuity.md) records the unchanged private channel and timing behavior.

The reviewed build context is `.tools/jaw-transition-113/source`, copied from phase 112 and changed only for the jaw resource type, source-support validation and associated tests. Compile the unchanged original shader source with the pinned toolchain:

```powershell
docker build --pull=false -f .tools/jaw-transition-113/worker.Dockerfile -t syntelle-render-native:jaw-transition-113 .tools/jaw-transition-113/source
```

Source closure, field maps, actual-rendered lip/enamel/laughter identity, CPU/CUDA and matched cached speech videos are retained in the experiment folder. Sixteen actual old107/new GPU states preserve exact protected pixels; CPU/CUDA minimum PSNR is 88.485 dB. Head-control-disabled browser playback complements deterministic Head0 inspection.

Cold Linux-to-Windows bind I/O stalled resource reads during initial offline runs. Copying unchanged packages with native Docker file copy to a labelled local volume resolves the validation path; the failed runs and logs remain evidence. Do not count stalled renders, a reused-transport decoding failure or recording-time cadence as a performance result. Fresh labelled fixtures passed both cached EN/ES routes without widening production deadlines. Complete original A/V recordings, normal-rate crops and matched frame sheets are preserved. Sampled frames retain coherent lip/dental edges without a new visible skin seam or neck detachment; the deformation remains subtle at normal framing. Exact continuity and fixed-head isolation come from the separate field and deterministic render checks, rather than approximately aligned browser videos.

The local review candidate is installed through `.tools/jaw-transition-113/install.ps1`. Its labelled runtime asset volume, `syntelle-jaw-transition-113-assets`, has 120 files verified byte-exact against the selected package. The installed route completes three original cached laughter previews, retains audio/video tracks and rate 1, cleans up its session and makes no provider or microphone calls. The phase 107 API/profile audio and phase 106 player remain unchanged. Both qualified laughter languages stay enabled. The rollback command restores exact phase 112.

Archive through `app.audits.finalize_jaw_transition`. The authoritative external receipt is `.tools/jaw-transition-113-archive.json`; only a successful receipt with `all_hashes_match` confirms PostgreSQL byte restoration. The finalizer archives complete sources/resources, rejected bindings, startup failures, requests/decisions, original cached audio, comparison media, install/rollback commands and exact runtime binaries. Owner visual acceptance remains open; these bounded technical checks do not establish anatomical reconstruction, a new real-time performance result or broader device/future-character readiness.
