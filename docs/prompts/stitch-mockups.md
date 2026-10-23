Design high-fidelity desktop and mobile mockups for Syntelle, a browser platform that turns a portrait into an expressive digital character people can converse with through text or a microphone. Initial use cases are tutoring and customer service. The experience should feel approachable, calm, and capable.

## Brand

Syntelle is the product. Colors Studio is the company behind it. Give the product its own identity; use a simple typographic Syntelle wordmark and a discreet "by Colors Studio" attribution. The final product logo will be developed separately. Do not invent a Colors Studio logo, reuse its corporate gradient as the product palette, or invent company claims.

Use "character," "portrait," and "conversation" in the interface. Use English UI copy with an English/Spanish conversation-language selector.

## Visual direction

Use jade and ink with light and dark themes. Keep the same layout and hierarchy in both themes.

| Role | Light | Dark |
| --- | --- | --- |
| Page | #F7FAF8 | #0D1714 |
| Surface | #FFFFFF | #15251E |
| Subtle surface | #EDF4EF | #1C3027 |
| Main text | #172B24 | #ECF5EF |
| Secondary text | #52665C | #A7BDB1 |
| Jade action/link | #0F766E | #5ED2AD |
| Action hover | #115E59 | #83DEC0 |
| Text on jade action | #FFFFFF | #0D1714 |
| Decorative divider | #D9E5DD | #2C493A |
| Essential control border | #71877C | #698275 |
| Error | #B42318 | #FF9D95 |
| Warning | #8A4B08 | #F2C064 |

Use readable sans-serif typography such as DM Sans, clear focus indicators, simple line icons, consistent modest corner radii, and restrained surface separation. Pair status colors with labels or icons. Keep controls comfortable on touch screens and respect reduced-motion preferences. Avoid neon glow, decorative gradients, glass panels, excessive cards, and analytics dashboards.

## Main conversation workspace

Open directly into a prepared sample character, ready to start without registration. Use an original fictional semi-realistic illustrated adult character, with softly rendered facial surfaces, slightly stylized proportions, a clear frontal face, professional styling, and a quiet background. Follow the broad visual treatment of the owner's earlier portrait reference without copying its identity or assets. Do not use the photorealistic human generated in the previous mockups. Make the portrait the dominant visual element, with a conversation panel beside it on desktop and a practical stacked layout on mobile. Treat the supplied HTML mockups as flexible composition references, not an exact implementation target.

Include:

- A clear "Start conversation" action and an "Upload your photo" alternative.
- A readable conversation transcript, text composer, microphone control, and a way to stop spoken output.
- English/Spanish language selection, a visible theme control, and a collapsed editable task prompt.
- Clear Ready, Listening, Thinking, and Speaking states.
- Discreet separate speech and listening allowances. Illustrative defaults are three minutes of character speech and six minutes of processed listening per 24 hours; the owner can change them.
- A small indication that the prepared character and configuration are saved in this browser.

Use a generic helpful-assistant task prompt. Do not invent company information. Keep the first view focused on starting a conversation rather than a marketing landing page or setup wizard.

## Upload and preparation

Design an upload panel with file browsing and desktop drag-and-drop. Accept JPEG, PNG, and WebP up to 5 MB (5,000,000 bytes). Explain that the image must show one person facing forward with a clearly visible face. Include a preview, validation progress, a successful result, and concise errors for an oversized, unsupported, corrupted, or unsuitable image, with a replacement action.

Show preparation progress after an image is accepted. Preparation may take up to ten minutes. Explain that processing continues after the tab closes and the visitor can return in the same browser while temporary resources remain available. Include preparation failure and ready-to-use states.

Explain briefly that uploaded photos and temporary files are deleted from our servers after 24 hours, while the prepared character remains saved in this browser. The sample is ready immediately and does not use the custom creation allowance. The initial custom allowance is one completed character per 24 hours.

## Conversation exceptions

Include compact state variants for:

- Listening exhausted: disable the microphone while keeping text available if speech allowance remains.
- Speech exhausted: disable both text and microphone conversation and show when access returns.
- Microphone permission denied: explain how to continue with text.
- Connection failure: provide a clear recovery action.

Allow people to interrupt spoken output. Switching between sample and custom characters does not reset allowances. Keep these states understandable without technical provider names or backend details.

## Private owner settings

Design a separate owner-only settings screen with two editable limits: speech minutes and listening minutes per 24 hours, initially 3 and 6. Include Save, saving, success, and error feedback. This screen is separate from public visitor access. Do not add account management, analytics, model selection, or unrelated administration.

## Deliverables

Provide the main conversation workspace at approximately 1440 px desktop and 390 px mobile in both light and dark themes. Include supporting upload, preparation, exception, and owner-settings views or component variants. Keep components, hierarchy, copy, and colors consistent across views. These are mockups of the full MVP, not a claim that the features are already implemented.
