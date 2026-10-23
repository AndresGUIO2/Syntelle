# Backend-only player

The owner requested one current interface with character rendering exclusively in the backend. `/` and `/?view=stream` now load the same remote workspace in development and production. Former `trial`, `head`, `oral`, `idle`, `expressions`, `character` and `view=validation` query parameters cannot activate client rendering. Historical source, prepared packages, prompts and evidence remain preserved.

`App.vue` composes the header and remote workspace only. The public player decodes a WebRTC video/audio stream; the private native Go worker owns character animation, rendering and encoding. Missing/disconnected backend media leaves the remote player unavailable with reconnection controls. It never downloads a local character package or starts a canvas renderer.

## Current interface

Pricing is now a [standalone `/pricing` page](pricing-page.md), with normal header,
home and browser-history navigation. It no longer opens a dialog or mounts a
streaming session. Shared language and theme remain available; the privacy policy
is linked from the account dialog.

The microphone is the primary start action. When no stream is active, turning it
on connects the remote player and starts capture as soon as media is ready, so a
separate **Start conversation** click is unnecessary. The browser may still ask
for microphone permission after the user presses the control. Reconnect and resume
actions remain available for recovery states.

### Platform language (October 22, 2026)

The header's **English / Español** selector now controls the entire interface and
Syntelle's speech/recognition language. There is no separate chat-language control.
The global preference survives reload and takes precedence over saved setup
languages. See [platform language](platform-language.md) for behavior and bounded
verification.

### Conversation language (historical phase 72)

The following record is superseded by the global platform-language selection above.

**Customize → Conversation language** offers **English** (`en-US`) and **Español**
(`es-CO`). The selected language applies to microphone recognition and spoken
replies, with Ava retained for both. English remains the fresh default. The
existing local settings and saved account configurations retain the choice.
Changing it uses the existing stream reconnection path; the selector is disabled
during connection setup, microphone startup/capture and pending/spoken replies.
Interface labels remain in English. No provider call is needed to change language.

Eight focused Chromium/WebKit desktop/mobile fixture checks cover both language
directions, reload persistence, Ava retention, stream replacement and reply-time
locking alongside the existing appearance/transcript controls. Type checking,
scoped lint, production build and two public-boundary checks pass. The existing
pricing panel was reviewed and added to the public-source allowlist after the
initial build and browser checks exposed its missing registration. Evidence and
source copies are in `.tools/conversation-language-72/`, with generic PostgreSQL
archival and verified restoration. Only the local web container is replaced;
no paid request, character asset or backend change is involved. These fixture
checks do not establish new live recognition or physical-device approval.

### Background continuity and picture in picture (October 22, 2026)

Sound-control correction: the former one-shot **Turn sound on** action is now a
persistent icon button, matching the PiP control's 44px target and styling. A
crossed speaker indicates the user's muted preference; the normal speaker indicates
sound enabled. Accessible action names/tooltips switch between **Unmute sound**
and **Mute sound**, and the control remains mounted after activation. Browser
playback rejection restores muted state and keeps retry available. Async activation
attempts are invalidated on media replacement, mute and disposal. Internal stale
output suppression remains independent of the user's sound preference.

Evidence is `.tools/sound-control-64/`. Six focused Chromium-desktop/WebKit-mobile
fixture checks pass for persistent controls, keyboard focus, mute/unmute, mute
retention across reply events, failed activation/retry and background continuity.
The initial fixture mistakes (asserting audible output while stale-output
suppression was still active, and assuming mobile tap focus) remain recorded.
Type checking, scoped lint and the public build pass. Only the local web container
is updated; no provider call, backend/worker change or asset regeneration occurs.

Follow-up: the PiP control is now a 44px icon button with an accessible name and
tooltip, without visible text. The player also registers the browser Media Session
`enterpictureinpicture` action. The browser can use it to open the same live video
automatically when the user leaves an eligible tab; repeated callbacks never close
an existing window, and automatic denials remain quiet. Unsupported action
registration retains manual PiP, and disposal unregisters the action.

This uses the browser's automatic action rather than attempting to bypass gesture
requirements in `visibilitychange`. As documented by
[Chrome](https://developer.chrome.com/blog/automatic-picture-in-picture-media-playback),
automatic media PiP requires playing media with audio focus, recent audibility,
site eligibility and the browser's permission/engagement conditions. A silent idle
portrait does not establish eligibility. The browser may ask permission and handles
return-to-tab closure for automatically opened windows. Other browsers retain their
own support and rules; no universal or silent automatic-opening guarantee is made.

Follow-up evidence is `.tools/automatic-pip-63/`: eight focused Chromium/WebKit
desktop fixture checks pass, including the automatic action, visible-tab no-op,
duplicate action, quiet denial, icon accessibility and unchanged session counts.
Browser automatic-action dispatch is simulated in those checks; the separate real
Chromium/native idle test verifies manual PiP and continuity, not real automatic
permission eligibility or physical-device behavior. Type checking, scoped lint and
the public production build pass. Only the web container is replaced locally.
No paid call, worker/asset change or new rendering code is involved.

Tab visibility no longer deletes the stream, closes signaling/WebRTC, interrupts
the current reply or creates a replacement session on return. Pending connection
setup and bounded transport recovery can also finish while the tab is hidden.
Microphone capture still stops when leaving the tab and requires an explicit
action to restart. New chat, account changes and component disposal retain their
existing release behavior. A genuine connection failure still uses the existing
bounded recovery policy without replaying user commands.

The video offers **Picture in picture** when the browser exposes a supported
video API, with Safari presentation-mode fallback. Opening it requires an explicit
button click; browser close events restore the button state. Denials show a local
message without reconnecting. The same video/stream supplies the floating window;
no private rendering resources enter the browser. Audio retains its existing
explicit sound opt-in. Browser/OS freezing, tab discard, app termination and
physical-device PiP behavior remain outside this short desktop verification.

Evidence and before/current sources are in `.tools/background-playback-62/`.
Twelve focused Chromium/WebKit desktop/mobile checks pass for hidden-tab replies,
media identity, unchanged session creation/deletion counts, PiP enter/exit/browser
close, denial and unsupported controls. PiP APIs and visibility are explicitly
simulated in these protocol tests. A separate actual Chromium/native idle test
opens real browser PiP, receives advancing 720x960 video while dispatching hidden
visibility, and returns with one session and zero visibility-triggered deletions.
The initial checks against the stale container are retained separately. Type
checking, scoped lint and production build/private-source checks pass. Only the
local web container was rebuilt/replaced; no provider request, worker replacement,
asset generation, schema or budget change occurred. Generic PostgreSQL archival
and byte-verified restoration preserve this change independently.

The remote workspace now reuses the latest `ChatPanel`: microphone-only conversation, centered microphone with status below, transcript, Stop reply, New chat, and customization. The obsolete stream composer and exact-text mode are removed from this workspace. Accounts and saved configurations remain available. Background, voice, movement, blinking, expressions and instructions still use the existing backend settings protocol.

The mobile viewport helper now lives in `features/conversation/composables`; the historical demo import re-exports it. The remote workspace uses the same visual viewport/keyboard handling and mobile customization sheet. Existing stream preferences take precedence. If absent, supported appearance, background and Syntelle instructions migrate from the old local demo preferences. No old draft, audio or provider record is deleted.

## Public/private boundary

Both `npm run build` and `npm run build:player` enforce `config/player-boundary.ts`. Their dependency graph must exclude character/demo renderers, laboratory modules and private resources. Only the three background thumbnails may cross the experiment resource boundary. Normal output is `dist/`; the compatibility stream build uses `dist-stream-player/`.

The development server also refuses direct requests for private renderer/laboratory sources and materials. Client-demo build modes (`demo-baseline` and `demo-protected`) fail explicitly instead of exporting another character renderer. The former private Vite entry and Node/Chromium render worker are retired. Historical modules and immutable comparison snapshots remain as evidence; deterministic TypeScript exporters still verify native behavior. The default browser suite targets the remote workspace, and Playwright/Chromium remains a receiver/test dependency only. Historical browser-rendering suites are no longer selected by that public application configuration.

## Docker Desktop media path

The first Windows receiver check exposed an existing local transport gap: the native worker advertised container-internal ICE addresses and ephemeral UDP ports that were not published to Windows. Its successful Linux container-network checks did not establish Windows-host connectivity.

The local native launcher now selects `--local-media-port 8081`. The worker shares one Pion UDP mux, advertises `127.0.0.1:8081`, and Compose publishes that UDP port on loopback only. ICE/DTLS/SRTP share this port; private worker control, character assets and credentials are not exposed. The worker owns and closes the socket after its media hosts stop. `--local-media-port 0` explicitly preserves the direct ICE path used by container-network comparisons. Other ports require an identical loopback Docker publication. No public STUN/TURN service, cloud route or new dependency is added.

The local transport image is built from the frozen `native-efficiency-57/before-source` baseline, which matches the running `syntelle-render-native:local` image identity, plus only the media-port changes. This avoids installing concurrent efficiency candidates. Its exact isolated build context is retained in `.tools/backend-only-workspace/native-build/`; current repository sources also contain the same optional transport wiring. Existing character assets and animation/rendering behavior remain unchanged.

## Bounded verification

The twelve isolated UI/protocol checks pass in Chromium and WebKit desktop/mobile. They cover the root/compatibility/retired query links, microphone centering, no canvas/private downloads, direct private-source rejection, migrated settings, backend configure/transcript/interrupt/new-chat behavior and unavailable-backend behavior. Windows WebKit lacks WebRTC/MediaStream, so these checks use explicit transport/media fixtures and do not claim actual decoding there. The initial fixture failure and corrected results remain in `.tools/backend-only-workspace/`.

Type checking, scoped lint, normal production compilation and its private-renderer gate pass. Local transport tests cover rejected ports, explicit direct-ICE mode, occupied-port failure, two peers sharing the advertised port and socket release. Native launcher checks preserve read-only resources and token-only environment forwarding.

The corrected Windows Chromium check receives advancing 720 × 960 native video at `/`, with zero page errors, private-resource downloads or provider calls. Its screenshot was inspected. This is a short functional check, not a sustained performance or physical-device benchmark. Live media results are recorded separately in `.tools/backend-only-workspace/live-review.json`. Earlier failed host-media checks remain evidence. No model, image, recognition or synthesis request is authorized by this interface/transport change. No provider budget, preparation assets, calibration, schema, concurrency claim, production network approval or physical-device acceptance changes.

Pion's [single-port setting](https://github.com/pion/webrtc/blob/v4.2.25/settingengine.go) supplies the existing mux/NAT configuration used here; dependency versions remain pinned.

## Public-source minimization (October 22, 2026)

The player boundary now allows only explicitly reviewed interface, account, transport,
viewport and microphone source files. Historical renderers, experiments, obsolete
conversation clients and the unused stream composer are excluded even from direct
Vite development requests. Unknown future source modules fail the production build.
Public background thumbnails have independent copies in `src/assets/backgrounds/`;
the historical originals remain unchanged and no experiment resource is allowed.

Vite filesystem access is restricted to the web application root. Development
requests for configuration, tests, scripts and repository evidence are refused.
Build checks reject unapproved resource extensions, including geometry, manifests,
images and source maps. A post-write check also covers files copied from `public/`.
Source maps remain disabled. Deploy only the resulting distribution directory;
these checks do not protect files independently exposed by a different web server.

The initial public bundle already excluded the animation algorithms, so this change
hardens that separation rather than claiming removal of a previously shipped engine.
Required interface and transport JavaScript remains inspectable. The server still
needs to restrict its own responses and private files; hiding interface strings or
obfuscating the receiver would not establish algorithm confidentiality.

Verification: two focused Node tests exercise actual Vite HTTP refusals, successful
interface-module responses and rejected build inputs. Type checking, production
build and scoped lint pass. No paid call, asset generation, backend algorithm change,
visual acceptance or production deployment is part of this change.


## Browser backgrounds (October 22, 2026)

The owner authorized moving background composition and blur to the frontend while
keeping all Syntelle character processing in the backend. The current live worker
sends a 1440 x 960 H.264 frame with premultiplied character RGB and full-size alpha
side by side; the public receiver produces the 720 x 960 presentation and PiP video.
This supersedes the preceding complete-opaque-scene contract. Character source
materials, geometry and animation code remain private. The original bitrate cap,
media clock, audio tracks, provider policy and prepared assets remain unchanged.
See [browser background composition](frontend-background.md) for implementation, rejected checks,
bounded validation and remaining performance/device limits.
