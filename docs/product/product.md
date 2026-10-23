# Syntelle product and scope

Status: first MVP scope ready for review; prototype validation pending. Updated October 16, 2026.

Syntelle turns facial photographs into characters that speak and move in the browser. Its first release will be a public demo for tutoring and customer service experiences, with simple access and low operating costs.

## Product identity

Syntelle is the platform; Colors Studio is the company behind it. The selected palette is graphite and soft violet, with light and dark interface themes. The existing Colors Studio logos belong to the company identity and do not determine Syntelle's visual design. Exact colors and usage are defined in [Brand and interface colors](brand.md).

Use Syntelle publicly and syntelle for internal project identifiers. Use character for the generated visual identity in product copy, documentation, and internal naming. The final product logo remains to be selected; mockups can use a simple typographic wordmark. The [Stitch design prompt](../prompts/stitch-mockups.md) describes the full MVP experience for design exploration, not implemented functionality.

## Problem

Preparing a portrait for animation usually requires separating layers, reconstructing hidden areas, and preparing mouth and eye variants. Syntelle aims to automate that preparation and enable live conversations without continuously generating heavy video for the Standard tier.

## Users and experience

Anyone can open a link without registering and talk to a prepared sample character. Visitors can also upload a photograph, receive a compatibility check, and prepare their own character if the image is suitable. Both options support an editable task prompt and conversation through text or a microphone.

Voice conversation is open: visitors can interrupt the character while it speaks. The character responds with speech and lip synchronization. English and Spanish are supported, with priority given to English quality.

## MVP decisions

- A standalone Syntelle demo accessible from desktop and mobile browsers.
- iPhone Safari support is required at MVP launch, including the main character and conversation journeys.
- A sample character ready for conversation on entry, with an original semi-realistic illustrated treatment similar to the broad style of the protected reference, using owned or authorized material and the same conversation controls as custom characters. The photorealistic human generated in the UI mockups is rejected. Using the sample does not consume the creation quota; it shares the visitor's speech and listening quotas.
- Photographs of one person, facing forward, with a clearly visible face.
- Accept JPG/JPEG, PNG, and WebP uploads up to 5 MB per image.
- Reject incompatible images with a clear reason and replacement guidance.
- A Standard character with 2.5D parallax, visemes, and subtle motion.
- Depth maps based on Depth Anything Small, already used by the owner; the exact version and configuration remain to be verified.
- Image and variant generation based on the owner's existing Nano Banana workflow, automated through the Gemini API from the server.
- Facial region detection and automatic transparent mouth and eye crops aligned with the base photograph.
- An acceptable initial preparation wait of up to ten minutes, with visible progress. Actual duration must be measured; the prepared character can be reused in the same browser.
- Once the server accepts preparation, it continues if the visitor closes the tab. Returning in the same browser restores progress or downloads the completed character while temporary resources remain available under the 24-hour retention policy.
- Additional variants and revised transitions to address the robotic movement observed in the previous example. Visual tests will determine the final image count.
- An editable task prompt and a default prompt whose company information will be supplied later.
- Azure Speech selected for TTS with viseme events. Google remains the priority for the LLM and speech recognition; specific providers and models are still open.
- Provisional spoken response target: start audible speech within about three seconds after the visitor finishes speaking. Validate this in the prototype before treating it as a release commitment.
- Store the character and its configuration in the browser for reuse there.
- Automatically delete photographs and temporary resources from our server 24 hours after each file is created, including files from failed or abandoned jobs. Keep the character already saved in the browser. The public sample character package is an application asset, not a visitor's temporary file.
- Per-visitor limits every 24 hours: one character and independent character speech and listening quotas configurable by the owner. Initial defaults are three minutes of speech and six minutes of processed listening audio; these are not fixed product constants.
- An owner-only private panel to change speech and listening quotas without code changes or redeployment. Settings are stored on the server; the administrative authentication method remains open.
- Consume the creation quota only when the character is ready to use. Failed preparation releases the reservation; retries remain subject to limits and the global budget.
- When the configured speech quota is exhausted, end both voice and text conversation until quota renewal. Keep the character and configuration locally.
- If the listening quota is exhausted first, disable the microphone and allow text input to continue; spoken responses consume the remaining speech quota.
- For simplicity, the speech quota counts generated audio. Interruptions cancel pending generation without reversing consumption for audio already generated.
- A global demo budget, with its amount and exhaustion behavior still to be defined.
- Azure Container Apps selected for the backend, replacing Cloud Run, with scale-to-zero, low cost, and simple maintenance as priorities. Evaluate Express before selecting it for both services. No infrastructure has been provisioned.
- Vue selected for the web application and Go for the main API.
- A separate static frontend deployment on Cloudflare. Workers Static Assets on the free plan is recommended; Vue will not be served from the Go container.
- A separate Python/FastAPI image processor, also on Azure Container Apps with scale-to-zero. Optimize its startup, memory, inference, and cost.

Without visitor accounts, identification and limits are approximate. Protected administrative access is separate from public demo access. Local storage supports recovery while resources remain available in that browser; it does not provide recovery on another device.

## Later development

The product vision includes an engine that other applications can embed and a Pro tier with real-time generative facial animation. External integration, the Pro tier, and visitor accounts with server storage are outside the MVP.

Illustrations as input and automatic correction of rejected photographs are also outside the MVP.

## Protected visual reference

The owner supplied a copyrighted local character example to study its resource structure and limitations. It must not be copied, redistributed, or included in the public demo. Syntelle tests will use owned or authorized resources. Findings are recorded in [Character assets and animation reference](character-reference.md).

## Open decisions

- Syntelle's final product logo.
- Company information for the default prompt.
- Supporting Vue, Go, and FastAPI tools, the inference runtime, and measured resources for each Azure Container Apps service.
- Specific providers and models, supporting infrastructure, and estimated budget.
- Measurable photograph validation rules and accepted and rejected examples.
- Minimum animation quality, synchronization, and interruption targets, plus validation of the provisional three-second response target and agreed preparation wait.
- Remaining browser coverage and exact test devices and versions; iPhone Safari is required at launch.
- Private panel authentication.

Routine implementation defaults: use a rolling 24-hour quota window from first counted consumption, apply changed quota values to subsequent operations without resetting consumption, and allow one automatic retry for transient preparation failures within the global budget. Preserve completed work rather than regenerating it. These defaults keep the first implementation simple and can be revised after the prototype.

## Technical planning

The [draft technical plan](../../specs/001-syntelle-demo/plan.md), [provider and cost comparison](../../specs/001-syntelle-demo/research.md), and [tasks](../../specs/001-syntelle-demo/tasks.md) record proposals and open decisions. Vue will be hosted separately on Cloudflare; the Go API and independent Python/FastAPI processor will run on Azure Container Apps. Supporting tools, operational persistence, durable dispatch, and resource sizing remain to be finalized through measurements.
