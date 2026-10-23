# Image preparation with Nano Banana

The owner-supplied prompts describe localized mouth or eyelid edits to a reference image. They are the starting point for Syntelle's preparation workflow. The original text is preserved unchanged in [Original prompts](../prompts/nano-banana-source.txt).

## Existing workflow rules

The general mouth prompt uses the image as an identity and composition reference. It requests preservation of framing, resolution, proportions, lighting, texture, color, head position, and other features. Expression changes only as needed to articulate the viseme, with very subtle cheek adjustments allowed.

Blink prompts restrict editing to the eyes and eyelids. They preserve the mouth, eyebrows, and expression and distinguish a normal blink from a sleepy look or emotional gesture.

The owner currently crops generated regions manually. For the MVP, Syntelle must generate these crops automatically from detected facial regions and landmarks.

The owner currently generates depth maps with Depth Anything Small. This variant is the starting point given the low-cost priority; its exact version and configuration remain to be verified. Do not assume V1 or V2.

## Gemini integration

The owner has generated variants manually through Gemini and has now configured a local API credential. A [local A-viseme trial](../experiments/oral/viseme-trial.md) has successfully generated and composed the first A mouth layer using MediaPipe detection, alignment, automatic mouth masks, and PNG/lossless-WebP overlays. Independent checks confirmed exact reconstruction and zero changed pixels outside the mask. Static review found a usable opening with some upscaling softness; motion quality and owner acceptance remain pending. This owner-operated CLI is not yet connected to the public upload workflow. It always sends the same approved reference image rather than chaining prior generated variants.

The subsequent [seven-pose batch](../experiments/oral/viseme-batch.md) reused the identical input bytes, bounded paid requests to one per group and saved exact prompts/settings. Reviewed teeth/tongue masks are character-specific preparation data. A's dentition is shared across poses without mesh stretching; soft tissue remains pose-specific. EI, O, U, MBP and CHSHR passed internal endpoint review, while FV and SZ are excluded from animated playback. Automatic oral-layer separation for arbitrary visitor photographs remains pending.

Requests pass through the Syntelle server. Configure the key as a server secret, for example `GEMINI_API_KEY`; do not include it in the browser, local character storage, repository, or logs. Google recommends server-side calls to keep the key confidential; see [Gemini API keys](https://ai.google.dev/gemini-api/docs/api-key).

Check image validity and available quotas before requesting generations. Finalize the global limit and cost per character when selecting the model, resolution, variant count, and retry policy. Failures must not trigger unlimited retries.

The [image generation and editing API](https://ai.google.dev/gemini-api/docs/image-generation) accepts instructions with reference images. Select the specific model for quality and cost using the supplied prompts; prior experience in the Gemini interface does not by itself determine the API model or configuration.

Local storage refers to the prepared character and its configuration. Preparation sends the photograph to the server and generation provider. Our server photographs and temporary resources are deleted 24 hours after each file is created, as agreed in [the specification](../../specs/001-syntelle-demo/spec.md).

## Variants described in the source text

| Group | Supplied description |
| --- | --- |
| A | Visible, natural vertical opening with little jaw descent. |
| EI | Slight opening and readable horizontal extension without becoming a smile. |
| O | Compact oval opening and rounded lips, distinct from U. |
| U | Narrow opening with rounded, slightly protruding lips. |
| MBP | Closed lips with light pressure and no visible teeth. |
| SZ | Narrow horizontal gap and closely spaced teeth, with little interior darkness. |
| FV | Upper teeth resting gently on the lower lip, without visible lower teeth. |
| CHSHR | Slightly rounded, narrowed lips for CH, SH, J, and R-like sounds according to the owner's grouping. |
| Half blink | Upper eyelids lowered roughly halfway; irises and pupils remain partly visible. |
| Full blink | Gently closed eyes and relaxed eyelids. |

These names are groups from the supplied workflow. Define and verify their mapping to the selected TTS events for English and Spanish.

## Differences from the local reference

The prompt set includes MBP, but the inspected image set has no independent layer for that group. The reference has an LDTN image whose prompt is missing from the supplied text. Resolve this when defining the generation inventory rather than assuming one set replaces the other.

## Proposed Syntelle preparation

1. Validate the input photograph.
2. Generate each variant from the same reference photograph to limit accumulated changes between edits.
3. Check identity, framing, position, and changes outside the intended edit region. The prompt states the intent; generated output still requires validation.
4. Align variants and automatically generate transparent mouth and eye masks and layers.
5. Prepare Depth Anything maps and preserve alignment with character images and layers.
6. Optimize and package resources for browser playback and local storage.

Automatic layer extraction is an agreed requirement. Finalize its algorithm, processing placement, and Depth Anything configuration during planning.

## Automatic mouth and eye crops

Facial landmark detection locates regions; creating an animation-ready layer also requires alignment, edited-area selection, and a transparency mask. The proposed process is:

1. Detect facial landmarks in the photograph and check correspondence in generated variants.
2. Correct small variant displacements using features stable for the specific edit, without undoing mouth opening or eyelid closure. Incompatible identity or pose changes fail validation.
3. Delimit each region using the base and variant. The mouth must include enough margin for lips and jaw changes; eye regions must cover eyelid changes. A fixed rectangle for every face is insufficient.
4. Create masks with an opaque interior where the base should be replaced and soft edges that blend into the skin. Mouth and eye masks must work simultaneously without interference.
5. Export transparent layers in a shared coordinate system with the base and depth maps. Compact crops must retain their placement coordinates on the canvas.
6. Compose each variant over the base and verify alignment, edges, color, and the absence of duplicated teeth or lips before delivering the character.

Preserve the base outside edited regions. Facial detection does not guarantee generated lighting or geometry consistency; verify these in the final composition. Aligning generated variants is preparation work and does not change the policy of rejecting incompatible input photographs.

MediaPipe Face Landmarker is integrated into the local Python preparation prototype and has detected 478 landmarks on the owner's approved character. Mouth masks and alignment are custom workflow logic built around those coordinates, not automatic detector outputs. See the [official documentation](https://developers.google.com/edge/mediapipe/solutions/vision/face_landmarker). Public acceptance rules still require calibration with a broader authorized test set.

## Facial geometry for transitions

MediaPipe Face Landmarker is used in the local geometry preparation experiment. Its model estimates 478 three-dimensional facial landmarks and 52 expression coefficients and can return transformation matrices. These outputs describe the face in the analyzed image. The prototype uses saved lip coordinates for development-only WebGL transitions between reviewed poses; see the [batch results](../experiments/oral/viseme-batch.md). This local comparison does not implement browser detection or production facial animation. See the [official guide](https://developers.google.com/edge/mediapipe/solutions/vision/face_landmarker).

Measure the base and generated variants, save landmarks and correspondences, and test a textured mesh that smoothly deforms lips and eyelids between poses. Variants supply mouth and eye appearance; geometry may improve transitions. Verify the combination, especially teeth, jaw opening, and layer boundaries.

During playback, audio and TTS visemes continue to control the mouth. The Syntelle engine must convert these signals into mesh movement and image selection. Detected expression coefficients are not a future animation stream generated from audio.

Run detection on images during preparation and reuse the results during conversation. Browser detection must not block interaction: the [web guide](https://developers.google.com/edge/mediapipe/solutions/vision/face_landmarker/web_js) states that detection calls are synchronous and suggests a separate worker to offload the UI thread.

## Depth maps

Depth Anything supplies depth for portrait parallax. Face Landmarker geometry provides localized facial references; align both outputs in the same image space and normalize them for their respective purposes.

The choice between a reused base map and additional per-pose maps remains open. With multiple maps, verify that their scales and changes do not cause jumps when mouth poses alternate. Do not assume facial detector depth values and Depth Anything values are directly interchangeable.

The [Depth Anything V2 documentation](https://github.com/DepthAnything/Depth-Anything-V2) describes relative depth estimation models. This technical reference does not confirm the owner uses V2; the specific version remains open.

## Expansion to reduce rigidity

Evaluate intermediate opening and blink states together with the transition technique. Current instructions favor discrete poses with subtle articulation; additional variants must preserve identity and support smooth state changes.

Estimate the final image count and cost after testing visual usefulness. The first local experiment requests one A pose with Nano Banana 2.1 at 1K; the production model, complete pose inventory and batch cost remain undecided.

## Resource usage

The current local [automatic dental-reference trial](../experiments/oral/viseme-batch.md#automatic-original-reference-selection--2026-10-19) ranks the eight cached original poses, proposes enamel masks and selects EI under the recorded gates. The browser exposes source/mask review. No further generation is used. This local heuristic and its internal inspection do not establish automatic semantic separation for arbitrary visitor uploads.

The owner supplied the prompt text for this project. Protected images from the external reference have not been copied. Tests and the demo will use owned or authorized photographs and variants.
