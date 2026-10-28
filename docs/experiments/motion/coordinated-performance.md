# Coordinated Serena performance

Date: 2026-10-27. Phase 103 implements the approved coordination design. The local review candidate is installed with Serena plain delivery. English directed styles and both laughter companions are enabled for technical review; Spanish explicit styles and owner naturalness review remain pending.

## Direction and phrase lifecycle

The existing director emits `performance-intent/4`: clean spoken text, emotion/energy, exact anchors and one separate purpose (explain, reassure, ask, affirm, disagree or playful). It uses the existing model request. Quiet phrases remain valid. The instructions match the eight-cycle rolling 20-second allowance, 2.2-second spacing, non-overlap and existing nod/shake speeds. Legacy protocols remain accepted.

Private preflight runs on the authenticated worker connection before TTS. The native emotion controller evaluates a copy without changing visible or played history. One pending reservation binds worker instance/generation, session ownership, turn, phrase, requested revision, text SHA-256, voice and language. The deadline is two seconds; its 90-second lease uses the pause-aware presentation clock. Old workers receive no unknown activity commands; unsupported preflight fails before synthesis.

First PCM publication and state commit precede activation acknowledgement. Only then can N+1 prepare over N's remaining playback. Failed sends, stale identities, cancellation and expiry cannot activate a direction. Activation starts from the current visible expression, including ongoing recovery. Durable activation receipts identify actually played text; queued/superseded text is excluded from echo context.

Requested and applied emotions are separate. Manual changes finish the audible phrase and invalidate only unplayed preparation. Replacement keeps text/index but receives a new settings revision linked to the superseded token. A→B→A never revives old preparation. Existing debounce coalesces controls. Settings/direction commit atomically at first PCM; staged settings recover at reply end if no phrase remains. Pause freezes presentation; background and reduced-motion controls remain immediate. The UI explains next-phrase application.

## Voice and existing motion

The source default is `en-US-Serena:DragonHDLatestNeural`, selected through the profile's optional `voice` field. Registry, server coercion, saved settings and capability reporting agree. Historical Ava/Andrew cache identities remain unchanged. The candidate is selected through an explicit local Compose override, preserving the preceding image/profile.

Private typed synthesis maps the approved purpose/emotion to reassuring, curious or amused only when the exact voice/language capability enables it. Missing/incompatible styles choose contextual plain text before the call. Application values serialize escaped SSML separately from the clean transcript. Directed bindings include delivery, capability/serializer revision and exact provider representation. A failed directed request stops without fallback synthesis, alternate serialization or voice substitution. Public speech input exposes no provider-markup fields.

Word events require valid native timing and clean UTF-16 text mapping. Otherwise span/acoustic timing is explicitly approximate: searches stay near the anchor and uncertain word accents are omitted. No alignment model was added.

Laughter lookup remains exact `voice|language`. Serena now has original 800 ms English and 950 ms Spanish companions. Native Rest events are valid for nonverbal laughter, whose prepared facial path uses audio energy. The initial English output failed the ordinary spoken-phoneme validator; that immutable failure and its original bytes were preserved and re-evaluated offline with a separate nonverbal validator. No generation/request was repeated, no ordinary speech gate changed, and neither sample was trimmed. Ava remains historical comparison material only.

Semantic cues smoothly reduce competing posture and spontaneous speaking glances. Reassurance eases posture gain to 0.6 and glance gain to 0.15 while preserving intentional cues and sustained mood. Other purpose choices are optional director instructions, never mandatory gestures or habitual smiles. Head/brow emphasis shares the admitted audio anchor, approximately 150 ms anticipation and 320 ms release. The brow accent composes after emotional attenuation and carries its release across boundaries. Gaze uses final head composition. Listening/preparing signals use existing states, without automatic agreement nods or artificial delay. Articulation/contact, qualified bounds and the phase-102 final laughter bridge are preserved.

Microsoft lists Serena in its [voice catalog](https://learn.microsoft.com/en-us/azure/ai-services/speech-service/language-support?tabs=text-to-speech). The [HD documentation](https://learn.microsoft.com/en-us/azure/ai-services/speech-service/high-definition-voices) describes English expressive tags and distinguishes DragonHD from Omni. Actual Serena outputs are still required to qualify styles and languages.

## Bounded qualification

Configuration: `docs/characters/coordinated-serena-experiment.json`. PostgreSQL identity: `authorized-coordinated-serena-2026-10-27`. Limits: 14 synthesis attempts, 4,000 submitted UTF-16 provider characters, zero images/analyses, no retries and stop on provider error. Historical experiments and ordinary owner-conversation limits are unchanged.

Run from `services/processor`:

```powershell
.venv/Scripts/python.exe -m app.workflows.speech.qualify_serena --catalog
.venv/Scripts/python.exe -m app.workflows.speech.qualify_serena --slot reassure-en-plain
.venv/Scripts/python.exe -m app.workflows.speech.qualify_serena --prepare
```

Regional availability must pass before any slot. Fixed slots cover three purposes × English/Spanish × plain/directed plus two laughter companions. Spanish explicit styles are skipped before calls. Requests/catalog/bindings commit before the native SDK call; only exact received slots are reusable. The exporter checks original PCM/events and voice/language, refuses incomplete controls, and exports without activation. Audible language/style review remains required.

Default host/Docker DNS initially failed (snapshots 712/713). The installed API's existing resolvers verified Serena in Canada Central (snapshot 714), without changing host DNS. **Actual usage: 11 synthesis attempts, 1,131 submitted UTF-16 provider characters, zero provider failures/retries/images/analyses.** Six plain controls and three English directed controls returned valid PCM/native events. Spanish explicit styles were skipped before calls. Both laughter companions passed the nonverbal timeline and original duration gates. An initial application-validation failure was reconciled without changing its database status or re-requesting the audio; provider completion, not an application rejection, controls the stop-on-provider-error rule. Complete decisions and requests/results remain archived. No budget or historical allowance was reset. Audible delivery/naturalness acceptance remains an owner gate.

## Verification and delivery

Focused shared-contract/API/speech/conversation/streaming/native tests pass, including race checks. Isolated PostgreSQL coverage includes 34 complete phrases, one pending reservation, settings replacement, cancellation and activated-only echo history. Worker transport tests pass with a Docker network; the first network-disabled run lacked the expected interface and remains recorded. Vue type check, scoped lint and production build pass.

Controlled Chromium videos use identical cached Ava audio/text, framing and resources. Complete English/Spanish carcajadas were replayed at Playful 50% and 100%; phase-102 recovery tests remain active. These checks cover visual coordination/lifecycle, not Serena delivery or owner acceptance. Preliminary failures and corrected results remain in `.tools/coordinated-performance-103/`.

`comparison.json` records the isolated 30-second decoded-frame comparison without video recording, decoded Web Audio onset timing, approximate PCM-publication spacing and timestamped CPU/memory samples. Cached first-audio measurements exclude live model/TTS latency. Small negative gap estimates are scheduling/20 ms block uncertainty, not audible overlap. This does not establish sustained-device performance or competitive parity. See [verification](../../../specs/004-coordinated-performance/verify.md) for exact remaining gates, including WebKit.

Candidate Docker tags end in `:coordinated-performance-103` and are installed locally through `candidate.compose.yaml`. `.env`, geometry/image/material resources, public endpoints and cloud deployment remain unchanged. Previous images end in `:before-coordinated-performance-103`; `rollback.ps1` selects the old Compose images/profile and restarts the managed worker. `install.ps1` selects the review candidate. Installed Chromium verification confirms Serena and available language-bound laughter without provider calls. The matched Serena video compares the same text/facial direction with plain audio followed by reassuring audio; all framing/rendering settings are identical. Actual English-directed/Spanish-plain qualification remains separate from owner audible style acceptance. A Linux WebKit check plays actual cached Serena PCM through Web Audio; full WebRTC/video failed to establish media in the isolated fixture and remains unverified.

Complete sources, reused input bindings, decisions, audio/events, videos, binaries and failures are archived in PostgreSQL with hash-verified restoration. The receipt is `.tools/coordinated-performance-103-archive.json`. Future-character readiness and physical-device acceptance remain separate milestones.


## Phase 109: complete laughter and direct expression recovery

The current local candidate supersedes phase-108 laughter presentation. [Complete laughter recovery](complete-laughter-recovery.md) documents private PCM spans, complete bank actions, the jaw-neutral source companion, one presentation lifecycle, direct live-baseline return and playback-owned preview state. Both original Emma recordings and listening approvals remain unchanged. Sources, failures, real production-path replays, comparison recordings and rollback are retained in PostgreSQL; owner naturalness and future-character readiness remain separate.
# Owner-reported fear delivery mismatch (2026-10-28)

The owner reports that the chase role-play response neither sounded nor looked frightened, despite spoken text claiming fear. This is an unresolved performance defect; the phase113 jaw correction does not close it.

Read-only inspection of the existing conversation records confirms:

- The active Emma capability profile has empty style lists in English and Spanish. Speech preparation selects contextual plain text; approved emotion metadata alone does not serialize a fearful vocal delivery. The current directed style mapping covers reassuring, curious and amused only. This describes the application's configuration, not a claim that the provider can never express fear.
- The director assigned a playful tone, then proposed `fearful` at intensity 0.35 / energy 0.60 for the first phrase and `playful` at 0.30 / 0.55 for the following phrase. These choices are insufficiently coherent with the intended frightened performance.
- Automatic expressions were enabled. Worker decisions accepted the first fearful proposal and retained it for the next phrase under `minimum_duration`; both reservations activated. The stored decisions therefore do not support blaming disabled expressions or policy suppression for this case.
- The owner found the resulting face too weak. The screenshot and decision records do not establish which facial regions failed perceptually; inspect the existing cached performance before changing deformation or amplitudes.

Follow-up should address sustained communicative delivery, Emma-compatible vocal direction and the visible prepared fear expression together. Preserve the selected voice, jaw/lip synchronization, geometry bounds and restored laughter. Do not claim that stronger text, a keyword rule or emotion metadata alone provides emotional TTS. Qualifying a provider delivery requires its own explicit scope and actual-output review.

This investigation used existing records and source/configuration only, with zero synthesis, model, image or analysis requests and no runtime changes.

