# Microphone-only conversation workspace

The [recognition-turn correction](recognition-turns.md) groups nearby confirmed speech fragments into one message after a brief settling pause, preventing an SDK phrase boundary from immediately launching and interrupting separate replies.

The owner selected voice as the sole conversation input on October 20, 2026. The default local workspace removes the message textarea, Send, writing suggestions, keyboard-send hint and Read aloud tab. The existing transcript remains visible, including recognized speech, streamed replies and interrupted turns. Customize remains available; its character instructions are a preference rather than a conversation message.

## Composition and behavior

- `DemoWorkspace` composes the existing character stage, ChatPanel and customization dialog. It no longer mounts SpeechEditor or owns a mode switch. Historical speech components, APIs and laboratory entry points remain available in source.
- `ChatPanel` owns the header, current status, transcript and errors. `ConversationMessages` presents conversation history and a microphone-oriented empty state.
- `ConversationControls` receives readiness, recognition, microphone/startup and playback state, and emits microphone/stop events. The primary button starts or turns off capture. Stop reply appears during preparation or playback and restores focus to the microphone button when used.
- `useConversation` retains recognition, streaming, cancellation, playback and persistence. Startup is guarded against repeated clicks and suspended attempts. New chat closes the old microphone/session and connects the new conversation without opening capture.

Microphone access requires an explicit button press. Recognized final speech creates a turn through the existing backend; acoustic activity alone only interrupts playback. Partial recognition stays visible. Capture ends on manual shutdown, hidden-page suspension, recognition failure or new chat. Microphone errors offer recovery instructions without claiming typed chat is available. Missing recognition and unsupported browser audio have explicit states. No automatic paid request, new endpoint, dependency, asset or provider allowance is introduced.

The microphone control is now a centered 64 px circle with its action label below. A three-column control layout preserves its center when Stop reply appears; narrow screens retain the stop action as a labelled icon button. Existing Chromium/WebKit layout and focus checks pass, with desktop/mobile composition review and screenshots in `.tools/circular-microphone-38/`.

The compact-header follow-up removes the redundant character/mode toolbar. Customize now occupies ChatPanel's header-actions slot beside an icon-only New chat button; the existing launcher ref still restores customization focus. Theme switching is icon-only with an action tooltip. All three controls retain accessible names and 44 px targets. A 56 px brand header and 12 px workspace top padding reclaim 76 px above the desktop panels compared with phase 38. Eight existing Chromium/WebKit checks cover theme persistence, customization/focus, mobile keyboard resizing and both-theme layouts at three sizes; desktop/mobile screenshots were visually inspected. Source and evidence are in `.tools/compact-header-39/`.

## Bounded verification

The subsequent [progressive speech path](progressive-conversation-speech.md) reduces the wait between visible reply text and audible playback. It overlaps opening-phrase preparation with model generation, then delivers ordered native speech parts while later parts are prepared.

Type checking, scoped lint, formatting and production build pass. Offline Chromium/WebKit scenarios cover transcript streaming, long responses/recognition text, stale events, interruption, new sessions, unavailable services, keyboard focus, customization persistence, reduced motion and the layouts 1280 × 720, 1440 × 900 and 390 × 844 in both themes. Desktop/mobile compositions were visually inspected. Chromium exercises actual AudioWorklet processing with a local synthetic capture stream, partial recognition, activity interruption, manual shutdown, hidden-page shutdown and cancellation of pending permission access.

Windows WebKit lacks Web Audio: its UI/focus/failure checks pass, while its actual capture/playback scenarios are explicitly skipped. Physical-device microphone and browser/system permission behavior remain owner checks. No provider request is used by these fixtures.

Before-source copies, screenshots, test reports and current source copies are in `.tools/voice-workspace-37/`, preserved with the generic PostgreSQL archive and verified restoration. Earlier reply-length evidence and all character/performance packages remain unchanged.
