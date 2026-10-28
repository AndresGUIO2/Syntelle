# Verification — 004 Coordinated Serena performance

**Date:** 2026-10-27 · **Verdict: FAIL for complete qualification; local review candidate installed.**

The owner's bounded validation commands supersede a generic full-suite default. No production promotion, emotional-voice capability or future-character readiness is claimed.

## Command output

Current-run focused results, preserved under `.tools/coordinated-performance-103/`:

```text
api-final.log (speech + isolated integration + race):
ok syntelle/api/internal/speech
ok syntelle/api/internal/conversation
ok syntelle/api/internal/streaming
api-release.log and protocol-compatibility.log: passed
native-release.log:
ok syntelle/render-worker/internal/nativeanimation
ok syntelle/render-worker/internal/nativemedia
ok syntelle/render-worker/internal/nativeadapter
worker-transport-release.log:
ok syntelle/render-worker/cmd/render-worker
processor-release.log: 3 passed
Vue: vue-tsc --noEmit passed; scoped ESLint exited 0; vite build passed
Chromium baseline/candidate controlled replay: passed
Serena cached replay: 2 reservations / 2 activations; passed
Installed candidate: Serena; both language-bound laughter capabilities available; passed
Linux WebKit PCM: active audio samples 58; passed
Linux WebKit full WebRTC/video: media timeout; failed
Voice experiment: six plain + three English directed controls; both nonverbal companions valid
```

The initial worker-command test used `--network none` and lacked its expected Docker interface; its focused rerun with a Docker network passed. A baseline comparison exposed an unknown activity command in old workers; capability negotiation fixed it and the comparison passed. Preliminary failures are retained rather than overwritten.

## Traceability

| Requirement | Tasks | Current-run evidence | Status |
|---|---|---|---|
| REQ-1 | T-1, T-3 | Contract/default/cache tests; regional catalog snapshot 714; six native Serena controls; installed Chromium session reports Serena | Covered technically; audible acceptance pending |
| REQ-2 | T-1, T-2, T-4 | Reservation/policy/media race tests; successful-send-only commits; stale/duplicate acknowledgement tests; paired cached Serena replay records two reservations and activations | Covered |
| REQ-3 | T-2, T-4 | A→B→A invalidation, send-boundary serialization, pause/accessibility and same-text replacement tests; 34-phrase isolated integration | Covered technically; owner interaction review pending |
| REQ-4 | T-1, T-3 | Serializer/cache/isolation/no-fallback/capability tests pass; English reassuring/curious/amused return valid native audio/events; Spanish styles remain disabled | Technical qualification covered; audible delivery acceptance pending |
| REQ-5 | T-5 | Native accent/final-head-gaze/reassurance tests; matched Ava videos; real cached Serena playback; existing motion/contact limits | Covered technically; naturalness acceptance pending |
| REQ-6 | T-5 | Activity forwarding and final-head attention tests; old-worker command exclusion; no generated nod or artificial delay | Covered technically |
| REQ-7 | T-3, T-6 | Exact voice/language lookup; installed EN/ES previews enabled; original 800/950 ms audio retained; initial speech-only rejection reconciled offline with a bounded nonverbal gate | Technical qualification covered; audible laughter acceptance pending |
| REQ-8 | T-6 | Chromium videos; 30-second comparison; actual WebKit PCM; before/current sources/images; PostgreSQL archive/restoration receipt | Partial: full WebKit streaming and complete vocal/owner review remain open |

No tasks lack requirement references. **T-6 remains unticked.** The experiment used 11 attempts / 1,131 submitted characters, zero provider failures, zero retries and no budget reset. Spanish styles were skipped before calls. The first English laugh failed an ordinary speech-only phoneme check despite successful provider completion. Its original failed database record/audio/events remain immutable; offline nonverbal validation permits valid Rest timelines only for a bounded full-length laughter cue. Ordinary spoken-phoneme validation remains strict, including native preparation. The real preview exposed the same speech-only assumption in the native adapter; its bounded full-laughter path was corrected and all four Serena EN/ES Playful 50/100 previews then passed. The qualification command now halts on provider/uncertain failures, while preserving application-rejected completed outputs without preventing distinct approved slots. The reconciliation event documents this correction. A matched cached video compares plain/reassuring Serena on identical text, facial direction and framing.

## Measurement

The serial, isolated 30-second cached comparison measured 29.73 fps baseline and 29.60 fps candidate against the 30 fps target. P95 decoded-frame gap was 50 ms for both; maximum gaps were 66.7/83.4 ms. Decoded first-audio onset was 157–160/159–228 ms. PCM-publication gap estimates were -8 to -6/-11 to -2 ms, within scheduling/block uncertainty. Candidate worker samples averaged 39.04% of one CPU core and about 836 MiB; baseline 36.27% and about 994 MiB. These samples exclude live model/TTS latency and do not establish sustained performance or a general resource improvement. Full sample methods/raw data are in `comparison.json`.

## Archive and rollback

`app.audits.finalize_coordinated_performance` stores complete source/input/output bytes, provider operations/results, rejected audio, decisions, videos and both runtime binaries in PostgreSQL. It separately snapshots unchanged prepared resources and restores both snapshots to new directories, verifying every SHA-256. The authoritative result is `.tools/coordinated-performance-103-archive.json`; absent or unsuccessful restoration invalidates archive completion.

The previous phase-102 image/profile is preserved. Run `.tools/coordinated-performance-103/rollback.ps1` for rollback or `install.ps1` to restore the review candidate. `.env` and original visual resources are unchanged; the explicit Compose override selects the candidate.

## Next actions

- Owner review of the installed Serena voice, manual next-phrase emotions and visual naturalness.
- Investigate full WebKit WebRTC/video with bounded environment diagnostics; PCM playback alone does not close this gate.
- Keep Spanish explicit styles disabled. Review the English styles and both Serena laughter companions audibly; preserve all fixed slots and do not retry them or reset the allowance.
- Return to implementation/verification for remaining gates before claiming the complete performance plan qualified.

## Owner posture correction, phase 105

REQ-5 / T-7: the old-source background-drift regression fails at 672/871 moving frame steps; the correction passes at 139/871, with a maximum uninterrupted adjustment of 0.533 seconds. Opposite-side sweep, bounds and short-phrase continuity checks pass; native animation/adapter/media race suites pass. Matched cached Serena before/after Chromium recordings pass with identical text, audio sequence and settings. The local worker was installed with its exact preceding image preserved. This is a technical correction candidate; owner naturalness remains pending. The phase-104 mouth presentation diagnosis is unchanged, and no full-feature PASS is inferred. See docs/experiments/motion/brief-posture.md and .tools/brief-posture-105-archive.json for sources and byte-verified restoration.


## Owner synchronization and Emma correction, phase 106

REQ-1/4/7/8/9 and T-8/9/10: Emma HD passes two separately reserved regional EN/ES synthesis checks without retry; complete native PCM/events and true historical cache identities are preserved. No explicit Emma styles or laughter are enabled. Inline playback now draws the original native A/V element directly, without recapture or rate/timeline changes. Eight desktop/three mobile player checks, focused API/native race checks, types/lint/build, exact cached native Emma replay and Linux WebKit PCM pass. Five native marker/tone pulses per version show decoder-to-visible median 16.8 to 0 ms (max 33.4 to 16.7 ms), with display-tick uncertainty and no acoustic/phoneme guarantee. Local install/rollback and byte-restoration evidence are recorded by the phase-106 finalizer. T-6 and overall owner/device/WebKit-streaming acceptance remain open. See `docs/experiments/motion/mouth-presentation-and-emma.md`.


## Phase 107: Emma laughter and conversational presence

REQ-10: two fixed nonverbal requests / 20 characters succeeded without retry; original PCM/native events, independent language bindings and Linux WebKit playback pass. Auditory classification is pending because this environment cannot consume audio; an owner question is pending and no accepted listening receipt is fabricated. Normal-demo laughter remains disabled until that reply. REQ-11: new brief/quiet timing, approved-energy priority, bounds, diagonal variety, phase continuity and final gaze/accent checks pass, with 212/871 moving state steps versus 139/871 before. Focused native/API/shared race, ten processor, three desktop/two mobile and type checks pass. Real cached English deliveries and authored Spanish directions, both Playful laughter intensities, interruption and subsequent speech pass isolated functional replay. REQ-9 source hashes/rate/original tracks remain intact; five marker pulses show zero added decoder-to-canvas delay, but received-image lag reaches 367-550 ms under unrelated 100-percent GPU occupancy. The serial 30-second comparison is about 2.43/2.23 fps and is not an isolated 30-fps qualification. Sources/evidence/rollback are archived and byte-restored by the phase-107 finalizer. T-11/T-13 and overall owner/device/streaming acceptance remain open.


### Phase-107 audible acceptance and activation

The owner listened to both original Emma clips and confirmed "Ambos suenan bien". Hash-bound review receipts and immutable profile-v3 now activate both languages in the normal demo. Installed checks verify language availability and full manual preview completion for English and Spanish, preserving the original A/V tracks and rate. There were no additional synthesis calls: the experiment remains exactly two requests / 20 characters. T-11 is complete. Prior pending-review evidence stays archived; overall naturalness, uncontended 30-fps and device gates remain open.


## Phase 108: audio-matched laughter and optional mandibular rig

**Date:** 2026-10-27 local / 2026-10-28 UTC. **Verdict: FAIL (overall acceptance remains partial).** This does not indicate a failing functional candidate: the explicit real-time/owner gates remain open. Focused commands: processor `pytest tests/test_prepare_mandible.py`, native animation/adapter/media race tests, native cached-geometry/contact tests and `TestMandibleCPUCUDAPacked720`; full outputs/status are retained in phase 108.

| Requirement | Tasks | Evidence from this iteration | Status |
|---|---|---|---|
| REQ-1/4/7/10 (current Emma overrides) | T-8/11/14 | Exact original profile laughter/voice/capability identity; EN/ES installed preview and Linux WebKit PCM | Covered for unchanged scope |
| REQ-2/3 | T-2/4 | Native media/policy race suite; unchanged API sources/binary | Covered for unchanged scope |
| REQ-5/6/11 | T-5/7/12 | Native animation/adapter race suite, unchanged attention/gesture speeds, head-disabled jaw | Covered for unchanged scope |
| REQ-8 | T-16 | Retained baseline, full PostgreSQL restoration receipt and comparison media; uncontended playback and owner review pending | Partial |
| REQ-9 | T-9/16 | Original A/V tracks and rate 1.0 in installed checks; player/timeline source byte equivalence | Covered; no new hardware timing qualification |
| REQ-12 | T-14/16 | Original EN/ES peaks within 30 ms; independent jaw/face, gaps, Playful 50/100, cancellation/followup, pause tests and normal/slow clips | Covered technically; owner naturalness pending |
| REQ-13 | T-15/16 | 405 joint configurations, area >0.25, fixed-head A 3.150 px, canonical-rest pixel identity, contacts/clearance and CPU/CUDA PSNR >=88.26 dB | Covered technically; owner naturalness pending |
| REQ-14 | T-14/15 | Raw repeated/shared-pose native IDs retained; stable-vowel/modulated-audio/contact tests, calibrated <=5% residual and unchanged tracks/timeline | Covered technically |

Unticked tasks: historical T-6/T-13 and current T-16. No new scope-only task. Next action: repeat a bounded real-time comparison with free GPU capacity, then owner visual acceptance; no provider retry or regeneration is needed. See `docs/experiments/motion/audio-matched-articulation.md` and the phase-108 restoration receipt. Earlier verification records remain historical.

### Current focused output (exit status 0)

```text

preparation-tests.log
..                                                                       [100%]
2 passed in 7.55s

phrase-handoff-tests.log
ok  	syntelle/render-worker/internal/nativeanimation	3.451s
ok  	syntelle/render-worker/internal/nativeadapter	2.506s
ok  	syntelle/render-worker/internal/nativemedia	1.073s

parity-release.log
=== RUN   TestMandibleCPUCUDAPacked720
2026/10/28 01:14:16 INFO Native renderer selected renderer=cuda
--- PASS: TestMandibleCPUCUDAPacked720 (12.53s)
PASS
```


## Phase 109 - complete laughter and direct expression recovery (2026-10-28)

Status: technical local-review candidate; final owner naturalness acceptance remains open.

| Requirement | Current evidence |
|---|---|
| REQ-15 complete action/span | API zero/partial-strength regressions; real preparation-lane red/green test; production-inputs, production-decisions and production-envelopes; actual manual/automatic browser replays |
| REQ-16 one lifecycle/live return | Completion, interruption, pause, repeated preview, new-contact and preview-end race tests; settled Neutral/Playful/manual-smile frame sequences |
| REQ-17 composition/ownership | Source-bound companion at amplitude 1; Jacobian/inverse and combined-field checks; exact baseline restoration, endpoint and enamel-sentinel regressions; 50 CPU/CUDA cases including head bounds |
| REQ-18 delivery/controls | Four source-UI Chromium tests, desktop/mobile installed previews, Linux WebKit original-PCM audio; final isolated serial 30-second comparison; local install/rollback and PostgreSQL receipt |

Final cadence: baseline 29.93 fps, candidate 29.80 fps; P95 gaps both 49.9 ms, largest gaps 99.9/72.2 ms. Cached received-audio onset 210.1/193.4 ms. Original audio/video tracks and rate 1 retained. These measurements exclude live model/TTS latency and do not qualify a strict sustained 30 fps target. Fifty CPU/CUDA states have minimum PSNR 88.26 dB. Prepared jaw peaks are within 30 ms of measured original RMS peaks; sampled-frame routes are within 23.34 ms.

The production-path review exposed a dropped span in the asynchronous queue and a later enamel ownership leak; both have failing-before/passing-after regressions. A zero cue is also omitted before consuming the reply allowance. Source images, rig/geometry, opaque oral materials and original Emma recordings remain unchanged. Normal-speed recordings and matched slow mouth crops are preserved. See docs/experiments/motion/complete-laughter-recovery.md and the phase-109 archive receipt for the reproducible technical result. Final naturalness, physical-device/full-WebKit streaming and future-character acceptance remain separate.

## Phase 110 owner-reported regression correction (2026-10-28)

**Owner rejection:** phase110 remains visually unacceptable: laughter/dental appearance is worse than the preceding version. The local runtime is restored to phase107, withdrawing the new jaw as well. Passing the checks below does not supersede this rejection. New restoration evidence belongs in `.tools/laughter-restore-111/`.

Overall acceptance remains **FAIL / pending owner naturalness**; this does not erase the reproduced defects or promote the historical candidate. The bounded correction has passing technical evidence:

| Requirement / task | Evidence produced in this correction |
|---|---|
| REQ-13 / T-22 | Red lower-neck movement 2.329 px; corrected jaw-only movement 0 px, open-A chin 3.150 px. Cached 478-point anatomy, two preparation checks and three native attachment/contact checks. |
| REQ-17 / T-22 | Exact full-laughter/Jaw0 screenshot reproduction; closed-smile/full-reference source coordinates share one material contour. Monotonic source-scale aperture 18.83→58.63 px, unchanged 58.65 px source peak. Exact signed-area interpolation minimum and correspondence regressions pass. |
| REQ-17/18 / T-22 | Rejected ghosted RGB, invalid corner diagonal and tiny-entry attempts retained. CPU endpoint, frame-feedback and independent enamel tests pass. A mouth-only test rejects unsynchronized CUDA snapshots 25.53 dB; ordered-stream copy passes with 76.79 dB minimum over eight mouth states. Final worker 15-state whole-frame minimum 88.24 dB. |
| REQ-9 / T-23 | Original Emma WAV/PCM/events and seven timing/playback source files byte-identical to frozen 109; actual A/V tracks and rate 1 retained. No synthesis/model/image calls. |
| REQ-17/18 / T-23 | Final ordered-image EN/ES production replays: six complete cached previews, two fixed-head cached speech replays; dense full-recording sheets and selected-expression/manual-smile recovery inspected. |
| REQ-8 / T-23 | Local stable image/resource installation, preserved rejected 109/snapshot 769 and explicit rollback; archive and exact restoration identified by the external 110 receipt when present. |

Current output: `.tools/laughter-correction-110/`; document: `docs/experiments/motion/laughter-regression-correction.md`. Recording-time frame diagnostics include capture/GPU test activity and are not a new performance qualification. Historical unchecked owner/device/voice qualification tasks are not silently closed by this regression repair. No new scope or untraced task is introduced.

## Active restoration after owner rejection

T-24 restores exact phase107 runtime/profile/package, with original Emma EN/ES recordings and phase106 synchronized player. It withdraws the new jaw/within-viseme/laughter-composition work rather than claiming another visual repair. Three installed cached previews and exact running-image checks pass. PostgreSQL snapshot771 restores149 files byte-exactly; original runtime766/resources730 and rejected770 remain preserved. Receipt: `.tools/laughter-restore-111-archive.json`. Owner visual acceptance remains open.

## Phase112 scoped restored-base candidate

Overall owner naturalness remains pending. Technical checks cover the current scoped requirements:

| Requirement / task | Fresh evidence |
|---|---|
| REQ-19 / T-25 | Frozen107 red2.399-degree handoff; green0-degree first-frame jump with existing320ms release; unchanged gesture trajectory test; staged nativeanimation/adapter/media race suites pass. |
| REQ-21 / T-26 | Actual seed60s idle215→347movingsteps; longest nearly-static9.63→6.33s; old speaking trace hashes identical, existing bounds/speeds/quiet/reduced-motion checks retained. Frame counts are diagnostic, not activity targets. |
| REQ-20 / T-27 | Source-bound0.75%FH jaw; original textures/geometry/oral/reference resources unchanged. Conservative oral sampling protection; fixed-head chin movement1.36px for A at720p, neck/clothing pixels unchanged. Original107 GPU binary vs new GPU has exact matched mouth/teeth pixels and exact laughter/rest frames, not merely matching new CPU/GPU implementations. |
| REQ-8/9/19/20/21 / T-28 | Explicit frozen107 source-patch allowlist; shader source compiled with original toolchain after rejecting stale archived PTX;12CPU/CUDA states minPSNR88.49dB; actual EN idle/speech/head-control-disabled/laughter and ES speech/laughter recordings, original A/V tracks/rate1, no new providers. Local candidate/rollback and PostgreSQL receipt identify the installed result. |

Evidence: `.tools/presence-continuity-112/` and `docs/experiments/motion/presence-continuity.md`. Only the reproduced semantic handoff is claimed repaired; transport scheduling and other possible perceptual jolts are not independently ruled out. The head-control-disabled browser case is not proof of mathematical Head0; deterministic offline fixed-head geometry is the isolation proof. No sustained-performance, device or future-character acceptance is claimed.

## Phase113 connected lower-face skin transition

REQ-22 / T-29 reproduces the owner's stationary middle skin band. The phase112 edit-rectangle protection extended 49 source pixels below the measured lip support and left three separate moving regions. The versioned `speech-jaw/2` source-bound support and smooth exterior transition in T-30 connect those regions while preserving the existing hinge, angle, 0.75-percent limit and speech clock.

Current evidence in `.tools/jaw-transition-113/`: three preparation tests; 54 lip/enamel support cases across poses, smiles and Playful; seven fullface/inactive states; positive central bridge and stationary lower-neck/upper-face geometry; stale support and deliberately frozen bridge rejection; legacy version-1 loading. The final native-precision binding is `70bf36a1d067c92c72d2418a7a77ae74faa9bedb6dd4f1498d40bc428afe4f0b`; the earlier raw-float draft remains rejected evidence.

The actual frozen107 GPU binary and candidate GPU match exactly for frontal lip/enamel pixels, lower-neck/clothing pixels and complete rest/manual-smile/laughter states, including accepted head-boundary laughter. Sixteen CPU/CUDA states have minimum PSNR 88.485 dB. Active central skin pixels now change relative to phase112. Source closure permits only the jaw resource type, its validation and its tests; deformation, oral/laughter rendering, head/idle and playback timing methods remain unchanged. These are focused continuity and preservation checks, not anatomical reconstruction or owner naturalness acceptance.

T-31 technical browser review and local installation are complete. Fresh isolated EN/ES fixtures pass cached speech with head controls disabled and complete original Playful laughter, with no errors/provider/microphone calls, session cleanup 204 and original A/V tracks/rate 1. Matched frame sheets show coherent lip/dental edges without a new visible skin seam or neck detachment. These are sampled observations; the preserved complete synchronized videos remain available for owner review, and browser alignment is insufficient for exact pixel claims. The installed phase113 route completes three cached previews with the exact `e83c2bc5...` worker image; all 120 files in its labelled native asset volume match package `70bf36a1...` byte-exactly. Both qualified laughter languages remain enabled and phase112 rollback is preserved.

Cold Windows bind reads stalled offline validation and a reused fixture connection failed decoding; those attempts remain preserved. Fresh Docker-local resources and fresh labelled fixtures resolved validation without changing presentation deadlines. No new provider, image, analysis, synthesis or model request is made. `app.audits.finalize_jaw_transition` archives complete source/resource/failure/media bytes and exact binaries; only a successful `.tools/jaw-transition-113-archive.json` receipt confirms PostgreSQL restoration. T-31 remains unticked for final owner naturalness acceptance. No new performance/device/future-character qualification is claimed.
