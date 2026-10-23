# Conversation workspace refresh

The [microphone-only follow-up](voice-conversation.md) supersedes this screen's composer, writing suggestions and Read aloud tab. The transcript, character stage, theme and customization remain; current controls are Start talking, microphone shutdown and Stop reply.

The [complete conversation reply correction](full-conversation-replies.md) removes the AI response cap and shortened-audio notice while keeping complete transcript paragraphs and one interruptible audio clock. Its chat-input follow-up also removes the user-message 500-character cap and unexplained remaining-character counter.

For the latest speaking posture, contextual direction and rejected laughter companion, see [contextual performance direction](../experiments/motion/contextual-performance.md). This adds optional prepared-speech metadata and separate PostgreSQL direction evidence while preserving the transcript/composer layout and existing cancellation APIs.

Current simplification: the owner temporarily selected Syntelle only. The main workspace shows a static character identity rather than a character picker; Cafe and Woodland remain preserved in the catalog and technical laboratories. Language selectors have been removed from both Chat and Read aloud. The existing internal `en-US` default remains unchanged; this presentation change adds no automatic language detection. The earlier three-character/language-control verification below describes phase 32. Follow-up evidence belongs in `.tools/workspace-simplification-33/`.

The local demo now prioritizes conversation with a 5:7 portrait/chat layout, a 24 px desktop gap and aligned character/mode controls. Syntelle's existing DM Sans, graphite/violet themes and original Colors Studio assets remain authoritative. The supplied conversation HTML is a flexible reference; its original bytes are preserved in `.tools/conversation-ui-32/reference.html`.

## Behavior and component boundaries

- `DemoWorkspace` owns character selection, mode and the existing session connection. Its toolbar aligns with the two desktop panels; Chat remains the default. `useWorkspaceViewport` measures the visible mobile viewport and reserves composer space when an editable field opens the keyboard. It removes the compact stage/toolbar temporarily during keyboard occlusion and restores them afterward.
- `ChatPanel` presents one current status, the transcript, errors and the composer. `ConversationMessages` presents real history and a compact empty state. Three English/Spanish suggestions fill the draft and request focus without submitting, synthesizing audio or requesting microphone access.
- `ConversationComposer` owns its typed draft model, bounded textarea growth (64–132 px) and send/microphone/stop events. There is no 500-character message cap or remaining-character counter. Ctrl/Command+Enter sends; plain Enter retains multiline text. During playback, a nonempty draft exposes Send alongside Stop reply so typed interruption remains possible.
- User messages align right on a subtle surface; character replies align left. Long messages wrap. Only the transcript scrolls during ordinary desktop conversation. Incoming changes follow the bottom only if the reader was already near it; an empty transcript starts at the top.
- Read aloud retains its existing speech/cache/cancellation behavior with matching typography, spacing and a flexible editor. Customize retains its native dialog, contained scrolling, preferences, Escape dismissal and explicit focus restoration. Its instruction textarea uses 16 px text to avoid small-input mobile zoom.

Below 900 px the stage moves above chat. It displays the full original character using presentation-only contain framing; backgrounds still cover their area. This keeps eyes and mouth visible for all three characters without changing geometry or source pixels. The composer accounts for the bottom safe area. Short desktop windows allow workspace scrolling; Read aloud keeps natural mobile page scrolling.

The shared stage's new optional CSS properties (`--character-stage-fit` and `--character-stage-position`) affect presentation only. Their defaults preserve preceding laboratory framing. No HTTP/WebSocket contract, persistence schema, provider setting, budget, character resource or rendering algorithm changes.

## Verification and evidence

Evidence, original guidance and source copies are retained in `.tools/conversation-ui-32/`. Before/after screenshots cover 1280 × 720, 1440 × 900 and 390 × 844 in light and dark themes. Focused Chromium/WebKit checks exercise starters, explicit sending, history scrolling, unavailable chat, character changes, customization focus, reduced motion and a simulated visual-viewport keyboard resize. Existing conversation and write-and-listen fixture checks cover streaming, interruption, playback, cache reuse and stale-result cancellation.

The initial checks exposed a desktop empty-state clipping issue: its intrinsic content exceeded the transcript's available height by roughly 5 px. Earlier mobile cover framing also cropped the mouth; the current contain framing corrects it. Preserve the preliminary records in `first-check/`, `second-check/` and `first-pass/` alongside final results.

All browser review uses intercepted local HTTP/WebSocket replies and PCM fixtures; no provider call or speech-ledger insertion belongs to this change. Archive/restore this evidence with the generic PostgreSQL preparation archive, never a historical motion finalizer. Physical-keyboard/phone behavior, visual owner acceptance, sustained rendering performance and public-upload readiness remain pending. A simulated keyboard is not a real-iPhone verification.

The final bounded run passes 19 cases with one explicit Windows WebKit microphone skip (its existing Web Audio limitation). All six viewport/theme combinations pass in both tested browser projects. Type checking, validation-only production build, protected demo build, ESLint and Prettier checks pass. The local web container is rebuilt for the existing port 5173; API, processor and PostgreSQL services are not redeployed.
