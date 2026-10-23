# Documentation

[Cloud deployment preparation](development/cloud-deployment.md) covers Terraform, Modal, the app.syntelle.online proxy, offline verification, private inputs and rollout gates.

[Independent API and renderer modules](development/renderer-service-boundary.md) describes separate source, dependency and image boundaries and unchanged contracts.

Current application entry: [backend-only player](development/backend-only-player.md). Both `/` and `/?view=stream` display remote video with the latest microphone-only UI; former public client-rendering routes and export modes are retired. Historical experiment descriptions below remain evidence.

The private [render worker](../services/render-worker/README.md) is native Go only. Its standard build/start commands do not use Chromium; browser dependencies remain for player/receiver tests and deterministic fixture preparation. The unused browser worker and private web entry are retired, with original comparisons preserved in the evidence archive.

Start with [repository structure](development/repository-structure.md) for code locations and command conventions.

[NVENC and automatic OpenH264 fallback](development/nvenc-fallback.md) documents native hardware encoding, Docker detection, real RTX 3090 checks and remaining cadence/memory limitations.

[Native GPU rendering and WSL2](development/native-gpu-rendering.md) records CUDA/CPU selection, preserved Docker data, byte-exact frames, bounded CPU/memory comparisons and the remaining Windows player shortfall.

[GPU-resident media and Windows cadence](development/gpu-resident-media.md) supersedes that shortfall: GPU mouth/eyes/skin, direct NVENC surfaces, raw-clock scheduling and matched CPU/memory/player evidence.

[Shared immutable GPU assets](development/shared-gpu-assets.md) records per-content device sharing between independent sessions, reference-counted cleanup and bounded memory/player comparisons.

## Current local demonstration

- [Repeated nods](experiments/motion/repeated-nods.md): one nod with intensity and repetition controls, a validated wider pitch range and shared conversational movement limits.

- [Explicit Start and demo time](development/stream-demo-limits.md): a static preview with the selected background before interaction, production five-minute sessions/15 accumulated minutes, and the explicit unmetered local-development exception.

- [Streaming admission controls](development/stream-admission.md): request/connection throttling by identity and trusted client IP, atomic per-visitor session admission, and current single-process/public-deployment boundaries.

- [Streaming message inactivity](development/stream-inactivity.md): 30 seconds before the first message, two minutes after user input, server-side resource release and explicit resume with preserved conversation history.

- [Pricing page](development/pricing-page.md): standalone `/pricing` navigation, direct access and browser history, replacing the rejected modal.

- [Platform language](development/platform-language.md): one header-level English/Spanish selection for the interface, account, settings, status messages and Syntelle conversation.

- [Accounts and rendered streaming candidate](development/accounts-and-streaming.md): implemented Syntelle-only local candidate at `/?view=stream`, with a private RGB-D render worker, optional Google/GitHub/email-code login, saved setups and integration keys. The previous default remains at `/`; live identity/email delivery needs provider configuration and a verified sender. Verification uses fixtures and cached speech; owner review and public deployment remain pending.

- [Workspace redesign](development/workspace-redesign.md): layered graphite/violet surfaces, responsive portrait framing and a local microphone RMS indicator.

- [Streamed text and early voice](development/streaming-voice-replies.md): partial text, durable opening audio before full model completion, complete phrase queues and confirmed microphone interruption.

- [Conversational speaking attention](experiments/motion/speaking-attention.md): brief gaze breaks and soft posture during speech, preserving cached assets and the preceding fixed-gaze comparison.

- [Contextual performance direction](experiments/motion/contextual-performance.md): structured Gemini direction, steady speaking posture, explicit smiles, bounded head gestures and the visually rejected facial laughter companion.
- [Expressive speech and phrase direction](experiments/motion/expressive-performance.md): streamed numeric LLM cues, smiling speech, reviewed cached laughter and optional wider head comparisons in the native server renderer.

- [Microphone-only conversation](development/voice-conversation.md): current voice controls, transcript, interruption and bounded browser verification.
- [Recognition turn grouping](development/recognition-turns.md): combine nearby SDK speech fragments into one conversation message, with cancellable settling and preserved original evidence.
- [Conversation workspace refresh](development/conversation-workspace.md): aligned panels and mobile layout, including the historical typing interface.

- [Protected local demo builds](development/protected-demo.md): separate baseline/protected builds, runtime-only manifests, conservative engine obfuscation and offline review without geometry streaming.

- [Local owner Chat and Read aloud](development/local-conversations.md): Gemini 3.8 Flash chat, hands-free Azure recognition, conversation persistence, uncapped local-owner policy and remaining live verification.

- [Conversation workspace](development/user-demo.md): local Syntelle demo and customization, with preserved historical write-and-listen behavior. Technical trials remain available separately.
- [Coordinated diagonal posture and gaze](experiments/motion/coordinated-attention.md): current user-demo attention, viewer-focused eyes, automatic source-only preparation and the preserved previous idle.
- [Manual and speech-driven expressions](experiments/motion/facial-expressions.md): soft smiles and raised eyebrows with shared source-only preparation and local controls.
- [Longer visible idle](experiments/motion/idle-posture.md): shared user-demo posture timing with short irregular holds, using the existing validated rotation bounds.
- [Articulated neck and depth breathing](experiments/motion/neck-and-breathing.md): `head=neck-compare` candidate; previous RGB-D, articulated neck and depth breathing share one clock. Owner visual review pending; bounded verification only.
- [Depth-aware head parallax](experiments/motion/depth-aware-parallax.md): current optional `head=natural` candidate, calibrated perspective geometry and three synchronized comparison views.
- [Breathing and double blinks](experiments/motion/vitality-motion.md): preserved `head=vitality-candidate`, asymmetric torso breathing and shared-frequency double closures.
- [Speech-context movement and blinking](experiments/motion/contextual-motion.md): preserved `head=contextual-candidate`, small speech-driven nods and independent blink planning.
- [Motion, eyes and backgrounds](experiments/motion/motion-trial.md): prepared resources and browser controls.
- [Automatic oral corpus](experiments/oral/automatic-oral-corpus.md): Syntelle, cafe and woodland mouth packages and remaining validation.
- [Preparation database](preparation/preparation-database.md): PostgreSQL evidence, budgets and restoration.

## Product and design

- [Optional login and saved characters](../specs/002-optional-login/spec.md) and [rendered character streaming API](../specs/003-character-streaming-api/spec.md): October 21 scope records with local implementation updates. The Syntelle-only candidate extends the original MVP; neither record establishes a deployed public service or live-provider acceptance.

- [Product scope](product/product.md), [brand](product/brand.md), [character preparation](product/character-preparation.md) and [character reference](product/character-reference.md).
- [Design reference](design/design-reference.md). Original HTML sources remain in `design-references/`.
- [MVP specification](../specs/001-syntelle-demo/spec.md), [technical plan](../specs/001-syntelle-demo/plan.md) and [tasks](../specs/001-syntelle-demo/tasks.md).

## Development and experiment records

- [Browser background composition](development/frontend-background.md): frontend backgrounds and blur with private backend character rendering, synchronized packed alpha, sound and PiP.

- [Local development commands](development/local-dev.md): one-command background startup, readiness, stop/status, selective cleanup and bounded Docker cache/log retention.

- [Go service organization and reliability](development/go-service.md): command/internal packages, burst handling, speech access protection and nonfatal interruption-persistence warnings.
- [Development skills](development/development-skills.md): project-local Vue/FastAPI/UI guides and six pinned Go skills, usage triggers, project overrides and integrity records.

- `development/`: first build, code organization, skills and the repository map.
- `experiments/oral/`: mouth, dental material, contours and corpus experiments.
- `experiments/motion/`: eyes, foreground extraction, depth, movement and speech experiments.
- [Registered smile](experiments/motion/registered-smile.md): Syntelle's reference, reusable preparation and current limits.
- [Speech articulation pacing](experiments/motion/viseme-pacing.md): suppress fleeting mouth targets and soften transitions on the original audio clock.
- `characters/`: per-character configurations, corpus manifests and recorded settings. These paths remain stable.
- `prompts/`: original source prompts. Their contents and locations remain unchanged.

Historical records describe the implementation at their recorded stage; the latest sections in the project instructions and current experiment documents take precedence. Immutable images, models and complete evidence remain in the ignored `.tools/` working tree and PostgreSQL archive. They are not application source.

- [Local streaming efficiency](development/streaming-efficiency.md): full pinned Chromium with available GPU rendering, shared draw/capture scheduling, H.264 negotiation, bounded CPU/data/quality measurements and remaining device limits.

- [Express CPU-only streaming](development/express-cpu-streaming.md): bounded Linux software-rendering optimization, exact visible-pixel checks, rejected two-session/two-vCPU profiles, and a public compute/egress cost reference.

- [CPU rendering diagnosis](development/cpu-rendering-diagnosis.md): software graphics process attribution and a deliberately frozen diagnostic control; the fifty-session objective remains unvalidated.

- [Native CPU rendering](development/native-rendering.md): specialized Go character rendering, deterministic animation parity, native H.264/Opus WebRTC, and historical quality/resource comparisons against the retired Chromium worker.
- [Native worker efficiency](development/native-efficiency.md): bounded speech preparation, fewer encoded-payload allocations, exact pixel preservation and measured native-before/after CPU and memory.
- [ISPC and libyuv trial](development/cpu-simd-trial.md): optional native kernels, exact frame checks, failed conversion compatibility, bounded Linux CPU/memory comparisons and the decision to retain the current default.
- [libyuv color validation](development/libyuv-color-validation.md): explicit BT.601 tolerance with exact scalar reference preserved, sustained CPU savings, two-decoder fixed-frame checks and remaining quality/memory gates.
