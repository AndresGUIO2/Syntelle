# Implementation plan

**Status:** approved · **Date:** 2026-10-27

The owner's approved plan is the complete `Coordinate Serena's Voice, Intention and Gestures` request in this conversation. Implementation uses the current worker as the sole policy authority and keeps one unactivated phrase; duplicating policy in the API would permit divergent voice/face decisions.

1. Extend the shared private contracts and current director with v4 purpose, approved delivery identity and reservation messages; preserve legacy wire behavior. (REQ-1, REQ-2, REQ-4)
2. Split native policy evaluation from activation, implement private reservation/commit and staged settings on the session send owner; route acknowledgements through existing streaming/conversation channels. (REQ-2, REQ-3)
3. Add typed directed synthesis, exact representation/cache bindings, Serena runtime defaults and regional/capability checks. Preserve old speech bindings and voice-specific laughter lookup. (REQ-1, REQ-4, REQ-7)
4. Change the phrase pipeline to await activation before preparing the next phrase; coalesce settings-driven replacement of unplayed speech with new durable identities. (REQ-2, REQ-3)
5. Extend existing attention/expression composition, final-head gaze and private lifecycle hints; retain motion/contact/laughter bounds and legacy behavior. (REQ-5, REQ-6)
6. Run the independent bounded voice experiment, comparisons and isolated checks; install a local candidate and archive complete evidence with restoration. (REQ-7, REQ-8)

Primary file groups: `packages/render-contract`, API conversation/speech/streaming modules, native animation/media modules, existing Vue streaming settings/composable modules. New processor commands belong under `app.workflows`; archives use `app.preparation.archive`.

Risks: first-audio/interphrase latency from reduced lookahead; stale settings during first PCM; undocumented Serena style behavior; mismatched laughter voice. Detect these with explicit activation-race tests, recorded latency and exact voice/language comparisons. Stop paid experiments on the first provider error; never retry or change serializers automatically.

Verification uses focused Go/race tests in cached containers, Vue type/lint checks, cached browser fixtures, exact-text normal-speed recordings, one WebKit audio check, one isolated 30-second player comparison and PostgreSQL restoration. No broad geometry/browser matrix is required.

Owner A/V + Emma correction: RemotePlayer owns one native synchronized playback element and its controls; useBackgroundVideo draws decoded frames to an inline canvas. Only picture-in-picture requests create a captured video stream. No audio delay, playback-rate change or articulation retiming is permitted. Emma configuration/capabilities/cache bindings are separate from historical voices. Focused browser checks cover synchronized marker presentation, controls and PiP.


## Phase 107: Emma laughter and conversational presence

REQ-10 uses a separate bounded private nonverbal probe and canonical speech preparation workflow; profile publication admits each accepted voice/language independently and preserves complete rejected outputs. REQ-11 extends the existing posture, semantic attention and expression directors with short varied diagonal adjustments and quieter reassurance/low-energy holds, retaining current bounds, admission and worker emotion authority. REQ-9 retains the phase-106 playback component and exact mouth sampler. REQ-8 uses phase-106 runtime/profile rollback and matched cached recordings with verified PostgreSQL restoration. No new models, image work, public interfaces or deployment.


## Phase 108: articulated jaw and audio-matched laughter

REQ-12/14 extend immutable native preparation with retained events, separate pulse/facial trajectories and bounded residual lip channels. REQ-13 adds an optional original/landmark/depth-bound jaw companion, gross/residual oral attachment and shared pre-head geometric articulation for CPU/CUDA. Existing images/audio, clocks, contact protection and legacy fallback remain intact. REQ-8/9 preserve phase-107 rollback and archive exact preparation, source, rejected diagnostics and cached comparisons; capacity contention does not qualify real-time performance.

## Complete laughter repair, phase109

First add failing regressions and private PCM span binding shared by API/manual preview/worker. Then add a single face lifecycle independent of jaw pulses and keep emotion policy active through visual tails. Prepare a cached-source jaw-neutral reference companion with deterministic mandibular-oval projection and shared-coordinate baseline composition; retain canonical opaque dental/iris sources and existing qualified rig bounds. Replace preview timer with existing playback lifecycle, run full production-path/rendered regressions, compare cached real-time playback, install and archive/restoration-verify a local candidate. Owner approved direct return to the current selected expression; no provider or new model request is authorized.

## Phase110 regression repair

Reproduce the reported settled smile and whole-neck jaw motion before changes. Constrain mandibular support from cached face anatomy and rebuild its bound laughter companion without changing original textures/audio. Correct any oral/smile composition regression at its owning stage rather than weakening expression amplitude. Check settled baseline, fixed-head jaw, laughter recovery and actual installed playback; retain rejected phase109 and byte-restore new evidence. (REQ-8/9/13/17/18)

## Phase112: preserve restored laughter and improve conversational motion

Use exact frozen phase107 runtime source, package and Emma profile as the baseline. Apply only three patches: retain the existing semantic-focus release envelope across phrase handoffs; shorten idle holds without wider/faster targets; add an optional source-bound body-only speech jaw separate from all oral/laughter channels. The jaw deforms only protected lower-face geometry, is driven by existing viseme interpolation, and is zero during laughter/recovery/rest. Keep canonical oral/teeth/eye sources and A/V playback untouched. Check failing-before/passing-after head continuity, bounded idle activity, subtle fixed-head jaw/neck isolation and byte-identical matched laughter renders; then cached player review, local candidate, exact107 rollback and PostgreSQL restoration. No changes to rejected108-110 resources or their frozen evidence. (REQ-8/9/19/20/21)

## Phase113: remove the stationary skin island

Start from the frozen phase112 source closure and preserve its head/idle/audio/laughter behavior. Reproduce the two full-oral-crop clamps that freeze excess below-lip skin. Replace them with optional speech-jaw/2 protection derived from actual cached lip/support contours and a smooth transition outside its sampling/triangle guard. Version the validation precisely rather than weakening rectangle checks globally. Test central/lateral displacement continuity, actual lip/enamel pixel identity (not surrounding skin), fixed lower neck/clothing and original laughter identity; compare cached head-fixed speech before/after and retain112 rollback. Archive complete preparation, rejected results, source closure and review with PostgreSQL restoration. (REQ-8/9/20/22)

## Phase118 approved approach

Use the frozen phase117 runtime closure and a separate package metadata selector; preserve unrelated working-tree work. A video-loop-owned presence coordinator consumes existing immutable acoustic/semantic analysis and uses emitted PCM progress. Hold the actual background pose during gestures, prefit amplitudes, and retain legacy behavior without the selector. Propagate streaming/final/preview state internally without altering samples, visemes, buffers or public messages. Extend focused tests and compare identical cached recordings before installing under stable local names. Archive complete new evidence and restore hashes; finish with a scoped commit and push. (REQ-31–34)

The owner's too-still correction selects normal speaking deadlines2–3s, quiet4–5s, speaking travel650–750ms, and idle travel550–900ms with1–1.7s holds. Existing acoustic opportunities may anticipate the deadline within the minimum spacing. Target differences remain bounded and distinct; this replaces indefinite waiting for cues. (REQ-31)
