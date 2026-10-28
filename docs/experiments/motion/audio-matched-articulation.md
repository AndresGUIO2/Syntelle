# Audio-matched laughter and mandibular articulation (phase 108)

This local review candidate reuses Syntelle's immutable images, paired geometry, oral materials and the two owner-approved Emma recordings. No image, analysis, synthesis or conversation-model request was made. Historical phase-107 resources/profile/runtime remain available. Owner naturalness acceptance and an uncontended real-time comparison remain open.

## Shared preparation and playback

`app.workflows.motion.prepare_mandible` adds optional `mandibular-rig/1` to the existing native motion manifest. Cached 478-point landmarks must match its source anatomy; source, oral/motion builds and geometry hashes bind the companion. A bilateral estimated hinge, source lip-contour union and smooth lower-face/neck weights are derived without manual masks or character conditions. The hinge depth uses a common estimated prior of 0.75 face width, not measured hidden anatomy. Original geometry and all textures remain byte-identical. Head-disabled articulation still works.

The initial target is 2% of measured face height. The existing signed-area gate (>0.25) is checked at five jaw amounts, 27 accepted head combinations and three breathing extremes. Automatic angle reduction retains 0.25104 minimum area and 3.363 px maximum chin excursion at 720; fixed-head open A measures 3.150 px. Earlier frontal-only passes and unsafe combined candidates are retained as rejected diagnostics. No geometry gate was relaxed.

The native worker prepares private `audio-articulation/1` channels before queued PCM playback. Original validated native events remain separate from the existing deduplicated pose timeline. No lip event is retimed, no Rest is inserted and no extra controller or public setting is introduced. A centered 40 ms preparation window smooths jaw targets without delaying audio. A 40 ms gross-jaw bridge starts from the currently visible mandible at adjacent phrase activation; the new phrase's lip/contact timeline takes precedence immediately. Nonverbal pulse envelopes are not passed through this bridge. Calibrated aperture/lower-row motion supplies gross jaw targets; short lip consonants retain syllabic jaw continuity. Upper teeth retain the head frame. Existing lower-row/lip gross displacement is subtracted before additional jaw travel is applied; opposing opaque enamel has a source-derived per-column relative stop.

Acoustic deviations from the local mean provide deterministic residual lip opening and corner tension, capped at 5% of their measured source ranges. Contacts disable these residuals. Flat sustained vowels remain stable. Raw events with identical IDs or different IDs sharing EI retain their timing/context. Loudness cannot change the phonetic base or override MBP/FV contact.

Each original laughter PCM supplies independent 20 ms RMS/periodicity analysis with robust active-interval normalization. Jaw attack/release are 20/80 ms. Jaw opening and amused expression are independent: silent gaps can close the jaw while the expression persists. Reference baked opening is removed from the lip field before the prepared jaw gap is applied. Jaw amplitude is not multiplied by laughter twice. Only the final burst permits lifecycle recovery; phase-102 completion/cancellation bridge, 240 ms cancellation and new-speech priority remain in place. Pause freezes presentation. Ordinary jaw release is 80 ms, behind resting contact geometry.

## Reproduction

From `services/processor`, using the existing virtual environment:

```powershell
.venv/Scripts/python.exe -m app.workflows.motion.prepare_mandible --source ../../.tools/paired-parallax-100/prepared-v2/package --landmarks ../../.tools/foreground-11/prepared/syntelle/base-landmarks.json --output ../../.tools/jaw-articulation-108/a-new-output
```

Preparation refuses an existing output. Use its package hash when deriving the existing reviewed performance profile and the private API package binding. Retain original voice/language/audio entries and listening receipts. Packages lacking the companion keep legacy behavior. CPU and CUDA use the same host-side body/oral attachment calculation before final parallax composition.

Install the reviewed local candidate from the repository root:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .tools/jaw-articulation-108/install.ps1
```

Restore phase 107:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .tools/jaw-articulation-108/rollback.ps1
```

## Evidence and limits

Evidence lives in `.tools/jaw-articulation-108/`. Both original audio hashes, approval receipts, full provider results and native events are retained, with no renewed synthesis or auditory approval. English/Spanish prepared jaw peaks are within 30 ms of corresponding original RMS peaks (one 30-fps frame). `*-waveform.png/json` show exact timing conventions. Normal-speed offline videos and silent 3x-slow mouth GIFs supplement native browser recordings; offline assembly is not a real-time benchmark.

Two preparation tests pass. Native animation/adapter/media race checks pass, including stable vowels, identical IDs, shared-pose IDs, supported acoustic variation, pause, head-disable and new-speech precedence. Oral checks cover Rest/MBP/FV/EI/U/O/A, signed triangle orientation, contact invariance and opposing enamel clearance. Canonical rest is pixel-identical to phase 107. Nineteen packed-720 CPU/CUDA states have minimum RGBA PSNR 88.26 dB. Unsafe early hinge/weight trials, a stale embedded-kernel test, an obsolete 960 test-buffer assumption and a too-short English-followup deadline during Spanish review remain preserved; corrected focused checks pass.

Playful 50/100% completion, interruption and immediate cached speech pass for both laughter languages. Linux WebKit decodes/plays original PCM; this is not full WebKit WebRTC approval. Installed Chromium verifies Emma, both laughter languages, complete preview, the original A/V tracks and playback rate 1.0. Phase-106 player sources and timing functions are unchanged; this iteration adds no presentation buffer or playback-rate adjustment.

The GPU was occupied at 100% by unrelated owner activity. Do not treat correctness-render timings, recordings under contention or previous low-fps comparisons as an isolated 30-fps qualification. Repeat the bounded 30-second comparison when capacity is available. Final naturalness, physical-device and future-character/public-upload validation remain separate milestones.

`app.audits.finalize_audio_matched_articulation` freezes complete sources, rigs, failures, inputs, decisions and comparison media into PostgreSQL and verifies byte-exact restoration. Parent snapshot 766 retains phase 107; resource snapshot 730 retains the unchanged base package. The phase-108 receipt is `.tools/jaw-articulation-108-archive.json`.
