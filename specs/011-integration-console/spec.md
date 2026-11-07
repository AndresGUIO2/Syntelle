# Integration-first console, widgets and Paddle sandbox

## Current approved visual refinement (phase136)

The owner approved the complete night-blue implementation plan. Its palette in `docs/product/brand.md` supersedes historical dark colors in REQ-2/9 and the unchanged-conversation-palette restriction; all functional requirements remain in force. Apply the shared night palette and ivory primary actions to customer/staff pages, navigation states, conversation, widget chrome and email headers. Preserve the approved light UI/email body, original logos and portrait bytes, layout and media/auth behavior. Widget theme changes must synchronize the outer host and iframe through the validated channel without starting media. Verify both themes, responsive layouts, contrast and fixture-only delivery/media behavior before local installation; archive reviewed sources, checks and installation receipts with verified byte-exact restoration. No new provider calls, schema, dependency or cloud deployment are included.

Approved by the owner on 2026-11-06. This replaces the conversation-first product hierarchy of phase125.

- **REQ-1:** An integration-first console with Overview, Characters, Integrations, Usage and Billing; workspace selection, secondary Team/Settings, separate staff console and subordinate character Test/history.
- **REQ-2:** Coherent graphite/violet desktop/mobile layouts, adjustable/collapsible sidebar, explicit hierarchy, no duplicate headings or detached back links, both themes/locales and accessible controls.
- **REQ-3:** Workspace-owned character publications reference immutable packages independently of personal originals. Draft editing and publication remain separate. No silent adoption of personal histories or keys.
- **REQ-4:** Integration-scoped API keys can start authorized custom characters. External sessions cannot expose private settings, modify published configuration or enter the owner's personal history.
- **REQ-5:** Paste-and-use inline/floating website widgets use a hosted player, explicit start, exact allowed origins, short-lived scoped capabilities and enforced quotas. No customer backend or browser secret is required.
- **REQ-6:** Paddle sandbox checkout, subscriptions, top-ups, payment-method changes, signed durable webhook processing and workspace usage accounting work with isolated test fixtures. No real charges or production entitlement.
- **REQ-7:** Verify roles, tenant/session isolation, quota races, revocation, browser/media lifecycle and payment event ordering. Preserve phase125 source/images/data, archive review evidence and verify restoration.

No new image, analysis, synthesis or conversation-model requests. Preserve renderer/package/voice/animation. The owner has no Paddle sandbox account yet: actual sandbox verification is an explicit external prerequisite, not a passing fixture claim. Production pricing, commercial refund terms, extended prepaid advances, public deployment and physical-device qualification remain outside this delivery.

Test-only catalog: Starter USD10/month,60 minutes,1 concurrent session; Team USD20/month,180 minutes,2 sessions; top-up USD5/30 minutes. Maximum session duration300 seconds. These are integration fixtures, not approved launch prices.

## Approved phase127 refinement

The console and character creation redesign supersedes the workspace-first presentation of REQ-1/2. Existing service and ownership boundaries remain unchanged.

- **REQ-8:** Personal Overview and Characters do not require a workspace or workspace role. Workspace destinations remain distinct, including explicit loading, empty, forbidden and failed states. Request a workspace only during the first website connection.
- **REQ-9:** Use a graphite `#15131B` sidebar in both themes. Console canvas/panels are `#F5F4F7`/`#FFFFFF` in light mode and `#302C38`/`#383340` in dark mode. Preserve 264px preferred, 240–320px resized and 72px collapsed navigation, independent scrolling, keyboard resizing and a mobile modal drawer. Keep 32px desktop headings, 15px body text, 44px targets and the unchanged conversation player.
- **REQ-10:** One shared brand signature preserves original SVG bytes: expanded attribution groups `by` with the 120px Colors Studio wordmark; collapsed/mobile attribution retains `by` with the original multicolor symbol. Dark navigation always uses the white-lettered original.
- **REQ-11:** Character creation has Portrait and Review and create steps. Validate existing JPEG/PNG/WebP, 5MB, 4MP and dimension limits locally, retain original bytes, require explicit submission and keep request idempotency. Show one submitted character's real status, actionable failures and Test/Connect when ready. Preserve in-memory unsent drafts, expired-portrait name fallbacks and a separate included Syntelle sample.
- **REQ-12:** Integrations show website connections and a dedicated empty state. Setup preserves selected character, asks for an initial workspace when needed, then character, website/appearance and review. Confirm a workspace-owned copy explicitly; distinguish multiple copies; separate Save draft from Publish integration. Preserve confirmed intermediate resources and refresh uncertain mutations without automatic replay.
- **REQ-13:** Account-scoped typed navigation intent carries only validated identifiers and an allowlisted destination. Sign-in returns to `/characters/new`; identity changes/cancellation clear relevant intentions and forms. Never persist image bytes or credentials in browser storage. Test navigation retains immediate suspension and explicit resume.
- **REQ-14:** Verify both themes/languages and responsive/keyboard flows using isolated fixtures. Freeze phase126, install a separate local web candidate and archive source/comparison evidence in PostgreSQL with byte-exact restoration. No provider calls, dependencies, APIs, schema changes or cloud deployment. Owner visual acceptance and actual Paddle/public-widget qualification remain open.

### Owner correction: phase128

The owner rejected the phase127 visual composition and authorized changing style guidance. REQ-9 now uses near-black navigation `#0D1016`, full-width canvas `#F2F3F7`/`#353740` and panels `#FFFFFF`/`#41444E`. A left-aligned gallery replaces the narrow centered strip; the included sample receives a full portrait card. Remove duplicated collection introductions and the dashed empty-state box. Existing character creation, connection, permissions and media behavior remain unchanged. Verify the reported 2545px dark layout before and after, including full-frame theme consistency, visible sidebar separation and no horizontal overflow; preserve phase127 as rollback and snapshot849 as archival parent.

### Owner screen review: phase129

REQ-9 now centers complete task-sized compositions within the continuous canvas. Use bounded operational pages, narrower whole-page setup/verification flows and collection widths matching occupied columns. Headers/actions must align with their body; illustration/copy must stay grouped. Validate mobile and ultrawide screens, both themes, populated/error states and short-window scrolling.

History is explicitly secondary: the owner selected a temporary right-side panel opened from the Test toolbar. Main navigation must not gain conversation rows when entering Test. Reuse existing history access, pagination, preview and deletion; opening or selecting history cannot start playback/capture. Preserve explicit continuation and stale-response/identity guards. The panel closes on Escape, backdrop, leaving Test or identity changes and restores focus on dismissal.
