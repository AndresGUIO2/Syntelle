# Coordinated Serena performance

**Status:** approved · **Date:** 2026-10-27

The owner explicitly approved implementation of the complete conversation plan, including the bounded 14-attempt / 4,000-character speech experiment. This authorization covers its specification, implementation tasks and verification; no further planning gates are needed.

- REQ-1: New Syntelle sessions use Serena in English and Spanish without relabeling historical audio. Given an old saved configuration, when a new session opens, then the runtime selects Serena and existing Ava evidence stays unchanged.
- REQ-2: One worker-approved phrase direction governs synthesis and facial activation. Given a proposed direction, when reserved before TTS, then it does not change played state until its first PCM is successfully sent; at most one unactivated reservation exists.
- REQ-3: Manual emotion changes take effect at the next phrase boundary. Given audible speech and a newer setting, when the next phrase activates, then its approved voice/face direction uses the latest prepared revision without losing or repeating text; idle changes apply immediately.
- REQ-4: Directed voice controls remain private, capability-bound and cache-safe. Given an unsupported style/language, then plain delivery is chosen before synthesis; a provider error never triggers another paid attempt or voice substitution.
- REQ-5: Existing gestures, posture, gaze and facial accents form one audio-clock performance. Given an intentional beat, then competing movement reduces smoothly, gaze follows final head composition, and articulation/laughter priorities and existing limits remain intact.
- REQ-6: Listening/waiting presentation follows actual lifecycle signals. Given listening or preparation, then attention uses existing safe resources without automatic agreement nods or artificial delay.
- REQ-7: Serena laughter is voice/language-bound. Given no qualified matching companion, then audible laughter is unavailable; Ava audio is never used under Serena.
- REQ-8: Verification and rollback preserve complete evidence. Given the new candidate, then focused automated checks, matched video/audio comparisons, bounded cadence/latency measurement, and PostgreSQL byte-verified restoration establish its technical result while owner/device acceptance stays separate.

Non-goals: image generation/analysis, new geometry/materials/models/dependencies, public endpoints, cloud deployment and future-character qualification.

Owner correction (2026-10-27), within REQ-5: background head posture must use short bounded adjustments with genuine holds, without repeated wide side-to-side traversals or an automatic negation pattern. Semantic nod/shake timing remains unchanged. The mouth A/V diagnosis is tracked separately in phase 104; this correction makes no synchronization claim.

Owner correction (2026-10-27), superseding REQ-1 voice selection: select Emma Dragon HD Latest, preserving true historical Serena/Ava/Andrew identities. Add REQ-9: correct received presentation A/V synchronization without slowing mouth trajectories or audio tempo. Inline composition must draw from the same native A/V playback element, without recapturing video into a second playback clock; preserve backgrounds/blur, mute, interruption and picture in picture. Verify with cached marker/audio and actual native speech. No provider styles/laughter may be transferred between voices.


Owner-approved phase-107 extension (2026-10-27):
- REQ-10: Prepare Emma laughter independently for English/Spanish under a separate two-attempt experiment, preserve complete accepted/rejected native outputs without retry, and enable each technically and audibly qualified language through the existing profile.
- REQ-11: Speaking presence combines a natural baseline and stronger meaningful accents, using 280-500 ms posture changes and 0.8-1.6 second holds (1.6-2.8 seconds for reassurance/low approved energy), while retaining quiet passages, qualified bounds, intentional gesture speeds, rolling limits, cross-phrase continuity and phase-106 A/V synchronization.
The explicit implementation request approves specification, plan and derived tasks together; no new planning approval is required. Existing owner/device acceptance remains separate.


Owner-approved phase-108 articulation extension:
- REQ-12: Existing approved laughter audio drives separate pulse-aligned jaw and facial envelopes without double attenuation, per-pulse neutral recovery or altered audio speed. Completion/cancellation retain phase-102 recovery and new-speech priority.
- REQ-13: An optional source-bound mandibular rig moves chin/lower cheeks coherently, independently of head gestures, while preserving opaque dental attachment and MBP/FV contacts. Open-A at fixed head shows at least 3 px chin travel at 720 px, starting from a 2-percent face-height range and reducing only for existing geometry safety gates.
- REQ-14: Preserve native event context and add small, source-bounded within-viseme lip/jaw variation, without artificial Rest insertion, repeated pumping of sustained sounds, changed contact timing or new assets/models/provider requests.
The explicit implementation request approves the associated spec/plan/tasks. Validate only Syntelle; preserve current audio approvals and legacy packages.

## Owner-approved complete laughter repair (phase 109)

- REQ-15: Standalone and appended approved laughter use a validated PCM sample span and identical relative-time analysis; accompanying speech/cues cannot suppress it. Zero requests are omitted before allowance/audio mutation; positive admitted fixed-bank laughter is a complete action.
- REQ-16: Full non-jaw laughter expression has a120ms entry and independent lifecycle; one320ms normal return continues beyond PCM as needed, while interruption remains immediate audio cancellation plus240ms visual release. Preview, pause, replay and new speech preserve visible-state continuity and lip priority.
- REQ-17: Source-bound jaw-neutral laughter and the live worker-approved selected baseline compose in shared coordinates; the qualified mandible is applied once, without a forced neutral waypoint, duplicated expressive overlays, new imagery or reduced geometry gates.
- REQ-18: Preview busy state follows existing playback/error/interruption events; production-path and rendered dynamic tests replace playback-only acceptance. Preserve phase108/snapshot768, full PostgreSQL restoration and owner visual acceptance as a separate gate.

## Owner-reported phase-109 regressions (2026-10-28)

Correction within REQ-13/17/18: the selected smile must retain its source-calibrated lip aperture and dental coverage outside laughter, including an already-settled positive mood. During internal laughter gaps, the closed-jaw oral shape must follow the existing source-supported smile rather than compressing the open laughing lips into the neutral aperture; full amused cheeks/lids remain independent. Jaw opening must visibly articulate the lower face without translating the entire neck or clothing; cached MediaPipe anatomy must bound its lower-face and short under-chin transition. Validate these properties directly with fixed-head rest/open-A renders, settled manual/Playful smiles and production playback. CPU/CUDA agreement and endpoint continuity alone do not qualify appearance. Preserve rejected phase109/snapshot769 and original audio/materials; make no provider or model calls. The owner's report authorizes the regression repair and its derived checks.

## Restored-base conversational continuity (2026-10-28)

The owner says the restored laughter is going well, reports occasional speaking jolts and static idle, and requests a subtle viseme-driven jaw without whole-neck movement. This authorizes the following focused changes and their checks; phase107 is the runtime/source baseline, and rejected phase108-110 laughter must not be reintroduced.

- REQ-19: Background attention suppression releases continuously across phrase and speech boundaries, without changing gesture trajectories, speech speed or the phase106 A/V clock. Reproduce the current2.4-degree single-frame handoff before repair. If another jolt is not reproduced, do not claim that every possible speaking jolt is solved.
- REQ-20: A separate optional body-only speech-jaw companion uses cached source anatomy and existing viseme interpolation. Start at0.75 percent face-height maximum excursion, allowing subtle motion rather than the superseded3px minimum in REQ-13. Lower neck/clothing remain stationary; mouth geometry, teeth, contact timing and the accepted restored laughter path stay unchanged. The channel is zero during rest and all laughter/recovery. Do not activate the rejected gross/residual oral-mandible channel or laughter companion.
- REQ-21: Idle holds shorten to1.6-2.6 seconds within the existing head/gaze bounds, movement speeds, anti-sweep policy and accessibility/pause behavior. Avoid artificial oscillation or larger motion to compensate for long holds. Preserve the phase107 speaking schedule except for the continuity repair. Speaking should retain stronger angular activity; a moving-frame count is not an activity target and need not exceed idle after shorter idle holds.

Use no provider/model/image/analysis requests, new dependencies or public controls. Build a review candidate from the frozen phase107 source with only these scoped patches; preserve exact phase107 rollback and archive source/input/output evidence in PostgreSQL. Matched fixed-head laughter states must remain pixel-identical, and real cached speech/player inspection must accompany geometry/continuity checks. Owner naturalness acceptance remains separate.

## Continuous lower-face jaw influence (2026-10-28)

- REQ-22: Repair the reported stationary middle band between moving chin/lateral jaw regions. Preserve phase112 head continuity, idle, viseme timing and body-only channel. Protect the actual cached outer-lip/enamel/contact support with a versioned source-bound domain, not the entire oral edit rectangle. Skin below/beside the lips inside that old rectangle may deform; its former whole-rectangle pixel-identity gate is explicitly superseded. Require a connected, smooth lower-face displacement transition outside the necessary lip sampling guard, stable lower neck/clothing, unchanged actual lip/teeth pixels and matched restored laughter. Keep the same0.75-percent maximum and existing geometry gates. Use only cached source calibration/geometry; no hand-drawn character masks, provider/model/image/analysis requests or new controller.

The owner's report authorizes this focused correction and derived checks; do not change the successful phase112 head/idle work or redesign laughter.

## Approved natural conversational presence (phase118, 2026-10-29)

The owner approved the complete natural-conversation plan and implementation, including local installation, scoped commit and push.

- REQ-31: Speaking posture prefers acoustic pauses or unoccupied emphasis, with bounded 2–3 second background deadlines (4–5 seconds for quiet delivery), 650–750 ms transitions and per-adjustment bounds of 1° yaw/0.5° pitch/0.15° roll. Idle uses distinct 550–900 ms adjustments and 1–1.7 second holds. Missing acoustic opportunities must not leave the character indefinitely still; all movement stays within prepared geometry.
- REQ-32: Semantic gestures compose around the held posture without automatic recentering or overlapping background actions; approved energy affects future action amplitudes, not the displayed pose. Existing gesture speeds/manual previews, torso breathing, source geometry/face/oral/jaw/laughter remain preserved.
- REQ-33: One per-session coordinator preserves motion across phrases/prefix replacements, consumes opportunities once, ignores incomplete endings, freezes speech-linked movement during underflow and settles interruption continuously. The optional internal conversational_motion.presence_version=conversational-presence/3 selects this behavior; missing selector preserves legacy motion. No audio buffering or public interface change.
- REQ-34: Matched cached baseline/candidate playback, focused continuity/bounds/race tests, bounded cadence/audio checks, local rollback and byte-verified PostgreSQL evidence establish technical verification. Owner naturalness remains separate. No paid calls, new assets/models/dependencies or public deployment.

Owner correction during phase118 implementation (2026-10-29): the character is too still both idle and speaking. This supersedes REQ-31's indefinite no-opportunity hold. Prefer acoustic opportunities, with bounded background adjustment deadlines when speech has none; use distinct small idle adjustments with shorter genuine holds. Preserve posture-relative gestures and avoid regular opposite-side swaying. Verify visible activity in both conditions without treating movement counts as owner acceptance.
