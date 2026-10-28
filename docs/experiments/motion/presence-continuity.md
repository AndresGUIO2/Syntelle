# Restored-base conversational continuity and subtle speech jaw

Phase 112 uses the frozen phase-107 source/runtime as its baseline. The owner likes that restored laughter and requests fewer speaking jolts, a less static idle and subtle speech jaw articulation. The rejected phase-108–110 oral/laughter algorithms are not included in the candidate build.

## Scoped changes

The attention director used only the current phrase's semantic suppression. At a phrase boundary, it could expose background posture by 2.399 degrees in one 30 fps frame. The existing 320 ms release envelope now continues that suppression across phrase/completion boundaries. Gesture trajectories and speeds are unchanged. This repairs a reproduced cause, without claiming every possible perceptual jolt is solved.

Idle holds change from 2.6–4.6 to 1.6–2.6 seconds. Existing adjustment speeds, amplitudes, target policy and listening/accessibility behavior remain. At the actual source seed, the longest nearly-static stretch changes from 9.63 to 6.33 seconds. Activity counts are diagnostics, not realism targets; unchanged speaking traces and angular travel preserve stronger speaking activity.

Optional `speech-jaw/1` is a body-only resource/channel. It uses cached 478-point MediaPipe anatomy, existing depth/projection and measured viseme apertures. Maximum chin excursion is 0.75 percent of source face height, approximately 1.36 px for open A and 1.82 px at maximum in framed 720p playback. This explicitly replaces the earlier minimum-3-px experiment with the owner's requested subtle range.

The channel samples the existing viseme timeline through a centered 40 ms window. Brief MBP/FV contacts may leave the skin-jaw curve coarticulated; the actual mouth/contact events are never removed or retimed. Sustained sounds stay stable. The channel is zero for preview, idle, reduced motion and laughter/its recovery, including cue-span entry protection.

Neck/clothing and upper-face weights stay zero. Every source triangle overlapping the oral sampling region and its measured filtering guard is protected. Unchanged triangles preserve the original source-copy path on CPU/GPU. No dental movement, oral jaw residual, oral material, mouth geometry or laughter companion is added. Upper/lower teeth and lip contacts remain exactly the preceding presentation.

## Reproduction and verification

Run from the repository root using the existing environment and a new output directory:

```powershell
$env:PYTHONPATH = 'services/processor'
services/processor/.venv/Scripts/python.exe -m app.workflows.motion.prepare_speech_jaw --source .tools/paired-parallax-100/prepared-v2/package --landmarks .tools/foreground-11/prepared/syntelle/base-landmarks.json --output .tools/presence-continuity-112/reproduction
```

The selected output is `speech-jaw-protected/package`; earlier preparations are retained failed/intermediate evidence. Preparation rejects the discarded oral-mandible/laughter companion and verifies unchanged original textures/geometry/oral data. The existing combined head/breath geometry gate is not weakened.

The runtime build context is `.tools/presence-continuity-112/source`, copied from frozen phase107 with a small explicit patch allowlist. `verify-source.py` checks this closure. Engine, oral, speech, laughter and shader sources remain byte-identical to the baseline. Compile the original shader source with its pinned original compiler/flags: the archived repository PTX was stale and its rejected first build is preserved. Do not substitute a broad build of the mixed development tree for this reviewed closure.

```powershell
docker build --pull=false -f .tools/presence-continuity-112/worker.Dockerfile -t syntelle-render-native:presence-continuity-112 .tools/presence-continuity-112/source
& .tools/presence-continuity-112/install.ps1
& .tools/presence-continuity-112/rollback.ps1
```

Rollback restores exact phase107 through the preserved phase111 command, not the rejected109/110 candidates. Normal `dev start` reuses the installed stable image/resource bindings. API/player images, Emma audio/profile content and phase106 A/V playback remain unchanged.

Evidence in `.tools/presence-continuity-112/` includes red/green handoff and pixel tests, preparation/neck isolation, race suites, actual old-binary/new-binary GPU comparisons, source closure, normal-speed cached speech/laughter and idle recordings. The actual107 GPU binary produces identical matched laughter/rest pixels; active speech-jaw frames retain identical mouth pixels and stationary lower-neck/clothing pixels. CPU/CUDA comparisons cover frontal and accepted head bounds. No new provider, image, analysis, synthesis or model requests are used.

Archive with `app.audits.finalize_presence_continuity`; its external receipt records PostgreSQL byte-exact restoration. Preserve source/runtime snapshot766, resources730 and restoration771. Owner naturalness, broader performance, physical devices and future-character qualification remain separate.
