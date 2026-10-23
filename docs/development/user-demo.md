# Conversation workspace

Current application routing is described in [backend-only player](backend-only-player.md). The former client-rendered workspace below is historical; its ChatPanel and mobile viewport behavior are now reused by the remote player. `/` no longer imports or activates this demo.

Current interaction: [microphone-only conversation](voice-conversation.md) supersedes the earlier typing and Read aloud interface described below. The default workspace uses an explicit Start talking button, retains the visible transcript and supports spoken interruption and Stop reply.

Latest reply policy: [complete conversation replies](full-conversation-replies.md) removes the AI's 500-character/three-sentence restriction and prepares the full answer for audio. The shortened-audio notice is removed; original paragraph formatting stays in the transcript.

Latest behavior: [contextual performance direction](../experiments/motion/contextual-performance.md) keeps speech centered, supports sparse validated head cues and removes habitual completion smiles. `/?performance=previous` preserves the preceding behavior. Actual Ava laughter audio is accepted, but its facial companion failed composition review and remains disabled. The current toolbar shows Syntelle only and has no language selector; the original three-character/language description below is historical.

Current interface: the [conversation workspace refresh](conversation-workspace.md) aligns the desktop toolbar and 5:7 panels, adds English/Spanish starter suggestions and a compact composer, and adapts the mobile workspace to the visible keyboard viewport. It retains Chat as the default and Read aloud as a separate mode. The original interface record below remains historical context.

The local development entry at `/` now presents a user-facing demo rather than the technical motion workbench. It follows the original two-panel design: one animated character on the left, a text editor and playback controls on the right. Mobile uses a shorter stage above the editor, natural scrolling and a customization bottom sheet. Both existing themes and the supplied Colors Studio logos are retained.

## Available behavior

- Choose Syntelle, Café or Woodland. The catalog references only the three current oral packages, phase-18 RGB-D packages and required browser materials. It loads the selected package definitions and decodes its materials on demand; selector previews reuse existing base images. Raw generations, overlays and historical variants are not included in this catalog.
- Enter up to 500 UTF-16 code units, choose English or Spanish, then Speak. Pause, Resume and Stop operate the existing audio-clock playback. Ctrl/Command+Enter also starts speech. Text remains editable and is retained across character changes and completed playback.
- Switching characters aborts pending synthesis, stops audio and disposes the previous rendering session. Late responses cannot start obsolete playback. Repeating the same text/language/voice within a character session reuses the existing client cache; the server's PostgreSQL ledger and original ceilings remain authoritative.
- Customize the three supplied backgrounds, background-only blur (0–20 px), Ava/Andrew voice, blinking and natural movement. The drawer uses native dialog focus containment, Escape dismissal, explicit focus restoration and contained scrolling. Voice/language changes are disabled during synthesis or playback.
- Fresh defaults are Syntelle, English, Light Studio, zero blur, Ava for Syntelle and Andrew for the other characters. Background settings reuse the existing local preference. Animation and per-character voice choices use `syntelle.demo.appearance`. Entered text, audio and the selected character are not persisted.
- Character loading failures and unavailable speech have explicit states. The editor remains usable while loading or when speech is unavailable. No synthesis occurs merely by opening, switching or personalizing a character.

Reduced motion disables autonomous movement and blinking while retaining speech articulation. Existing hidden-tab suspension, cancellation, partial expression uploads and WebGL disposal are reused. The shared stage displays a source foreground while rendering loads and uses proportional, top-aligned framing without changing geometry or source pixels. No rendering algorithm or gesture scheduler was redesigned.

The subsequent [visible idle correction](../experiments/motion/idle-posture.md) adds a common optional runtime posture profile to this demo: 4.8–6.8-second eased movements and 0.8–1.8-second holds within the existing turn envelope. The original laboratory timing and frozen packages remain unchanged.

The latest [coordinated attention correction](../experiments/motion/coordinated-attention.md) supersedes that idle in the user demo: diagonal head adjustments and viewer-focused iris/pupil movement use an optional source-bound eye preparation. `/` and `/?idle=attention` select it; `/?idle=previous` preserves the preceding movement. Natural movement governs both head and gaze; existing speech, blink, breathing and laboratory behavior remain available.

[Facial expressions](../experiments/motion/facial-expressions.md) add Smile and Raise eyebrows controls under Customize, plus an independent Automatic expressions during speech setting. Manual values are transient; the automatic preference is local. Reduced motion retains static manual expressions. `/?expressions=off` preserves a comparison without this extension. The soft expressions use existing source pixels; no image/provider request is made.

## Code boundaries

`apps/web/src/features/demo/` contains the workspace, explicit catalog and appearance preferences. Its components have focused responsibilities:

| Component | Responsibility and contract |
| --- | --- |
| DemoWorkspace | Own selection, draft, language and preferences; load explicit package contexts and connect controls to the current session. |
| CharacterSelector | Display supplied character previews; emit a typed selected character ID through `v-model`. |
| DemoCharacterStage | Receive oral/motion contexts and settings; own one existing renderer/playback lifecycle and expose its session to the workspace. |
| SpeechEditor | Display typed playback/readiness state; emit Speak/Pause/Resume/Stop and draft/language changes. |
| CustomizationPanel | Edit appearance/voice models in an accessible dialog; receive the launcher for reliable Safari focus restoration. |

Reusable presentation is in `features/character/components/CharacterStage.vue`. The old `experiments/motion/MotionStage.vue` is a compatibility adapter. Animation, speech and rendering remain in the shared character feature and do not import the demo catalog. App remains a thin view selector.

## Preserved entry points and boundaries

- `/`: new local development demo.
- `/?view=validation`: existing image-file validation slice. Historical validation/isolated module tests use this entry explicitly.
- `/?trial=motion&character=syntelle&head=natural`: existing RGB-D laboratory.
- `/?trial=motion&character=syntelle&head=neck-compare`: separate neck candidate; not promoted by this UI change.
- `/?trial=viseme-a`: existing mouth laboratory.

The new demo and laboratory remain development-only. Production builds retain the validation slice and exclude prepared character assets and local speech UI. This delivery does not expose the local synthesis endpoint publicly, deploy anything, add conversation/microphone functionality, complete upload preparation or imply public-launch approval.

No image, mouth, eye, foreground or depth resources were regenerated. No model request, provider budget change, migration or credential change belongs to this work. PostgreSQL continues to store actual synthesis records through the existing Go service; appearance preferences are local browser preferences, not fabricated server records.

## Bounded verification

Frontend formatting, scoped lint with zero warnings, type checking and production build pass. Ten focused browser checks pass: three demo scenarios on Chromium desktop/mobile, the personalization scenario on WebKit desktop/mobile, one existing upload flow and one existing laboratory background/pixel-invariance scenario. They cover character/draft state, personalization, focus restoration, preference restoration, speech playback/cache/cancellation, unavailable speech and reduced motion. No full pose/browser matrix was rerun.

Speech verification uses explicitly labelled local PCM fixtures intercepted before the API; no Azure call or speech-ledger fixture insertion is made. Screenshots cover both themes and layouts. These are functional/layout checks, not a new renderer benchmark or real-phone validation. Owner visual acceptance remains pending. Evidence and a source copy are kept in `.tools/user-demo-22/` and archived separately through the generic PostgreSQL archive, without running a historical motion finalizer.


## Conversation follow-up

The local workspace now defaults to Chat and retains this original Write-and-listen interface under **Read aloud**. See [local conversations](local-conversations.md) for private Foundry setup, microphone behavior, PostgreSQL sessions and the explicit uncapped owner policy. Historical speech allowance records remain unchanged; live Luna and physical microphone acceptance are still pending.
