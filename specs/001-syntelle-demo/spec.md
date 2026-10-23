# Syntelle public demo

Status: first functional MVP baseline ready for review. Technical choices and prototype-dependent acceptance thresholds remain to be validated. Updated October 16, 2026.

## Objective

A visitor can talk to a prepared sample character or turn a frontal photograph into a custom Standard character, assign it a task, and converse through text or voice in the browser without creating an account. Both characters use 2.5D parallax, visemes, and subtle motion. The experience prioritizes low cost and English, with Spanish support as well.

The overall product definition is in [Product and scope](../../docs/product/product.md).

## Main journey

1. The visitor opens the demo, sees a sample character ready for conversation, and learns the usage limits. They can use it or upload their own image.
2. To create a character, the visitor reviews the requirements and uploads a frontal photograph of one person with a visible face.
3. Syntelle checks whether it can work with that photograph. Using the sample skips upload, validation, and preparation and proceeds to step 6.
4. If the uploaded photograph is unsuitable, the demo explains why and how to replace it. The visitor can try another image.
5. If the photograph is suitable and quota is available, the demo prepares the custom character resources and displays progress.
6. The visitor defines the character's task through a prompt or uses the default.
7. They choose English or Spanish and start a conversation through text or microphone.
8. The LLM generates task-appropriate responses. Azure Speech produces speech and viseme events used by the engine for lip synchronization.
9. During voice conversation, the visitor can interrupt the character to speak.
10. The prepared character and its configuration are saved locally for reuse in the same browser.

## Functional requirements

| ID | Expected behavior |
| --- | --- |
| RF01 | Provide public demo access without visitor registration or sign-in. |
| RF02 | Accept frontal photographs of one person with a visible face, in JPG/JPEG, PNG, or WebP format, up to 5 MB per image. |
| RF03 | Validate the photograph before preparing the character and explain rejections. |
| RF04 | Automatically prepare character resources using the owner's Nano Banana prompts through the Gemini API, called from the Syntelle server. |
| RF05 | Display the Standard character with 2.5D parallax, viseme synchronization, and subtle motion. |
| RF06 | Support an editable task prompt and provide a default prompt. |
| RF07 | Support text and microphone conversation with spoken, animated responses. |
| RF08 | Maintain open voice conversation and allow the visitor to interrupt the character. |
| RF09 | Support English and Spanish, prioritizing English quality. |
| RF10 | Save the character and configuration locally and restore them from the same browser. |
| RF11 | Limit each visitor to one character per 24 hours and independent speech and listening quotas configurable by the owner through the private panel. Initial defaults are three minutes of character speech and six minutes of processed listening audio. Visitors cannot change their effective limits. Consume the creation quota only when the character is ready to use; failed preparation releases its reservation. |
| RF12 | Include a global spending limit whose amount will be defined after cost estimation. |
| RF13 | Work in desktop and mobile browsers, including required iPhone Safari support at MVP launch. |
| RF14 | Prepare additional variants and animation transitions to reduce the rigidity of the previous example while preserving facial identity, alignment, and lighting. Visual tests will determine the final set. |
| RF15 | Locate facial regions and automatically generate transparent mouth and eye crops aligned with the base image, without requiring manual visitor editing. |
| RF16 | Deploy with compute that can scale to zero when unused, retaining quotas and jobs in durable storage. |
| RF17 | Provide an owner-only private panel to inspect and change speech and listening quotas, with server-side authorization. Persist settings durably and apply them without code changes or redeployment, preserving accumulated consumption. |
| RF18 | Automatically delete photographs and temporary resources from our server 24 hours after each file is created, including files from failed or abandoned jobs. Keep the character already downloaded to the browser. |
| RF19 | Display a prepared original semi-realistic illustrated sample character using owned or authorized resources, following the broad treatment of the earlier protected reference rather than the photorealistic human in the UI mockups. Allow conversation without uploading or processing a photograph. Keep custom character creation available. Using the sample does not consume the creation quota and shares the visitor's speech and listening quotas. |
| RF20 | Continue server-accepted character preparation when the visitor closes the tab. Restore the same job's progress or completed result on return in the same browser while temporary resources remain available, without duplicate generation or creation quota consumption. |
| RF21 | Present the platform as Syntelle with its own product identity, distinct from the Colors Studio company brand, and support light and dark interface themes using the selected graphite and soft violet palette in [Brand and interface colors](../../docs/product/brand.md). The final product logo remains to be selected. |

## Initial acceptance criteria

These scenarios define behavior to verify. Thresholds and policies marked as open must be finalized before demo validation is considered complete.

### Access and photograph

- On opening the demo, visitors can start without credentials and see their usage limits.
- The sample character supports conversation without an image upload or preparation wait. Its resources are owned or authorized and prepared before deployment; selecting it invokes neither image generation nor Depth Anything.
- Uploading a custom image remains available. Sample and custom character conversations use the same speech and listening counters without resetting them when switching characters; loading the sample does not consume the creation quota.
- A photograph from the accepted example set can proceed to preparation when quota is available.
- Accept JPG/JPEG, PNG, and WebP uploads up to 5 MB per image. Check size in both the browser and server; reject larger files with a clear message before paid preparation.
- An image from the rejected example set does not proceed to preparation and displays an understandable reason and recommendation.
- Preparation failure is reported and releases the creation quota reservation. It does not consume the daily character allowance; another attempt remains subject to retry limits and the global budget.
- When the character becomes ready to use, exactly one creation unit is committed. Reloading or querying the same job does not consume it again.
- Initial preparation may take up to ten minutes, with visible progress. This is the agreed acceptable wait, not verified performance. The prepared character can be reused in the same browser.
- After the upload is accepted and a job is created, closing the tab or losing the browser connection does not cancel preparation. Returning with the same browser identity restores that job's progress or downloads its completed package while resources remain available. This does not extend the 24-hour file retention period.
- A valid photograph and its generated variants produce mouth and eye layers without requiring the visitor to draw masks or crop images.
- API generation starts after photograph validation and quota checks. The implementation default is one automatic retry for transient failures within the global budget, reusing completed work. Rejected photographs are not automatically retried. A final failure shows a clear error and allows the visitor to try again subject to quotas and budget.
- The Gemini key stays on the server and does not appear in browser responses, local storage, public files, or logs.

### Task and conversation

- Visitors can edit the task before conversation and try a tutoring or customer service scenario.
- The default prompt is separate from code so company information can be added later. Do not invent company information.
- Text input produces a spoken response with synchronized animation.
- With microphone permission, spoken input produces a spoken response with synchronized animation.
- Text conversation remains available if microphone permission is denied.
- Interrupting the character stops speech and speaking animation and processes the new turn without later playing discarded audio.
- Conversation journeys are verified in English and Spanish. The provisionally accepted response target is audible speech starting within about three seconds after the visitor finishes speaking; validate it in the prototype. Synchronization targets remain open.

### Visual quality

- Use the Syntelle product name consistently and keep Colors Studio identifiable as the company behind the platform.
- Provide light and dark interface themes using the selected graphite and soft violet colors, with readable text, controls, status indicators, and validation messages. Mockups may use a typographic Syntelle wordmark while the final product logo remains open.
- Demo resources are owned or authorized; the protected example supplied by the owner is limited to analysis.
- Variants preserve identity, framing, scale, and lighting, without visible mouth or jaw displacement when poses change.
- Test lip closures, sustained vowels, pauses, and fast speech with English and Spanish phrases.
- Transitions produce no double mouths, jitter, depth jumps, or visible layer boundaries.
- Automatically generated layers retain their coordinates relative to the base. Mouth and blink layers compose simultaneously without interference, preserving the base outside edited regions.
- A variant with incompatible identity, pose, or alignment fails validation before delivery as a character resource.
- Verify blinking and subtle motion during speech and idle states. Breathing must not deform the lips or make the face pulse.
- Evaluate improvement over the reported rigidity with owned or authorized resources; adding images does not replace reviewing the animation controller.

### Storage and limits

- After preparation, reloading restores the character and configuration while local resources remain available.
- Missing local data allows a new flow, subject to available quota.
- Automatically delete photographs and temporary resources from our server 24 hours after creation, including failed or abandoned jobs. Queries and page reloads do not extend retention. Temporary file deletion does not remove the local character or reset server quotas. Keep the public sample character package as an application asset.
- Inform visitors when a quota is exhausted and prevent further operations subject to that quota according to the agreed policies.
- The demo displays and enforces the owner's server-configured speech and listening quotas. Initial test defaults are 180 and 360 seconds respectively; editing browser data does not change effective limits.
- The speech quota counts generated audio duration, even when an interruption prevents full playback. Cancel pending generation. This implementation choice prioritizes the requested simplicity.
- Exhausting the configured speech quota ends the conversation: accept no new text or microphone turns until quota renewal. Close speech synthesis and recognition operations and retain the locally saved character.
- The listening counter accumulates processed audio per visitor within each 24-hour window up to the configured limit. Keeping the page open without sending audio to recognition does not consume listening time. Character speech has a separate counter.
- If listening is exhausted while speech remains, disable the microphone and close recognition. Visitors can continue sending text; spoken, animated responses consume the remaining configured speech allowance.
- Verify the main journey on desktop and mobile with the agreed browser matrix.
- iPhone Safari must pass launch verification for the sample character, custom image upload, preparation progress, local character reuse, text and microphone conversation, speech playback, and interruptions. Verify microphone permission handling and audio activation on a real iPhone.

### Private administration

- Only the owner can inspect and change speech and listening quotas in the private panel, initially three and six minutes respectively. Managing additional administrators is outside the MVP.
- Go checks authorization for every administrative read and write and validates values before saving. Unauthorized visitors cannot use those operations even through direct API calls.
- Settings survive scale-to-zero and backend restarts. Saving requires no redeployment and does not reset consumption counters.
- The demo obtains effective public limits from the server. As an implementation default, saved quota changes apply to subsequent operations, including active windows, without resetting accumulated consumption. Panel authentication remains a technical choice.

## Out of scope

- Visitor accounts and sign-in, cross-device synchronization, and a server-side character library. Protected administrative access is included in the MVP.
- An engine or SDK for embedding characters in other websites.
- A Pro tier and continuous facial video generation with models such as LivePortrait or Ditto.
- Illustrations as input, multiple people in one image, and complex poses.
- Automatic correction of rejected images.

## Open technical decisions

The owner selected Vue for the web, Go for the main API, and Python/FastAPI for a separate image processor. Cloudflare is confirmed for separate frontend hosting; Workers Static Assets on the free plan is recommended. Both backend services will run on Azure Container Apps with scale-to-zero, replacing the earlier Cloud Run choice. Express remains subject to compatibility checks for voice and image preparation. Optimize and measure processor startup, memory, inference, and cost; no performance results are available yet. Supporting tools and vision integration must respect these choices.

Parallax and visemes are agreed scope. The owner uses Depth Anything Small for depth maps, which will be the starting point for preparation; its exact version and configuration remain to be verified. The renderer and segmentation, reconstruction, and facial animation mechanisms will be finalized during technical planning.

MediaPipe Face Landmarker is the primary candidate for locating features, aligning variants, and obtaining geometry to support transitions. Test mesh deformation between generated poses while retaining visemes and audio as speech controls. Demonstrate a visual improvement before making that technique the final implementation.

The owner's Nano Banana workflow is the basis for image and variant generation. Original prompts, pose inventory, and proposed automatic extraction are documented in [Image preparation with Nano Banana](../../docs/product/character-preparation.md). The owner currently crops manually; automating this is part of the MVP. Mask generation, depth configuration, and processing placement will be finalized during planning.

The owner currently uses the Gemini interface and can supply an API key for automation. Generation calls will run on the server. Select the specific image model and configuration for quality and cost; integration and credentials are not configured. Choosing Gemini for images does not determine the LLM or recognition provider; Azure Speech is selected separately for TTS.

The local example inventory and observations are in [Character assets and animation reference](../../docs/product/character-reference.md). This reference does not authorize reuse of its images or code or expand MVP inputs beyond photographs.

Azure Speech is selected for Neural TTS and viseme events. The Go API on Azure Container Apps will request synthesis and forward audio and timing events to the browser; the FastAPI image processor does not participate in each spoken response. Verify specific voices and SDK integration in English and Spanish, including progressive delivery, interruptions, synchronization, mobile compatibility, and total cost. MediaPipe describes the observed face and does not replace TTS events. Google remains the priority for the LLM and recognition, with selection still open; choosing Azure synthesis does not automatically select Azure recognition.

The owner confirmed independent per-visitor speech and processed listening quotas configurable through a private panel every 24 hours, initially three and six minutes respectively. Exhausting speech also closes text chat and retains the local character. Exhausting listening first disables the microphone while allowing text until remaining speech is exhausted. For simplicity, count generated audio and cancel pending generation on interruption. The [technical plan](plan.md) enforces server limits. Use a rolling 24-hour window from first counted consumption and apply quota changes to subsequent operations without resetting counters as implementation defaults; panel authentication remains open.

The [provider and cost analysis](research.md) and [implementation tasks](tasks.md) complete this first specification.

## Implementation defaults and remaining validation

Core user behavior and deployment responsibilities are defined. The routine defaults below are selected for simplicity rather than additional owner-confirmed product decisions. Remaining technical choices and measurable quality thresholds belong in the prototype and implementation plan, without another round of routine setup questions.

| Topic | Baseline and remaining work |
| --- | --- |
| Images and devices | JPG/JPEG, PNG, and WebP uploads up to 5 MB; iPhone Safari required at launch. Use the plan's wider browser matrix as the test default. Calibrate resolution and facial validation thresholds with owned or authorized examples and record exact test devices and versions. |
| Preparation and failures | Up to ten minutes with progress; continue after tab closure and recover the same job in the same browser within resource retention. Default to one automatic retry for transient failures, preserving completed work and respecting budget. Final failure releases the creation reservation. |
| Demo limits | Owner-only panel, one character per 24 hours, and initial configurable speech/listening defaults of three/six minutes. Default to rolling 24-hour windows from first counted consumption; quota changes apply to subsequent operations without resetting counters. Select protected panel authentication during implementation. |
| Storage and deletion | Delete server photographs and temporary files after 24 hours. Keep downloaded characters locally and provide a local deletion option. If local data or browser identity is lost, explain that recovery is unavailable and offer the sample or a new creation subject to server quotas. |
| Success criteria | Provisionally target speech onset within about three seconds and accept preparation up to ten minutes. Validate the plan's proposed synchronization, interruption, rendering, and visual quality criteria in the prototype before release. These are not measured results. |

Creation quota is consumed only when the character is ready to use. Failed preparation releases the reservation without reversing provider costs already incurred against the global budget. Independent owner-configurable quotas, text continuation after listening exhaustion while speech remains, and closure of all conversation after speech exhaustion are confirmed. Three and six minutes are initial defaults, not immutable limits. The character remains locally saved.

## Implementation and validation decisions

Select the specific image model, reproducible Depth Anything Small version, inference engine, useful variants, and transition technique through the prototype. Azure Speech is already selected for TTS and visemes; specific voices and integration validation remain open. These technical choices and feasibility tests belong in [the plan](plan.md) and [tasks](tasks.md); the owner does not need to select every library to close product behavior. Public release requires validating quality, latency, visemes, and end-to-end cost.

Resolve durable quotas and administrative settings, durable job delivery, protected panel access, and cross-origin sessions before deployment, preserving account-free visitor access and the chosen services. Character storage remains local.

The owner deferred the global budget amount and company information. The draft can retain a configurable budget and generic prompt without inventing data; an explicit spending limit must be configured before enabling paid public usage. These deferred choices do not prevent closing functional scope.
