# Platform language

The header's **English / Español** selector controls the entire current public
interface and Syntelle's conversation language. This supersedes the phase-72
conversation-only selector, which has been removed from Customize.

The current phase-74 control replaces the rejected native dropdown with a quiet
globe/text trigger and a theme-aware menu with a violet selection check. It keeps
44px targets, localized accessible names, radio selection semantics, arrow/Home/End
navigation, Escape/focus restoration and outside-pointer/focus dismissal. Twelve
focused desktop/mobile Chromium/WebKit cases pass alongside type checking, scoped
lint and the public build. Light/dark and mobile menu screenshots were inspected.
Sources, the supplied rejection image and verification are preserved separately in
`.tools/language-control-74/` and its generic PostgreSQL archive. Only the local web
container changes; no provider calls, dependencies or conversation behavior change.

`stores/locale.ts` owns the shared Pinia preference, stored as `syntelle.locale`.
English is the fresh default. When no global preference exists, a preceding
Spanish stream preference migrates once. The selection updates the document's
`lang`, description, header, pricing, guest/signed-in account dialogs, saved-setup
and developer-access controls, customization, player controls, accessible labels,
conversation states and interface errors. Spanish messages live in `locales/es.ts`;
no localization library or service is added.

User-entered instructions, setup/key names, transcripts, replies and original
prompts remain verbatim. The immutable default instruction is still the original
English prompt. Known service errors use translated interface copy; an unknown
error receives a generic Spanish message while the original error state remains
available internally. Browser-owned permission dialogs, native form-validation
messages and external identity-provider pages follow their own language settings.

The same global choice supplies `en-US` or `es-CO` to the existing stream settings.
Ava remains the only selectable voice. Loading a saved setup retains the current
global language even if the setup contains a different historical language.
Changing language during capture or a reply stops the old stream and reconnects;
it never submits model, recognition or synthesis work itself. The pending-release
connection guard prevents an obsolete connection from becoming current when the
user changes language during startup. A late obsolete session is released.

The header adapts to two rows on narrow screens, retaining a visible global
selector and company attribution. The private renderer/material boundary allows
only the new selector, locale store and interface dictionary. Native rendering,
character resources, provider budgets, backend services and credentials are
unchanged.

Bounded verification covers the three new platform-language flows and four
existing control flows in Chromium/WebKit desktop/mobile. All 28 cases pass:
27 passed initially, and the remaining Chromium-mobile navigation check passed
when rerun alone after an initial `ERR_NO_BUFFER_SPACE` before page load. It was
not an application assertion failure. The first result and corrected recheck
remain evidence. Type checking, scoped lint, production build and two existing
public-boundary checks pass.
The boundary test's initial wrong-working-directory invocation and its corrected
run are retained separately; the former loaded no application Vite configuration.
The template/copy audit covers all fourteen active
translated components, with no untranslated interface text or missing literal
translation bindings. Desktop/mobile Spanish workspace/settings screenshots
were inspected. Verification uses transport/account fixtures, with zero paid
calls; it does not claim new live-provider or physical-device validation.

Evidence/source copies are in `.tools/platform-language-73/`. Its curated
`evidence/` directory uses the generic PostgreSQL archive with byte-verified
restoration recorded separately in `.tools/platform-language-73-archive.json`.
The local web container is rebuilt and replaced; earlier evidence remains intact.
