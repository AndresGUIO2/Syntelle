# Pricing page

Pricing is a standalone page at `/pricing`, reached through a normal header link.
The rejected modal implementation is preserved in `.tools/pricing-page-75/before/`
and earlier snapshots; it is removed from the current source and public allowlist.

`features/pricing/PricingPage.vue` presents the existing pricing-availability copy
without inventing plans or amounts. The shared header, language and theme remain
available. The privacy policy is linked from the account dialog. The product
wordmark and Back to Syntelle link return home. Account access from Pricing
navigates to the existing `/?auth` entry.

`composables/usePageNavigation.ts` handles the two local pages with the History API,
ordinary link destinations and normal modifier-key behavior. Back/Forward update
the view; navigation restores focus to the main content and resets its scroll.
Direct access and reload work through the existing Vite SPA fallback. Future
production hosting must retain the application fallback for `/pricing`.

Pricing does not mount the streaming workspace or create a renderer session.
Leaving the workspace disposes its session through the existing release path.
Returning home mounts the normal workspace with saved platform preferences.
No provider call, native-renderer change or dependency is added.

Eight focused Chromium/WebKit desktop/mobile checks cover direct access, reload,
Back/Forward, session creation/release, localized account access and the existing
platform-language journey. Type checking, scoped lint, production build and two
public-boundary checks pass. Desktop/mobile page screenshots were inspected.
Evidence and before/current source files use `.tools/pricing-page-75/` and a generic
PostgreSQL archive with byte-verified restoration. Only the local web container is
updated; public deployment and physical-device approval are not claimed.
