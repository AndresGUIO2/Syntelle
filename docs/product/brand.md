# Syntelle brand and interface colors

The platform is Syntelle. Colors Studio is the company behind it and retains its original, separate identity. Both light and dark interface themes are required.

## Current approved identity: night blue (phase136)

The owner selected the Azul noche preview and approved consistent use across management, navigation, conversation, embedded widgets and verification-email headers. This section supersedes every earlier dark palette and the earlier restriction on conversation palette changes. The approved light interface and light email bodies stay unchanged. Original Colors Studio SVGs and character/source-image bytes are protected.

| Dark role | Color |
| --- | --- |
| Canvas / panel | `#1B2B3E` / `#293F53` |
| Raised / subtle surface | `#344F66` / `#23364A` |
| Text / secondary text | `#F3F0EB` / `#BCC8D5` |
| Link and focus / hover | `#BDC6E9` / `#DBE3FB` |
| Primary button / hover | `#EEEAE2` / `#FFFFFF` |
| Button label / boundary | `#28262D` / `#EEEAE2` |
| Divider / essential boundary | `#4A6075` / `#9BAEC4` |
| Error / warning | `#FFB2AA` / `#F2C064` |

`apps/web/src/theme.css` owns these values and dark-only component roles. Navigation stays near-black `#0D1016`, with blue selected/hover/focus states. Use ivory primary actions throughout the dark interface, keeping accents separate from action fills. Derive photographic caption veils and decorative sheen from the shared palette without changing their opacity, source pixels or layout. Main text, secondary text, links and semantic text meet at least 4.5:1 on all four surfaces; essential boundaries meet 3:1. Retain the 1.2:1 dark canvas/navigation product floor.

The dependency-free external widget loader mirrors only the colors needed by its shadow host; browser tests compare those values with the iframe's canonical CSS. Explicit light/dark choices remain authoritative and system changes synchronize both sides through the existing validated message channel. Theme changes never start media. Verification-email headers use the night canvas, ivory wordmark and blue accent; the existing light body and code treatment remain intact.

The sections below preserve the earlier decisions and experiments for context. Their dark colors are historical, not implementation defaults.

The colors below implement that direction. The final Syntelle logo is open; use a restrained typographic wordmark in mockups. Do not recolor or redesign the Colors Studio logos to stand in for the product identity.

## Historical base color roles

| Role | Light theme | Dark theme |
| --- | --- | --- |
| Page background | `#FAFAF9` | `#141316` |
| Main surface | `#FFFFFF` | `#1C1B20` |
| Subtle surface | `#F3F2F5` | `#25232B` |
| Main text | `#24212B` | `#F3F0F7` |
| Secondary text | `#645E70` | `#BAB4C5` |
| Violet action and link | `#6D4BD0` | `#BBA3F5` |
| Violet action hover | `#5D3DB8` | `#CBB8FA` |
| Text on violet action | `#FFFFFF` | `#21172F` |
| Decorative separator | `#E2DFE7` | `#35313E` |
| Essential control boundary | `#8B8296` | `#857B94` |
| Error text and icon | `#B42318` | `#FF9D95` |
| Warning text and icon | `#8A4B08` | `#F2C064` |

The light interface retains its violet actions and accents. The current dark interface uses the night-blue roles above. Use error and warning colors only for their respective states, accompanied by labels or icons. Decorative separators cannot replace essential control boundaries.

## Contrast checks

Calculated 40 foreground and background pairs directly from the application CSS tokens using the WCAG relative luminance formula. Main text, secondary text, violet links, error text, warning text, and button labels meet at least 4.5:1 on their specified backgrounds. Essential control boundaries meet at least 3:1 against all three neutral surfaces. These targets follow W3C guidance for [text contrast](https://www.w3.org/WAI/WCAG22/Understanding/contrast-minimum.html) and [non-text contrast](https://www.w3.org/WAI/WCAG22/Understanding/non-text-contrast.html).

| Checked pair | Light ratio | Dark ratio |
| --- | --- | --- |
| Primary button label on violet | 5.89:1 | 7.87:1 |
| Primary button label on hover violet | 7.43:1 | 9.59:1 |
| Secondary text on main surface | 6.22:1 | 8.49:1 |

Ratios are rounded for display; checks used unrounded values. These are token calculations, not verification of a rendered interface. Recheck actual text, focus indicators, overlays, and all interactive states during implementation.

## Interface direction

Use a calm conversation workspace with a prominent portrait, readable text, layered neutral surfaces, restrained borders, and a limited number of actions. Start mockups with DM Sans and a sans-serif fallback. Prefer simple line icons and consistent spacing; avoid neon saturation, excessive cards, and dashboard metrics. Controlled violet gradients are approved for the workspace edge, portrait framing and microphone. Keep their area and opacity restrained, preserve the selected character background, and use opaque tinted panels instead of large blurred backdrops.

As implementation defaults, follow the system theme until a visitor chooses a theme, then retain that choice locally. Show a visible theme control and switch themes without animating every color. Respect reduced-motion preferences. These defaults require neither accounts nor server-side settings.

Use Syntelle as the product name, syntelle in internal project identifiers, and character for generated visual identities. Show the supplied Colors Studio logo as a secondary company signature beside the product wordmark. Do not substitute a text-only company attribution. Keep the logo background transparent. Show the original white-lettered wordmark directly on the dark header; in the light theme, use only the supplied multicolor symbol without an additional company-name label. At compact mobile widths, show the symbol in either theme. Preserve both SVG files unchanged, including their own colors and effects. Their brand colors do not replace Syntelle's graphite and violet tokens. Do not invent company claims, customers, testimonials, or credentials.

The application copies are `apps/web/public/brand/colors-studio.svg` and `apps/web/public/brand/colors-studio-symbol.svg`. They are byte-identical to the owner's `NewLogo.svg` and `colors-c-contorno.svg` in `E:/Carpetas/Documentos/01-MisProyectos/Colors/`.

### Console navigation signature and surfaces (phase128)

The console supersedes the earlier theme-dependent header attribution: its sidebar and mobile navigation stay near-black `#0D1016` in both themes. Use the original white-lettered Colors Studio wordmark at 120px width, preserving aspect ratio, grouped with `by` below Syntelle. Collapsed navigation and the mobile header retain `by` and the original multicolor symbol. The drawer shows the full signature. `BrandSignature.vue` owns these variants; no generated replacement, recoloring or SVG byte modification is permitted.

The owner rejected the phase127 composition and approved revising these guidelines. Console-only canvas/panel colors are now `#F2F3F7`/`#FFFFFF` in light mode and cool graphite `#353740`/`#41444E` in dark mode. Navigation uses light text independent of content theme. The full application frame owns the canvas color, including every gutter; never paint a narrow central dark strip over a light outer frame. Console content starts alongside navigation with responsive 32–80px desktop gutters and 56px top spacing. Sections may use up to 1,600px without centering a narrow block on an ultrawide screen. Mobile retains 24px top/16px side spacing. Use 34px desktop titles, 15px body text, 20px outline navigation icons and 44px minimum controls. The conversation player retains its own layout and palette.

Personal Overview/Characters precede grouped workspace destinations. History remains inside Test. Characters use substantial portrait cards, with the included sample explicitly identified and a useful first-portrait entry alongside it. Avoid tiny sample thumbnails under oversized dashed empty states, repeated collection introductions and duplicate primary actions. Prefer website rows and compact usage/billing details in their respective destinations. Maintain visible keyboard focus, readable status labels and separately scrollable navigation on short screens. Validate actual wide-screen composition at 2,545px as well as desktop/mobile; palette numbers alone do not establish visual quality. See [console redesign](../development/console-redesign.md) for ownership and review boundaries.

### Content distribution (phase129)

Center each complete page composition inside the available application canvas. Size pages for their actual content, rather than stretching all screens to the gallery maximum. Align each page heading and its primary action with the body. Use narrower whole-page compositions for creation/setup/verification and contextual states. Small collections should not leave their heading action hundreds of pixels beyond the occupied cards. Keep illustration and copy together, group related actions and reflow statistics compactly on mobile. Spare canvas remains the same themed surface; do not introduce contrasting gutters or fill it with invented information.

Navigation remains stable in Test. Conversation history is secondary, opened explicitly from its toolbar in a temporary right panel, full width on mobile. Keep modal close controls visible and return focus on dismissal. Long account lists and history scroll inside their own panels. Preserve the explicit action required to resume media.

### Console primary buttons (phase130)

The owner rejected the pale lavender primary button. Console and account primary actions now use a deeper, muted violet with white text and icons in both themes. Separate primary-button tokens from the accent used for links, focus and selected states. The conversation player keeps its existing palette.

| Primary button role | Light theme | Dark theme |
| --- | --- | --- |
| Background | `#7854BF` | `#7854BF` |
| Hover background | `#6846AA` | `#8561CC` |
| Label and icon | `#FFFFFF` | `#FFFFFF` |
| Boundary | `#7854BF` | `#A88BD0` |

White labels have 5.50:1 contrast at rest, 6.90:1 on the light hover and 4.59:1 on the dark hover. The dark boundary has at least 3.36:1 contrast against the console canvas and panel. Keep the existing bright accent for keyboard focus. Both shared button classes use these tokens inside console/account surfaces; outside them, the existing accent-based colors remain available. This color correction does not change button dimensions or interaction behavior. Visual acceptance remains owner review.

### Character gallery cards (phase131)

The included character uses a full-height photographic cover with a dark lower fade, a compact inclusion badge, a clear name/status row and a separated action footer. Preserve the supplied poster bytes; the fade is a display overlay. White labels stay on the dark footer in both themes. Test uses the shared violet primary action, and the website action remains secondary.

The first-character entry pairs a layered portrait-frame illustration with concise copy and a distinct action footer. Build the illustration from the existing line icons and themed surfaces, without generating a new person. Use 22px outer corners with an 8px inset around the 14px illustration panel, restrained shadows, a 500px minimum card height and the existing responsive gallery widths. Entire creation cards are single links with visible keyboard focus; do not nest interactive controls. Hover movement is limited to pointers that support hover and visitors who allow motion. Keep the original character preparation and connection behavior.

### Dark console palette correction (phase132)

The owner likes the light-theme button colors and rejected the flat gray appearance of the dark content area. Preserve all light-theme colors. This dark-only correction supersedes the phase128 console colors and phase130 dark button colors: use muted indigo/plum surfaces and a more vivid violet for primary actions. The near-black sidebar and conversation player retain their palettes.

| Dark console role | Color |
| --- | --- |
| Complete content canvas | `#37304B` |
| Main panel | `#433854` |
| Raised panel | `#514365` |
| Subtle surface | `#3B3250` |
| Main text | `#F7F3FC` |
| Secondary text | `#CBBFDB` |
| Link and focus accent | `#CAACFF` |
| Accent hover | `#DBC2FF` |
| Essential control boundary | `#B29DC5` |
| Primary action | `#8756E8` |
| Primary action hover | `#7950D1` |
| Primary action label/icon | `#FFFFFF` |
| Primary action boundary | `#B396EE` |

Keep the complete frame and content canvas consistent. Primary labels have 4.64:1 contrast at rest and 5.41:1 on hover. Rendered token checks cover body/link text at 4.5:1 or above and essential boundaries at 3:1 or above across all four console surfaces. The canvas/sidebar separation remains above the existing 1.5:1 product target. The light gallery is pixel-identical to the preserved phase131 view. These checks do not replace owner visual review.

### Neutral dark theme and ivory actions (phase133)

The owner rejected the phase132 violet button and overly colorful background. This correction supersedes its dark palette: use deeper neutral graphite for the canvas and layered charcoal panels, with ivory primary buttons and dark labels. Violet remains a small link, focus and illustration accent. Preserve the approved light theme.

| Dark console role | Color |
| --- | --- |
| Complete content canvas | `#25262C` |
| Main panel | `#303139` |
| Raised panel | `#3A3B45` |
| Subtle surface | `#2B2C34` |
| Main text | `#F3F0EB` |
| Secondary text | `#BFBCC8` |
| Link and focus accent | `#C4B3E2` |
| Accent hover | `#DFD2F3` |
| Essential control boundary | `#9996A8` |
| Primary action and boundary | `#EEEAE2` |
| Primary action hover | `#FFFFFF` |
| Primary action label/icon | `#28262D` |

Apply the same ivory treatment to sample-card actions, creation illustration controls and account buttons; the sample's light-theme border remains unchanged. Primary labels have 12.45:1 contrast at rest and 14.94:1 on hover. Body/link text still meets 4.5:1 and essential boundaries 3:1 across the four console surfaces.

The owner-requested deeper canvas intentionally narrows the luminance gap from the unchanged near-black navigation. Replace the previous dark-only 1.5:1 surface-separation product target with 1.2:1; the actual ratio is 1.26:1. This product target is not a WCAG text requirement. Keep the separate sidebar, visible divider and consistent full-frame canvas. Light-theme separation remains at least 1.5:1. The light gallery remains pixel-identical, and owner visual review is pending.
