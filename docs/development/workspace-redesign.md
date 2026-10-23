# Syntelle workspace redesign

The owner approved a complete conversation-screen redesign inspired by the supplied studio HTML. The implementation retains DM Sans, the graphite/violet palette, both themes and the original Colors Studio SVGs. The source HTML is preserved verbatim in `.tools/workspace-design-42/reference-original.html` (SHA-256 `d21acb557c8c46cacd28fbde10b467a87543c853216b032cd545621f1a7b201b`).

## Composition and boundaries

- `DemoWorkspace` orchestrates the existing character/session and conversation bindings. Its desktop columns are 55/45 with a 20 px gap; the demo header and content share a 1,600 px outer shell. Panels have 16 px corners, restrained shadows and violet edge accents. The selected background and renderer remain intact.
- `ChatPanel` composes the heading, secondary action group, independent transcript and voice controls. User messages use a soft violet tint; assistant messages use a raised neutral surface. Text rendering and scrolling behavior are preserved.
- `ConversationControls` derives the visual state from existing flags and retains microphone/Stop commands and focus restoration.
- `MicrophoneControl` presents the 64 px light-center button, dark microphone icon, violet ring, volume halo and state ring. Its only emitted event is `toggle`; it exposes only an imperative `focus` method to its parent.
- `useMicrophone` owns the local `inputLevel`; `useConversation`, `DemoWorkspace` and `ChatPanel` pass it to the controls. No provider, socket or character-animation code consumes the meter.

Mobile uses `clamp(180px, 30svh, 260px)` for the stage, above the independently scrolling transcript. The current common face-relative CSS crop of the existing foreground/poster bounds the visible upper body and keeps the portrait attached to the frame. The preceding 1.18 mobile crop is preserved in phase 43/44 evidence. It changes presentation only; source materials, preparation and geometry stay unchanged. Keyboard suspension and the existing safe-area footer remain supported. The existing internal character selection is exercised in fixtures without introducing a new product selector.

## Local microphone feedback

The existing AudioWorklet computes PCM16 RMS once per approximately 100 ms packet, alongside its existing little-endian serialization. For `rms = sqrt(mean((sample / 32768)^2))`, the normalized level is `clamp((20 * log10(max(rms, 1e-8)) + 48) / 30, 0, 1)`. The number accompanies the existing local `pcm` port message; only its original PCM bytes go to recognition. Activity gating, packet acknowledgments, resampling and interruption rules remain unchanged. No additional AudioContext, analyser, animation loop or network message is introduced.

Halo scale and opacity transition for 120 ms. Silence has a stable ring, without periodic listening glow. Stop, capture failure, hidden-page suspension and scope disposal reset the level through the existing stop path. A reply error has a static treatment even if capture is still enabled.

| State | Presentation |
| --- | --- |
| Off | Stable light button, violet ring and Start talking label |
| Starting | Restrained progress ring, disabled button, existing permission hint |
| Listening | Capture outline and halo driven by actual PCM volume |
| Preparing | Slow progress ring and existing Preparing reply header status |
| Speaking | Static dashed ring, reply-audio label and adjacent Stop reply; microphone capture stays separately labeled |
| Error | Static error ring/icon and existing recovery message |

Reduced motion disables progress rotation, press scaling and volume animation, retaining static rings, text and capture cues. Existing accessible button names, keyboard focus, minimum 44 px targets and Stop-to-microphone focus restoration are retained. [Brand guidance](../product/brand.md) now permits these controlled violet gradients and records the new surface tokens.

## Bounded verification

Offline fixtures intercept all API/WebSocket traffic. They cover silence, two increasing input amplitudes and return to silence through the real worklet, permission startup, preparation, speaking with capture on/off, confirmed interruption, connection loss, capture failure and level reset, reduced motion, streamed text and complete ordered audio queues. Existing customization, keyboard resizing, microphone start/stop, long-history scrolling and three-character selection checks pass.

The initial Chromium run passed 42 cases; two new framing cases ran before the async workspace mounted. Their failed traces are preserved, and the fixture now waits for the workspace before inspecting selection. Both framing cases then passed. After the mobile framing/control-boundary refinements, four focused Chromium cases passed. Windows WebKit passed 30 cases with 14 existing audio/capture skips; its two framing cases passed again after the final CSS change. This does not establish actual Safari audio playback or physical-device acceptance.

Desktop/mobile screenshots in both themes cover Syntelle, Café and Woodland. Inspection confirmed visible faces, attached mobile cutouts, readable message surfaces and accessible voice controls. Scoped ESLint, Prettier, `vue-tsc` and the production build pass. Verification introduced zero paid calls. All 590 experiment assets and both company SVGs retain their SHA-256 hashes. The production build remains validation-only and excludes the development demo/resources.

## Evidence and delivery

Before sources/screenshots were archived separately in PostgreSQL snapshot **496**, experiment `workspace-design-42-before`. Current sources, reference, screenshots, failed fixture evidence and check reports are stored separately under `.tools/workspace-design-42/`, using `app.audits.archive_preparation`; see the sibling `.tools/workspace-design-42-archive.json` for final snapshot and restoration hashes. Do not use a historical motion/speech finalizer for this UI change.

```powershell
# From services/processor; no provider calls.
.venv/Scripts/python.exe -m app.audits.archive_preparation --directory ../../.tools/workspace-design-42 --experiment workspace-design-42
# Rebuild only the local web service after checks pass.
docker compose build web
docker compose up -d --no-deps web
```

Speech behavior, provider choices, budgets, schemas, dependencies and character assets are unchanged. Keyboard input, replay, playback speed, presets, device selection and push-to-talk remain outside this delivery. Owner visual acceptance, physical microphone/speaker checks, real phones and earlier sustained-rendering/public-upload limits remain pending.


## Owner correction: original header and reference controls (phase 43)

The owner rejected phase 42's visual result, specifically its changed top bar, outlined microphone and control/icon styling. The current correction restores `AppHeader.vue` byte-for-byte from phase 42's before source. Its original dimensions, Colors Studio treatment, theme button and `AppIcon` glyphs are retained. The conversation content still uses the approved 55/45 layout.

`ConversationIcon.vue` provides one local 24 px SVG vocabulary for the conversation and customization surfaces, with consistent optical size and non-scaling strokes. Its microphone is a solid glyph, matching the supplied reference's treatment. `MicrophoneControl` now uses a white 64 px center, a separate 82 px orbit and soft masked violet glow instead of a thick painted rim. The existing real-volume meter and all state/command contracts are unchanged. Quiet New chat, a tactile customization button, circular Stop reply, consistent drawer/reset controls and stronger focus treatment retain 44 px minimum targets. The original top-bar control stays outside these styles.

The reference also informs the bordered conversation symbol, real status dot, separate speaker metadata, violet user-message outline, raised assistant surface and stronger corner/voice-hub light. No fake telemetry, extra actions, presets or keyboard mode are introduced. Both themes and reduced-motion/static-error behavior remain supported.

Phase-43 before/current sources, rejected phase-42 view, desktop/mobile captures and check reports are separate in `.tools/workspace-controls-43/`. Use generic PostgreSQL archive/restoration, with snapshot metadata in the sibling `.tools/workspace-controls-43-archive.json`. Preserve phase 42 and its snapshots 496/497 as rejected design evidence. Current offline checks cover framing for all three characters, both themes, customization, volume/silence/reset, startup/errors, streaming and complete playback. Physical-device and owner visual acceptance remain pending. No provider calls, speech/animation logic, character asset, budget, dependency or schema changes are involved.


## Approved polish: explicit actions (phase 44)

The owner liked phase 43 and requested a narrow icon/action correction. The theme control now has a solid crescent and a solid-center sun, retaining its existing accessible name, pressed state and 44 px target. The top-bar geometry and company marks remain unchanged. New chat now displays its action label instead of an ambiguous plus. Decorative chat symbols are removed from both the conversation heading and empty state; the unused local glyphs are removed as well. Voice controls, speech behavior and character rendering are unchanged.

Before/current source bytes and captures are separate in `.tools/workspace-actions-44/`, with generic PostgreSQL archive/restoration metadata in `.tools/workspace-actions-44-archive.json`. Verification is limited to theme switching, existing new-chat/customization/layout fixtures, scoped lint/format/type checks and the production build. No paid provider calls or character-resource changes are involved.


## Bounded bust framing and quiet icon actions (phase 45)

The owner requested less visual framing around icon buttons and a portrait range between the former shoulders-only wide view and the overly long torso view. The theme, customization and drawer-close controls now have transparent borders/backgrounds without painted outlines or shadows. Stop reply uses the same quiet treatment. Hover and keyboard focus remain visible; 44 px targets, text-button treatment and microphone presentation are retained.

`DemoCharacterStage` now derives presentation-only CSS variables from each existing motion package's face height, chin and source canvas. The view spans 2.6 face heights and ends at `min(source height, chin + 0.9 face height)`. It uniformly scales the original canvas/poster with its original aspect ratio, centers it horizontally and crops it at the stage boundary. An absolute inset composition binds CSS size-container units to the visible stage, without an observer, new animation loop or renderer resize. A failed WebKit check exposed percentage-height resolution against the preferred aspect height rather than the max-height-constrained box; the absolute inset corrects that behavior. Failed traces and diagnostic captures are retained. This preserves the selected background's separate fill/blur and every cached character pixel. Available source material bounds the shorter Café portrait; no hidden torso is generated.

Desktop stage height follows a 4:5 preferred aspect, limited by available height and a 960 px maximum. The transcript retains its existing available height. Mobile retains its existing `clamp(180px, 30svh, 260px)` stage. The preceding cover crop and 1.18 mobile scale are replaced only in the user demo, leaving technical laboratories, 3D geometry, eye/mouth patches and motion timing unchanged.

Focused browser checks verify original canvas proportions, bounded face size, upper-body space below the chin, both themes and all three cached characters. Additional Syntelle captures at 1900x945, 1280x1200 and 1280x720 verify the chest crop across wide, tall and short windows. Scoped lint/format/type checks and the production build accompany bounded layout/customization checks. Owner visual acceptance and physical-device review remain pending.

Before/current sources, the previous workspace, captures and reports belong in `.tools/portrait-framing-45/`; generic PostgreSQL snapshot/restoration metadata is in the sibling `.tools/portrait-framing-45-archive.json`. Preserve previous phase evidence. No paid request, asset regeneration, provider-budget/schema/dependency or speech behavior change is introduced.


## Shared desktop panel height (phase 46)

The owner identified phase 45's unequal desktop panel bottoms. The height limit now belongs to the shared workspace row instead of the character panel. Its height is the minimum of available height, the 55% character column's 4:5 budget and 960 px. Both columns fill that row and retain aligned top/bottom edges. The existing face-relative bust crop, uniform source proportions, independent transcript scrolling and voice-control placement are unchanged. Mobile keeps its preceding stacked height rules.

Focused checks at 1900x945, 1280x1200 and 1280x720 verify equal panel heights, top alignment, microphone visibility and the same bounded chest crop in Chromium and WebKit. Mobile/layout checks and scoped formatting/lint/type/production-build validation remain bounded. No image, animation, speech, budget, provider or dependency changes are involved.

Evidence and before/current sources are separate in `.tools/aligned-workspace-46/`, archived/restored with the generic PostgreSQL workflow; sibling archive metadata records the snapshots and byte matches. Preserve the unequal phase-45 view as design evidence. Owner visual and physical-device acceptance remain pending.


## Full-height panels with independent portrait sizing (phase 47)

The owner rejected the large unused area below phase 46's shared height cap. Both desktop panels now fill the available workspace height and retain aligned edges. The character is sized independently inside its stage: its face-relative bust frame is the minimum of stage height, 1.5 stage widths and 960 px, anchored to the stage bottom. The selected background fills the remaining stage area, providing headroom in tall windows. Uniform source aspect, the existing chest crop and bounded face size are preserved; the character does not grow with unlimited viewport height. Mobile's stacked layout remains unchanged.

Focused desktop checks cover viewport filling, equal panel edges, visible microphone, maximum face size and retained chest crop at 1900x945, 1280x1200 and 1280x720 in Chromium and WebKit. Existing mobile layout/scrolling checks and scoped lint/format/type/build validation stay bounded. No character assets, rendering algorithms, motion timing, voice behavior, provider budgets or dependencies change.

Preserve phase 46 as the rejected page-gap experiment. Before/current source and viewport captures are separate in `.tools/viewport-workspace-47/`, with generic PostgreSQL archival/restoration metadata in the sibling archive report. Owner visual and physical-device acceptance remain pending.


The normal Compose web rebuild encountered a Docker Hub TLS handshake timeout. Delivery reused the already installed local `syntelle-web` image with `Dockerfile.local` in this evidence tree, copying current source with build networking disabled and unchanged dependencies. Only the web service was recreated. The failed registry build and successful local refresh are preserved; the repository Dockerfile remains unchanged.
