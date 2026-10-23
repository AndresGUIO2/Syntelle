# Syntelle brand and interface colors

The owner selected Syntelle as the platform name and graphite with a soft violet accent as its palette. This replaces the earlier jade and ink direction. Both light and dark interface themes are required. Colors Studio is the company behind the product and retains its separate identity.

The colors below implement that direction. The final Syntelle logo is open; use a restrained typographic wordmark in mockups. Do not recolor or redesign the Colors Studio logos to stand in for the product identity.

## Color roles

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

Violet identifies primary actions, links, focus, and active controls. Keep large areas neutral: warm white in light mode and graphite in dark mode. Avoid tinting the entire interface with the accent. Use error and warning colors only for their respective states, accompanied by labels or icons. Decorative separators cannot replace essential control boundaries.

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

The [owner-supplied mockups](../design/design-reference.md) guide composition without requiring an identical implementation. Use an original semi-realistic illustrated sample character inspired by the broad treatment of the earlier protected reference. The generated photorealistic human in those mockups is not the selected character.

The original [Stitch prompt](../prompts/stitch-mockups.md) and supplied HTML mockups retain the earlier jade palette as historical reference material. The tokens above govern the current application. The first implementation milestone remains the separate local image-validation slice.


## Conversation workspace surfaces

The public workspaces use a centered 1,520 px shell, including responsive 24–40 px side gutters, with 32 px vertical padding and a 40/60 portrait/conversation split separated by 32 px. Align the header branding and actions to the same content edges. Conversation content has 28 px side padding and long character replies have a 72-character width preference. Panel corners remain 10–16 px according to the existing surface.

Mobile uses 16 px side gutters, 20 px vertical padding and 16 px between the portrait and independently scrolling conversation. The stage height is `clamp(160px, 26svh, 240px)`. The remote player preserves its video aspect ratio. The existing local renderer uses a shared face-relative CSS crop, capped on desktop at `min(88cqh, 135cqw, 800px)`; compact layouts retain their existing relative crop inside the smaller stage. These are presentation constraints, without changes to character geometry, source images or motion.

| Token | Light | Dark |
| --- | --- | --- |
| Raised surface | `#FFFFFF` | `#26232C` |
| Transcript surface | `#F6F5F8` | `#1E1C23` |
| User message | `#EDE7F9` | `#342B46` |
| Workspace accent wash | `#8C64D514` | `#9F7ADB12` |
| Portrait edge tint | `#9676CF33` | `#B296ED38` |

The 64 px microphone retains a light center (`#FFFFFF` to `#EEEAF5`) and dark icon (`#292332`) in both themes. The owner rejected the first thick-rim treatment. The current microphone uses a solid dark glyph, a separate 82 px fine orbit and a soft masked violet glow, with an essential control boundary and the existing violet keyboard focus outline. Conversation buttons share a consistent local SVG vocabulary and tactile neutral surfaces; the original top bar is preserved. The halo is a local visual meter, driven by captured PCM RMS at approximately 100 ms intervals. It does not drive speech recognition, interruption detection or character motion. Off and error states are static; reduced motion uses static rings and labels for every state.


Icon-only theme/customization/close controls use a quiet treatment: no painted border, raised box or permanent background. Keep 44 px targets and a visible keyboard focus outline, with subtle hover feedback on hover-capable devices. Text actions may retain a tinted surface. The microphone remains the visually prominent voice action.
