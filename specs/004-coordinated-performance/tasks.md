# Implementation tasks



**Status:** approved · **Date:** 2026-10-27



The owner's explicit implementation request authorizes these derived tasks. Keep existing unrelated changes untouched; frozen source is `.tools/coordinated-performance-103/before-source`.



- [x] T-1 — Add versioned delivery/purpose contracts and Serena identity. (REQ-1, REQ-2, REQ-4)

  - check: shared-contract tests and director v4 validation tests; historical v1/v2/v3 payloads remain accepted.

- [x] T-2 — Implement worker reservations, post-send activation and staged settings. (REQ-2, REQ-3)

  - depends: T-1

  - check: focused native policy/media race tests including failed sends, stale tokens, pause, A→B→A and boundary races.

- [x] T-3 — Implement directed Serena synthesis, compatible caches and default selection. (REQ-1, REQ-4, REQ-7)

  - depends: T-1

  - check: speech bindings/serializer/capability tests and Vue type check; zero automatic retries and no Ava laughter under Serena.

- [x] T-4 — Connect preflight, ordered phrase preparation, activation and settings replacement. (REQ-2, REQ-3)

  - depends: T-2, T-3

  - check: isolated conversation/streaming tests exercise one pending phrase, exact full text, cancellation and replacement.

- [x] T-5 — Coordinate attention, accents and activity presentation. (REQ-5, REQ-6)

  - depends: T-1, T-4

  - check: focused animation/adapter tests preserve limits/contact/laughter and exercise final-head gaze and secondary-motion suppression.

- [ ] T-6 — Qualify Serena audio and install/archive the local review candidate. (REQ-7, REQ-8)

  - depends: T-1, T-2, T-3, T-4, T-5

  - check: bounded experiment receipt, exact-text comparisons, browser audio/controls, latency/cadence report and PostgreSQL restoration hashes.



Coverage: REQ-1 T-1/T-3; REQ-2 T-1/T-2/T-4; REQ-3 T-2/T-4; REQ-4 T-1/T-3; REQ-5/REQ-6 T-5; REQ-7 T-3/T-6; REQ-8 T-6.



T-6 is technically implemented, including local installation and verified PostgreSQL restoration. It remains unticked because full WebKit streaming and owner naturalness acceptance remain open. Voice qualification used 11 attempts / 1,131 characters, with no provider failure or retry. Three English styles and two Serena laughter companions are enabled for review; Spanish explicit styles are unsupported. The initial speech-only laughter rejection remains immutable and was reconciled offline, without another request or budget reset.



- [x] T-7 — Correct slow background yaw drift within REQ-5 after owner feedback. Check: old-source regression fails; brief movement/hold, no wide opposite target, phrase continuity and native race checks pass; matched cached-audio Chromium reviews are retained in phase 105. Owner visual acceptance remains pending.



- [x] T-8 — Select Emma through shared voice configuration and preserve historical identities/capabilities. (REQ-1, REQ-4, REQ-7) Check: catalog/cache/default tests and exact voice-bound native recordings.

- [x] T-9 — Remove inline video recapture while preserving controls and PiP. (REQ-9) Check: same-element audio/video, marker/tone timing, mute/background/PiP/cleanup tests and unchanged mouth timeline hash.

- [x] T-10 — Install, compare and archive the A/V/Emma correction. (REQ-8, REQ-9) Check: local preview with cached speech, byte-verified PostgreSQL restoration and rollback.



- [x] T-11 — Prepare voice/language-bound Emma carcajadas. (REQ-4/7/10) Check: separate two-slot PostgreSQL accounting, exact PCM/events, no retries, retained failures and independently qualified profile entries.

- [x] T-12 — Add natural/expressive speaking presence to existing directors. (REQ-5/6/9/11) Check: brief/quiet timing, unchanged idle, bounds/opposite-sweep/phrase continuity, semantic gaze/accent and native race tests.

- [ ] T-13 — Compare, install and archive phase 107. (REQ-8/9/10/11) Check: cached EN/ES and Playful laughter matrix, controls/audio/marker checks, 30-second comparison, rollback and PostgreSQL restoration.



T-11 is complete: the owner audibly accepted both original clips; profile-v3 enables both languages without new synthesis. T-13 has technical replay/install/archive work complete but remains open for an uncontended real-time check; no overall naturalness/30-fps PASS is claimed.



- [x] T-14 — Add immutable audio-derived laugh/jaw and within-viseme trajectories. (REQ-12/14) Check: pulse timing, quiet gaps, breath robustness, sustained/repeated events, native contacts and recovery tests.

- [x] T-15 — Prepare optional mandibular geometry and CPU/CUDA attachment. (REQ-13/14) Check: source bindings, useful visible excursion, mesh gates, contact/row clearance, rest equivalence and parity.

- [ ] T-16 — Review/install/archive phase 108 with rollback. (REQ-8/9/12/13/14) Check: cached audio comparison, expression/pose matrix, clocks/rate, capacity-qualified playback and verified PostgreSQL restoration.



T-14/T-15 pass focused phase-108 checks. T-16 has local installation, archived comparisons and restoration complete when its receipt exists; it remains unticked for uncontended playback and owner naturalness review. No provider request was made.



- [x] T-17 - Bind/validate laughter spans, normalize complete bank actions and prepare offset-relative envelopes. (REQ-15) Check: failing-then-passing API/admission/adapter regressions and unchanged source PCM.

- [x] T-18 - Add one complete facial lifecycle/direct current-expression recovery with preview policy continuity. (REQ-16/17) Depends:T-17. Check: phase109 native/media race tests for completion, cancellation, pause, repeat and new contacts.

- [x] T-19 - Prepare jaw-neutral source companion and compose once on CPU/CUDA. (REQ-17) Depends:T-18. Check: cached preparation/Jacobian/attachment/endpoint/rendered/parity checks without gate/amplitude reduction.

- [x] T-20 - Bind preview busy state to playback lifecycle. (REQ-18) Depends:T-18. Check: frontend lifecycle/late-start/pause/error checks and types.

- [ ] T-21 - Review production replays, serial30s comparison, install and byte-restore archive. (REQ-15/16/17/18) Depends:T-19/T-20. Check: both languages/manual+automatic, settled baseline/Playful/smile, complete and interrupted recovery, original A/V and verified PostgreSQL receipt. Owner naturalness remains separate.


T-21 technical replay, installation and archive work is complete when the phase-109 receipt exists; its checkbox remains open for the owner naturalness gate. No external provider request was made.

- [x] T-22 - Reproduce and repair settled-smile and whole-neck mandibular regressions. (REQ-13/17/18) Check: failing-before/passing-after source-bound neck-isolation and settled-smile renders, retained dental/lip contacts and useful fixed-head chin motion. Done 2026-10-28: exact reported gap reproduced; shared-contour mouth sampling, strict continuous geometry gate and ordered CUDA snapshot pass; lower-neck jaw travel is zero with useful chin motion retained.
- [ ] T-23 - Review/install/archive the regression correction. (REQ-8/9/13/17/18) Depends:T-22. Check: internally inspected matched before/after images and cached playback, CPU/CUDA agreement, installed smile/laughter/speech controls and PostgreSQL byte restoration. Owner naturalness remains open.

T-23 technical evidence is in `.tools/laughter-correction-110/` and its external archive receipt when present. Keep the owner naturalness gate open; discarded prototypes and the rejected phase109 remain available.

Owner rejected phase110 on 2026-10-28 for unnatural laughter and dental deformation. T-23 remains unaccepted. Restore the exact pre-articulation phase107 runtime rather than making another unreviewed composition change.

- [x] T-24 - Restore the pre-articulation phase107 local runtime and preserve the rejected candidates. (REQ-8/9) Check: exact frozen image/profile/package hashes, unchanged original Emma audio, retained synchronized player, cached preview and PostgreSQL byte restoration. Done 2026-10-28: three restored cached previews pass; snapshot771 restores all149 files exactly. This is rollback, not renewed naturalness acceptance.

- [x] T-25 - Repair semantic attention release at phrase/speech handoff. (REQ-19) Check: reproduced2.4-degree jump; adjacent-phrase, completion, pause and unchanged gesture-speed regressions. Done2026-10-28: handoff jump0, existing320ms release and admitted gesture paths retained; staged race suite passes.
- [x] T-26 - Shorten idle holds within the existing envelope. (REQ-21) Check: exact-seed long-hold/activity trace, unchanged speaking trace/bounds/speeds and accessibility behavior. Done2026-10-28: longest quiet9.63→6.33s; exact speaking traces preserved.
- [x] T-27 - Prepare and render optional subtle body-only speech jaw. (REQ-20) Check: source-bound anatomy, subtle visible chin travel, zero lower-neck/clothing and oral-material/contacts changes; zero channel during laughter/rest and matched laughter pixel identity. Done2026-10-28: actual old107/new GPU mouth pixels exact; matched laughter frames exact; subtle active skin motion and stationary lower-neck/clothing pixels pass; no old oral-mandible path activated.
- [ ] T-28 - Build/review/install/archive the scoped phase107-based candidate. (REQ-8/9/19/20/21) Depends:T-25/T-26/T-27. Check: baseline source closure, identical cached audio/A/V tracks, CPU/CUDA, speech/idle/laughter playback inspection, local rollback and PostgreSQL byte restoration. Owner naturalness remains open.

T-28 technical records are `.tools/presence-continuity-112/` and its external archive receipt when present. Keep owner naturalness open; no strict performance/device/future-character gate is closed by this bounded correction.

Owner feedback identifies a remaining frozen middle skin band in phase112. Its whole-oral-rectangle pixel gate protected too much skin; actual lips/teeth remain protected under REQ-22 while surrounding skin may move.

- [x] T-29 - Reproduce and measure the stationary jaw-skin band. (REQ-22) Check: source-bound field/flow inspection and central/lateral bridge measurements on current112. Done 2026-10-28: the old crop protects 49 unnecessary source pixels below the actual lip support; moving skin splits into three disconnected regions with zero central weights.
- [x] T-30 - Prepare versioned actual-lip protection with continuous skin influence. (REQ-20/22) Depends:T-29. Check: old-case failing/new passing continuity/support sampling, actual lips/enamel/contact pixels, geometry gates, lower-neck/clothing and restored laughter identity. Done 2026-10-28: source-bound speech-jaw/2 joins one moving region, retains the same hinge/angle/0.75-percent amplitude, passes three preparation tests, 54 contact-support cases, legacy loading and stale/frozen-field rejection. Sixteen actual GPU states preserve lip/enamel, neck/clothing and inactive/laughter pixels against the original107 binary; minimum CPU/CUDA PSNR 88.485 dB.
- [ ] T-31 - Review/install/archive the focused transition correction. (REQ-8/9/20/22) Depends:T-30. Check: scoped112 source closure, same cached speech/audio/rate, CPU/CUDA, head-fixed before/after inspection, preserved112 rollback and PostgreSQL byte restoration. Owner naturalness remains open.

T-31 technical review and installation are complete: fresh EN/ES cached speech/laughter routes, sampled matched frames and three installed cached previews pass with original tracks/rate and no provider calls. The labelled native runtime volume's 120 package files match byte-exactly. `.tools/jaw-transition-113-archive.json` confirms archival/restoration only when present and successful. Keep final owner naturalness acceptance open.

## Phase118 approved implementation

- [x] T-39 - Freeze117 and record matched cached baseline. (REQ-34) Check: exact source/runtime receipts and microphone-free player capture.
- [x] T-40 - Implement optional acoustic presence and posture-relative gesture composition. (REQ-31/32/33) Check: deterministic clock/bounds/continuity/prefix/pause tests; legacy tests unchanged.
- [x] T-41 - Compare/build/install/archive and publish the scoped candidate. (REQ-34) Check: matched playback, bounded cadence, unchanged PCM/oral resources, production build, PostgreSQL restoration, rollback, commit/push.
