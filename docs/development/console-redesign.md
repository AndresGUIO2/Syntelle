# Console and character creation

## Historical neutral dark correction: phase133

The owner rejected phase132's purple background and primary action. Dark console/account surfaces now use deeper neutral graphite with ivory primary buttons and dark labels. Violet is limited to small accents. Sample-card actions receive the same neutral boundary; the approved light palette and rendered gallery remain unchanged.

The deeper canvas has 1.26:1 luminance separation from the existing sidebar. The canvas regression test now uses an explicit 1.2:1 dark-only product floor, replacing the earlier 1.5:1 target that favored lighter canvases. Its light target, full-frame consistency, bounds and overflow assertions remain unchanged. Text and control-boundary contrast targets are unchanged.

Evidence and phase132 rollback are in `.tools/neutral-dark-133/`. Bounded review covers desktop light/dark, wide/mobile dark, WebKit and account controls, including sample action colors and light-gallery screenshot hash equality. Production build, formatting and the two wide/mobile canvas checks pass with isolated API fixtures. No provider calls, new dependencies or renderer/resource changes are involved.

Archive with `python -m app.audits.finalize_console_redesign --experiment neutral-dark-133 --parent-snapshot 854`. Local installation changes only the web CSS. Keep owner acceptance and physical-device qualification pending.

## Historical dark palette correction: phase132

The owner approved the light-theme buttons and requested less gray in dark mode. Only dark console/account tokens change: indigo/plum canvas and panel layers replace the cool-gray surfaces, and primary actions use vivid violet `#8756E8` with white labels and a darker hover. The full frame shares the same canvas token. Sidebar, light-theme colors, card layout, portraits and conversation rendering retain their existing behavior.

`.tools/dark-palette-132/` contains the preceding source, isolated preview, screenshots, computed contrast pairs, production build and rollback to phase131. Review covers desktop dark/light, 2545px dark, mobile dark, WebKit and the shared account button. The light gallery screenshot is byte-identical before and after. The existing wide/mobile canvas checks remain unchanged and passing; no expanded rendering matrix or provider call is involved.

Archive with `python -m app.audits.finalize_console_redesign --experiment dark-palette-132 --parent-snapshot 853`. The local web image carries only the scoped CSS update. Keep owner acceptance and physical-device qualification pending.

## Current gallery card presentation: phase131

The included sample is now a photographic cover with a dark caption/action footer, keeping the original poster and readiness labels. The first-character entry uses a themed portrait-stack illustration, a more compact content hierarchy and a dedicated footer. Both entries keep the existing gallery width and responsive stacking, with 22px corners and a 500px minimum height.

`CharacterGallerySample` owns sample presentation and emits the unchanged navigation/connection events. `CreateCharacterCard` owns the single creation link and decorative illustration; `CharacterLibrary` composes it alongside collection state and personal cards. The new presentational component is explicitly listed in the public-source boundary. No dependency, portrait generation, provider request or preparation behavior is added.

Evidence, initial boundary failures, screenshots and phase130 rollback are in `.tools/character-cards-131/`. Verification uses isolated API fixtures for desktop dark/light, Spanish mobile and narrow reflow, a 2545px canvas, WebKit and a populated collection. Production build, source-boundary checks, scoped lint/format and the existing Chromium/WebKit collection-navigation checks remain bounded. Archive with `python -m app.audits.finalize_console_redesign --experiment character-cards-131 --parent-snapshot 852`. Owner visual acceptance and physical-device qualification remain pending.

## Current primary-button color: phase130

The owner rejected the pale lavender primary action. Shared console/account buttons now use muted violet `#7854BF` with white labels and icons, independently of the brighter link/focus accent. Light/dark hover colors and the contrasting dark boundary are documented in [the brand guide](../product/brand.md#console-primary-buttons-phase130). Both `.console-button.primary` and `.button-primary` consume the same scoped tokens; conversation controls retain their existing colors. Layout and interaction behavior are unchanged.

Evidence and the preserved phase129 web rollback are in `.tools/button-color-130/`. Verification is limited to the production player build, CSS formatting, rendered button contrast/states, desktop light/dark, mobile dark and a desktop WebKit check using isolated API fixtures. No provider requests are needed. Archive with `python -m app.audits.finalize_console_redesign --experiment button-color-130 --parent-snapshot 851`, never a historical experiment default. Owner visual acceptance remains pending.

## Current layout review: phase129

The full-frame phase128 palette is preserved. Complete page compositions are now centered within the available application canvas, with widths suited to their content. Operational pages use 1184px, billing 960px, integrations 1120px or 760px during setup, character creation 920px, contextual unavailable states 680px, and staff verification 640px. Character collection width follows its occupied columns. These are maxima, not fixed mobile widths. The surrounding canvas always uses the same theme surface; there is no differently colored central strip.

The integrations empty state pairs a 260px illustration with its copy across a 48px gap, replacing the nearly 400px gap measured at 2556px. Website rows put status below the title on mobile. Overview actions align with their copy; billing cards and top-up actions remain grouped. Mobile statistics pair labels with values, and workspace identity facts use a compact two-column lower row.

Portrait creation keeps photo selection visible inside the initial mobile upload area. Review and preparation align the portrait with readable content and actual failure explanations. Staff verification, legacy workspace controls and account settings use bounded forms. Long account lists scroll between persistent dialog controls.

The owner explicitly chose a temporary right-side history panel inside Test. Main navigation now remains unchanged when entering Test: no recent conversations or special history item appears in the sidebar. The Test toolbar opens `ConversationHistoryPanel`, which reuses the existing list and account-scoped history state. Selection previews saved text without media startup; explicit Continue remains required. Pagination, inline delete confirmation, Escape/backdrop closing, focus restoration and independent scrolling are preserved. The panel fills the mobile viewport and closes when leaving Test or changing identity.

Shared console CSS owns page alignment and responsive statistics; pages own their internal layouts; the history panel owns modal presentation; account settings own their dialog scrolling. No API, generation, rendering or speech behavior changes. Evidence is in `.tools/console-layout-129/`, including failed baseline checks, before/after screenshots and fixture-only interactions. Archive with `python -m app.audits.finalize_console_redesign --experiment console-layout-129 --parent-snapshot 850`. Local installation changes only the web image and retains phase128 rollback. Owner visual acceptance remains open.

## Preserved visual correction: phase128

The owner rejected phase127's narrow, sparsely composed gallery and weak navigation contrast. The 2545 × 1314 screenshot also exposed an actual theme bug: Vue's scoped compiler reduced `:global([data-theme='dark']) .navigation-shell` to the root selector alone, leaving the shell's own light canvas variable active. A complete-frame global theme rule now owns the background, so dark main content and the surrounding gutters cannot disagree.

Navigation is near-black `#0D1016`; the dark canvas/panels are cool graphite `#353740`/`#41444E`. Light canvas/panels are `#F2F3F7`/`#FFFFFF`. The main canvas spans all available space, with left-aligned content up to1600px and responsive gutters. Original Colors SVGs, navigation interactions, conversation presentation and media behavior are preserved.

Characters now uses a large included sample card beside a first-portrait entry and a gallery for personal cards. It removes the duplicated collection heading, repeated introductory text, tiny sample strip and large dashed empty box. `CharacterGallerySample` owns only the sample presentation; existing preparation, ownership and connection events are unchanged. The brand guide now documents this composition rather than treating the phase127 values as fixed constraints.

The 2545px dark failure was reproduced before correction. Bounded canvas tests cover four viewport widths in both themes and wide WebKit, recording actual backgrounds and overflow. The 1.5:1 surface contrast threshold is a regression target for visible separation, not a WCAG standard or proof of visual acceptance. Gallery checks cover populated/empty and mobile layouts. Production build and scoped frontend checks remain required before local installation.

Evidence and phase127 rollback live in `.tools/console-visual-128/`. Archive with `python -m app.audits.finalize_console_redesign --experiment console-visual-128 --parent-snapshot 849`; its external receipt verifies byte-exact restoration. No provider requests, API/worker changes or cloud deployment are involved. Owner acceptance and the earlier external-service release gates remain open.

## Preserved phase127 functional delivery

This candidate replaces the phase126 workspace-first presentation. Personal characters are available without a workspace. Website connections keep their own destination and ask for a workspace only as part of the first connection. Conversations and their history remain secondary under Test.

## Component ownership

- `BrandSignature`, the navigation panel and navigation items own original brand assets, grouped destinations and unclipped compact labels. The navigation shell owns resizing, scrolling and the mobile dialog.
- The character collection, upload steps, preparation receipt and account-scoped journey own the portrait-to-character flow. Prepared character state remains authoritative on the existing API.
- The integration setup, details and account/workspace-scoped journey own connection drafts, explicit sharing and publication. API credentials remain only in the existing transient credential view.
- The application shell owns authentication, route composition, validated connection intent and the existing retained chat lifecycle. It does not perform character generation or grant billing rights.

## Visual system

The navigation background is `#15131B` in both themes. Light console canvas/panels are `#F5F4F7`/`#FFFFFF`; dark console canvas/panels are `#302C38`/`#383340`. Console tokens do not change the conversation player. Main content is centered within 1280px, with 48px/40px desktop top/side padding and 24px/16px mobile padding. Controls retain 44px targets and visible focus.

The Colors Studio signature uses the original 120px-wide white-lettered SVG on dark navigation. Compact states retain `by` and the original symbol. No SVG recoloring, replacement or generation is involved.

## Journeys and failures

Character upload validates the supported file type, 5MB limit and existing image dimensions locally, retaining original bytes. Review is explicit before submission. Only server-reported preparation states appear; there is no generated progress percentage or automatic retry. Personal originals/portraits retain existing expiry behavior and intentional fallbacks. No identity editor or rename API is introduced.

Connection intent stores only validated identifiers and a fixed allowlisted destination. Ownership, role and readiness are checked through existing resources. Account changes clear transient forms and intentions. Image files and credentials are never serialized into browser storage.

Workspace, publication and integration creation preserve successful intermediate results. Uncertain mutation results cause a refresh and an explicit recovery state, not an automatic repeated POST. Existing publication copies are chosen explicitly when ambiguous. Draft saving does not publish an integration or start media.

Loading errors are distinct from an empty workspace list. Usage, Billing, Team and Settings preserve their own context before setup. Personal characters remain available to accounts whose selected workspace role is billing-only.

## Validation and installation

Use isolated API/browser fixtures for review. No image, analysis, synthesis, conversation-model or payment request is needed. Preserve native worker, API, processor, package and voice identities. The local web candidate is installed separately from phase126, with rollback in `.tools/console-redesign-127/rollback.ps1`.

Evidence lives in `.tools/console-redesign-127/`; `app.audits.finalize_console_redesign` archives sources and review evidence to PostgreSQL and verifies every restored byte. Never rerun the frozen phase126 finalizer for this change.

Actual Paddle Sandbox, Turnstile/public-network widget and physical-device qualification remain separate release gates. Windows WebKit navigation checks do not qualify Web Audio. Owner visual acceptance remains required.

The installed candidate is available at `http://localhost:5173/console` and `/characters`. Phase127 changed only the web image; the API, native worker and processor image identities match the frozen phase126 installation. `rollback.ps1` was parsed and its preserved image verified; it was not executed against the final candidate.

Technical review passed the production player build, scoped lint, public/private source boundary, 21 character-flow browser executions and 27 unique integration-flow/browser combinations. Seven shell checks passed with two intentional desktop-only skips. Twelve retained-conversation cases and a separate real-browser capture-stop test passed. Root fixtures cover unavailable workspaces, malformed or revoked handoffs, billing-only personal access and authentication return. Staff and disconnected-billing fixtures remain passing. See the individual receipts rather than treating test counts as production acceptance.

The review archive retains earlier failures and their corrections. `validation-summary.json` records limitations, including 200%-equivalent reflow rather than native zoom automation. The external PostgreSQL receipt verifies complete source/evidence restoration.
