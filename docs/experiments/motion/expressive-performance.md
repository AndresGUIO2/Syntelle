# Expressive speech and phrase direction

October 22, 2026. Implemented and internally reviewed in the local server renderer. Owner visual acceptance, physical-device checks and arbitrary-upload readiness remain separate.

## Current behavior

The native renderer uses `.tools/expressive-performance-67/runtime-package/`, with a private `character-performance/1` profile. The browser retains controls, audio/video playback and background composition. It receives neither expression resources nor LLM control syntax. The development player uses the same server path; historical client laboratories and earlier assets remain evidence.

Gemini's active director requests `performance-intent/2`, preserving the character task instructions and complete reply while explicitly requesting varied, meaningful expression. Its streamed envelope contains `version`, `tone` and complete `phrases`, each with spoken `text` and separate `cues`. A cue supplies a supported `kind`, unique exact `anchor` within the phrase and numeric `amount` in 0Ã¢â‚¬â€œ1. Prepared actions are `nod`, `double_nod`, `shake`, `emphasis`, `tilt`, `brows`, `smile` and `laugh`. Each complete phrase and its optional direction are validated before corresponding audio delivery. Malformed optional cues are omitted without discarding valid spoken text. Commands never enter TTS or transcripts. Archived `performance-intent/1` delivery remains supported.

The director receives prepared capabilities, voice/language availability and current manual smile/brow baseline. Application code owns timing, frequency and geometry. Two bounded synthesis workers prepare phrases concurrently and deliver them in order. Phrase request keys are distinct while retaining the original turn foreign key. Delivery evidence is committed before playback; cancellation joins producers and saves interrupted state.

The extension keeps 65% of idle posture during speech, with irregular 2.5Ã¢â‚¬â€œ4.5-second transitions and 0.6Ã¢â‚¬â€œ1.6-second holds. Brow emphasis is 50% and head emphasis is 0.9 degrees pitch within prepared limits. Double nods have two pulses; tilts use brief roll. One allowance admits at most eight head gestures per reply, separated by 2.2 seconds without overlap, including acoustic beats. Actual PCM energy and pauses place approximate phrase-span gestures. Sparse replies gain non-semantic emphasis at irregular 4Ã¢â‚¬â€œ7-second opportunities on the continuous reply audio clock, including across short phrases. Audio does not invent agreement/disagreement. Packages without the joint extension keep their earlier posture/expression behavior.

## Smiling speech and laughter

The slider is the smile baseline. At its maximum, speech targets half the prepared smile range. Numeric LLM targets temporarily intensify or soften the baseline, including explicit zero, then ease back to the latest slider value. Interruption releases expression smoothly. Automatic-expression and reduced-motion controls remain effective.

`joint-expression-articulation/1` prepares lips and cheeks together with existing articulation geometry. Immutable originals, cached MediaPipe landmarks, oral/motion packages and registered references determine regions and limits. MBP closure and central FV contact are preserved. Safe field amplitudes and lip-ring triangulations are selected automatically across articulation, smile and laughter mixtures. There are no Syntelle-specific coordinates, manual masks or placement overrides.

References contribute registered soft-tissue appearance and shape. The complete oral patch and padded transition exclude generated lips, teeth, tongue and cavity from that material. Original lip surfaces and existing canonical dental textures remain responsible for the mouth. Canonical teeth stay opaque, unchanged in color and geometry, with rigid jaw translation and coordinated aperture/occlusion. No competing lip seams or tooth-opacity dissolve is introduced. Soft-tissue frames are prepared once and reused; runtime makes no preparation/image request.

Laughter appends accepted cached Ava PCM at a phrase boundary with an 80 ms bridge. Original PCM remains exact, native visemes are shifted and derived audio records both hashes. Cheeks, eyelid narrowing, jaw articulation and opening follow the audio clock. At most one laugh is allowed per reply, at least 60 seconds apart and never in consecutive replies. Unsupported voice/language combinations omit laughter while preserving speech. The current bank supports Ava in `en-US` and `es-CO`, without voice substitution. The prompt requests laughter only for suitable playful content or explicit demonstration.

Rejected phase-34 laughter compositions and all intermediate candidates here remain preserved. The bounded new result passed internal composition and actual-player review before local activation; owner approval of its appearance remains pending.

## Reusable preparation and budget

`docs/characters/expressive-performance.json` binds the fresh PostgreSQL allowance `authorized-expressive-performance-2026-10-22`: four images, zero analyses. Exactly two image requests succeeded, one broad-smile and one laughing reference. No corrections, retries, paid analyses or paid speech requests were made. Complete requests/inputs were committed before paid calls. Two reviewed corrections remain available only within that original scope; none is needed to replay the result. Existing budgets were not reset.

Run from `services/processor`, with a new output directory when preparing. Do not overwrite frozen evidence or regenerate cached references.

```powershell
.venv/Scripts/python.exe -m app.workflows.motion.prepare_joint_expressions --manifest ../../docs/characters/joint-expression-inputs.json --output <new-private-directory>
.venv/Scripts/python.exe -m app.workflows.rendering.export_native --expressions <reviewed-expression-directory> --output <new-native-package>
.venv/Scripts/python.exe -m app.workflows.motion.prepare_performance_profile --package <native-package> --review <package-bound-review.json> --laughter <approved-cached-recordings.json> --output <new-profile.json>
```

Profile preparation requires composition/playback review bound to the exported package hash. Available actions derive from prepared resources; approved cached recordings bind by hash, voice and language. Owner acceptance is separate. These commands perform no provider calls and do not expose an upload endpoint.

The private active profile is `.tools/expressive-performance-67/active-profile/profile.json`. Local `.env` selects `SYNTELLE_RENDER_ASSETS`, `SYNTELLE_PACKAGE_HASH`, `SYNTELLE_PERFORMANCE_PROFILE_DIR` and the container `SYNTELLE_PERFORMANCE_PROFILE` path. Compose mounts only that profile directory read-only. Credentials remain outside evidence. The native launcher and `dev start` reuse configured assets. Installed image: `syntelle-render-native:expressive-67-ready`, also selected by the stable local tag. Previous image identities, resource volume and non-secret selections are preserved.

## Wider movement

`app.workflows.motion.prepare_expression_envelopes` evaluates 1.25x and 1.5x existing angular bounds using cached RGB-D inputs. All six three-character candidates pass 81 geometry configurations each. Syntelle also passes internal fixed-frame inspection at both signs of the wider yaw/pitch/roll combinations. Private `wider-125/` and `wider-150/` exports are optional review candidates. Exporter/loader reject wider selection without geometry validation and `internal-visual-pass`. Cafe/woodland remain geometry-only evidence, without exposed wider packages.

The default remains improved bounded 1x. Earlier clamped comparisons are preserved separately from corrected `envelope-review-unclamped/`. Wider bounds do not establish hidden-surface reconstruction or owner acceptance.

## Verification

- Ten processor tests pass: reference reuse/error bounds, profile review binding, three cached-character identity preparations, three nonzero synthetic fields, combined topology, MBP closure and FV contact.
- Focused Go checks pass for conversation, speech, streaming, native animation/rendering/adapter and API. Isolated PostgreSQL tests prove first-phrase delivery before model completion and cancellation with durable interrupted state. Fixtures cover numeric targets, baseline restoration, shared spacing, acoustic opportunities across phrases, double nod/tilt, pause, automatic controls and laughter policy.
- The prepared package passes neutral equivalence at all nine articulation endpoints, protected-pixel/alpha and immutable dental/source-material checks at multiple expression levels. All 72 directed pairs pass combined topology checks over 125 sampled configurations per pair.
- Nineteen focused states pass CPU/CUDA comparison at 720 and packed 1440 widths: 38 comparisons, maximum byte difference zero and exact CPU fallback. Concurrent GPU/media changes are preserved. This work claims no CPU/memory or sustained cadence improvement.
- Actual isolated API/PostgreSQL/native CUDA/NVENC/Chromium playback passes with cached PCM: smiling speech, boundary laughter, next phrase, audible energy and interruption. Text/audio are fixtures, establishing synchronization/control rather than a new pronunciation assessment. Normal-size captures passed internal inspection. A second reply verifies interruption without repeated laughter.
- Frontend lint, typecheck and production build pass. Four bounded Chromium/WebKit desktop checks cover controls/transcripts/background isolation. Windows WebKit is not counted as actual audio playback. Eight launcher checks pass.
- Installed main-player idle/background/PiP check passes with zero paid calls, 720x960 presentation, no dropped frames and approximately 28.6 presented fps in this brief run. This is not strict sustained-performance or real-device acceptance.

Full references, rejected/intermediate preparations, source snapshots, manifests, frames, playback reports and restoration records are under `.tools/expressive-performance-67/`. `archive_checkpoint.py` uses the generic PostgreSQL `Archive` interface and verifies every restored file by SHA-256. The external `.tools/expressive-performance-67-archive.json` records the final snapshot. Do not run historical finalizers against this work. No public upload endpoint, cloud deployment, new model or arbitrary-character readiness claim is included.


## Contextual expression and direct visual previews (phase 68)

The owner clarified that expression must not be a mechanism for obeying a user's gesture commands through chat. The active phrase prompt now chooses performance from conversational meaning and character personality, independently of requests such as a laugh command or an eyebrow percentage. The earlier recommendation to ask the LLM for a demonstration is superseded.

`Customize > Preview expressions` provides direct visual Smile, Laugh, Eyebrows, Nod, Double nod, Shake and Tilt previews. These use an enum-only authenticated `preview` command on the existing stream, never the conversation channel. They create no transcript, model context, TTS request or audio. Laughter here is a visual preview; normal contextual laughter still reuses the accepted recording during real conversation.

Previews last two seconds on the pause-aware native animation clock, use prepared capabilities and bounds, and return to the current controls. They do not change saved sliders or automatic-expression preferences. They are unavailable during a reply, disconnected playback or reduced motion, and a new conversation/interruption clears them. The existing automatic gesture policy is otherwise preserved.

Evidence is `.tools/expression-preview-68/`. Focused Go tests cover native sampling, capability restrictions, restoration and speech isolation. Two Chromium/WebKit UI checks pass, as does actual native CUDA/NVENC playback with automatic expressions switched off. Live Smile/Laugh/Eyebrows/Nod previews create only preview commands. PostgreSQL conversation/provider operation counts remain unchanged: zero model, speech or image calls. Preserve the initial blocked-module/browser and test-type failures alongside the fixes. Public module exposure adds only the small control component, with no character resources. Sources, frames and reports use a separate generic PostgreSQL archive with verified restoration. The local worker is `syntelle-render-native:preview-68`; phase-67 assets/profile and the previous worker image remain intact.


## Saved audio in manual laughter preview (phase 69)

The Laugh preview now plays the accepted cached recording as well as its prepared expression. `conversation.Service.LaughterPreview` reads the reviewed voice/language bank without a model, TTS provider, conversation store or conversational laughter allowance. Original WAV bytes and native visemes remain unchanged. Unsupported voice/language combinations fail without voice substitution or generation; the existing bank supports Ava in en-US/es-CO.

The existing authenticated preview command privately delivers that recording to the native audio/preparation lane. A preview-only clip carries the reviewed laughter cue; cheeks, eyelids and jaw follow actual emitted PCM position. Explicit preview activates its own expression even if automatic conversation expressions are off. Other previews remain visual-only. Playback status enables the browser's existing audio path after the user's click. No transcript/context message or raw browser audio payload is added. Interruption clears the preview queue, and manual playback can be repeated without consuming the conversation's 60-second/consecutive-reply allowance.

Current local worker: `syntelle-render-native:laughter-preview-69`. Source/assets/profile and earlier images remain preserved. Evidence is `.tools/laughter-preview-69/`: focused Go tests, frontend build, four Chromium/WebKit UI checks, actual CUDA/NVENC playback with two completed previews and measurable received PCM energy. Before/after PostgreSQL conversation/provider operation counts are identical. No paid calls, schema change, new generation, new voice or cloud deployment is included. Archive/restore uses the separate generic PostgreSQL record `.tools/laughter-preview-69-archive.json`.
