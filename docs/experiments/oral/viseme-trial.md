# Local A-viseme trial

Current local dentition/composition state: [Completed selected dentition](completed-dentition-trial.md). The A-only notes below remain historical evidence.

This is an owner-operated preparation experiment, separate from the public upload demo. It uses the owner-supplied `E:/Carpetas/Documentos/01-MisProyectos/Colors/Syntelle.jpg` as an immutable reference. A development-only browser view now plays the cached A layer for inspection. It does not expose paid generation through a public endpoint or implement the complete character-preparation job system.

## Local setup

The processor's optional `preparation` extra provides MediaPipe, NumPy, HTTPX, and local environment-file loading. The normal FastAPI validation service does not install the heavy preparation extra or load the face model at startup.

Put the Gemini credential in `GEMINI_API_KEY` in the repository-root `.env`. The file is ignored; `.env.example` contains only empty configuration. Never use a `VITE_*` variable for credentials. The CLI reads the secret locally, sends it only as an API header, and sanitizes provider errors. Compose and the browser do not receive this key in this experiment.

From `services/processor`, inspect the reference without a paid API call:

```powershell
uv run --frozen --extra preparation python -m app.viseme_trial --input "E:/Carpetas/Documentos/01-MisProyectos/Colors/Syntelle.jpg" --output ../../.tools/viseme-a-review
```

Add `--generate` to make one Gemini request and compose the result. Keep the same output directory when adjusting the local alignment or mask: the saved provider image is reused without another request. A failed or uncertain request is not automatically repeated. Use a new output directory only for an intentional new experiment. Alternatively, `--variant "path/to/generated-image.png"` processes a manually generated JPEG, PNG, or WebP without contacting Gemini.

## Detection and composition

- MediaPipe `1.1.1`, IMAGE mode, CPU, detects up to two faces so that zero or multiple faces can be rejected before generation. One detector instance is reused for the base and variant.
- The official float16 Face Landmarker bundle, version `1`, is downloaded once and checked against SHA-256 `64184e229b263107bc2b804c6625db1341ff2bb731874b0bcc2fe6544e0bc9ff`. Its source URL is saved in the run record. [Google's Python guide](https://developers.google.com/edge/mediapipe/solutions/vision/face_landmarker/python) describes the image API.
- The selected reference produces one face, 478 landmarks and 52 expression coefficients. Mouth/eye contours and stable alignment anchors are visible in `detected-regions.png`. These expression coefficients are not audio-driven visemes.
- Every generation receives the exact bytes of the same input file. Its hash is recorded, and a changed base cannot silently reuse an existing run directory. The supplied A description fills the original general prompt; the original source file is not edited.
- Generated framing is checked, then stable eye, forehead, temple and nose-bridge anchors fit a similarity transform. Lips and jaw are excluded from alignment so mouth opening is preserved. Large alignment residuals, scaling or rotation are rejected.
- The mouth mask covers the union of base and aligned-variant lip contours, with face-relative margins and feathering. It stays below the nose and above the chin. A visibly closed result is rejected as an A viseme.
- The final frame is composed over the decoded original base. Verification requires **zero changed pixels wherever mask alpha is zero**. The cropped RGBA layer retains its exact canvas coordinates.

Current thresholds are prototype defaults verified on this reference and synthetic geometry cases, not calibrated acceptance criteria for arbitrary visitor portraits. Browser-worker detection, comprehensive pose/occlusion checks and public server-side preparation remain future work.

## Image formats and provider configuration

The trial defaults to `gemini-nano-banana-2.1`, 1K, portrait 3:4, with `store=false`, no previous interaction, no search tools and no automatic retries. The local client makes one REST POST. Google documents the [image-editing API](https://ai.google.dev/gemini-api/docs/image-generation) and [stateless interactions](https://ai.google.dev/gemini-api/docs/interactions-overview).

The live endpoint rejected a PNG response request with HTTP 400 and explicitly required `image/jpeg` for this model. The provider response is therefore retained as its actual JPEG format. That compression cannot be undone by conversion. Processing and final exports use PNG and lossless WebP to avoid further lossy encoding; no JPEG output is used as the composed animation frame.

| File | Purpose |
| --- | --- |
| `base.png`, `base.webp` | Lossless working copies of the decoded, orientation-corrected reference; their decoded pixels are identical. |
| `base-landmarks.json`, `detected-regions.png` | Detected geometry and an annotated inspection image. |
| `base-mouth-mask.png` | Preliminary automatic mouth region before generation. |
| `generated-a.jpg` | Unmodified provider response, cached before alignment. Other actual formats retain their correct extension. |
| `composed-a.png` | Full-size composition using only the generated mouth region. |
| `mouth-a.png`, `mouth-a.webp` | Cropped transparent overlay; WebP uses lossless encoding and preserves transparent RGB values. |
| `mouth-mask.png`, `mouth-comparison.png` | Full-size mask and a close comparison for visual review. |
| `run.json` | Base/model/prompt hashes, request state, usage when available, alignment metrics and overlay coordinates. |

The WebP decode must exactly match the PNG overlay's RGBA pixels. The whole portrait should never alternate between full Gemini-generated frames. Preparation artifacts stay in ignored `.tools/` directories. The development-only browser trial copies the approved sample's `base.webp` and `mouth-a.webp` into `apps/web/src/experiments/viseme-a/`. The morph experiment also adds an opaque mouth texture and geometry JSON; all experiment assets are excluded from the production bundle with the trial view. They are not visitor uploads.

Only the base/detection files exist before generation succeeds. The composed A image and mouth layers must not be represented as generated until a provider result has passed composition and visual review.

## Browser transition inspection

With local Compose running, open [the transition trial](http://localhost:5173/?trial=viseme-a). It is available only in Vite development mode. The normal workspace and production build keep their existing file-validation behavior.

- `VisemeTrial.vue` loads the two cached lossless images and owns inspection controls; it makes no API or paid generation requests.
- `useVisemePlayback.ts` owns the animation clock, pause, pose selection and cleanup. Playback starts only after pressing Play, pauses when the tab becomes hidden, and is disabled when reduced motion is requested. Rest, A and manual blend inspection remain available.
- `VisemeStage.vue` receives read-only images, blend amount and boundary visibility. It redraws the immutable base and composites the native-size mouth at (740, 998) into a canvas. Both the portrait and magnified view use the same coordinates; neither alternates complete generated portraits.

The default method is now Landmark morph, with a 120 ms transition and 500 ms hold. Compare Crossfade only under Method, Direct cut under Transition, and the deliberately slower 300 ms transition. Rest and A freeze the endpoints; dragging Mouth blend pauses playback at an intermediate amount. Show crop boundary draws the enclosing rectangle as an inspection aid, not the feathered alpha contour.

Browser pixel checks confirmed zero changes outside the crop rectangle between Rest and A, with unchanged canvas size and placement relative to its figure. The preparation check above establishes the stricter zero-change condition outside the actual alpha mask. Playback in all three modes, reduced motion, readable image-load failures and no horizontal overflow passed in Chromium and WebKit at desktop and mobile viewport sizes. Static browser screenshots show no obvious rectangular seam at the A endpoint, but a 50% blend visibly superimposes the two lip contours. Alpha crossfading is not geometric mouth interpolation: the direct cut changes abruptly, while a longer fade prolongs the double-lip artifact. The generated patch also retains its previously observed softness. Owner motion review and real-iPhone verification remain pending; this does not establish production lip-sync quality.

Initial alpha-only verification on October 17, 2026: frontend type checking and the production build passed. The full Playwright suite passed 52 checks, including 12 transition checks and the existing 40 browser/API checks. The production output contained neither the trial component nor its image assets. The copied WebP files have the same SHA-256 hashes as the verified preparation exports.

### Landmark morph comparison

The owner accepted the first browser playback and authorized a geometric transition experiment using the cached images. No new Gemini requests were made. The feature boundaries remain small: the view owns loading and controls, the playback composable owns the clock, `mouthMorph.ts` owns one shared WebGL patch, and the two stage components composite that patch onto the unchanged portrait.

The local exporter `services/processor/scripts/export_mouth_morph.py` reads the saved base and variant landmarks and applies the recorded alignment matrix. It exports 40 corresponding lip landmarks plus 8 stationary boundary anchors, with 86 triangles. Explicit outer/inner lip connections prevent triangulation across the wrong contours. The surrounding skin uses Delaunay triangles. Nearly coincident closed inner lips are regularized by at most 0.691 pixels to avoid inverted triangles. The exporter checks signed triangle area over the entire interpolation interval, including quadratic extrema; the smallest signed double area is 5.802 square pixels. Tests also sample 101 intermediate meshes and verify coverage of the crop.

Reproduce the export from the repository root with the optional preparation environment installed:

```powershell
services/processor/.venv/Scripts/python.exe services/processor/scripts/export_mouth_morph.py --run .tools/viseme-a-05 --output apps/web/src/experiments/viseme-a
```

The opaque lossless `mouth-texture.webp` preserves RGB from the existing layer, including pixels whose alpha would be discarded by browser image premultiplication. `mouth-mesh.json` contains aligned local coordinates and provenance hashes. The browser uploads the base crop, opaque variant and original alpha layer once. During interpolation it moves vertices, samples both textures in their respective source coordinates and blends their colors. The mask is sampled at fixed canvas coordinates with nearest filtering, so deformation cannot extend outside the original edit mask. The renderer is shared between full portrait and detail and requires no per-frame inference or new runtime library. Rest and A bypass interpolation and render their existing exact endpoints.

Static comparison at 50% shows a single opening and substantially less doubled lip contour than alpha-only blending. Some interior texture stretching and the original source-resolution softness remain. This is two-pose texture morphing, not recovered facial anatomy or validated speech animation. Owner motion acceptance and real-device performance remain pending. If WebGL initialization fails, the page explains that deformation is unavailable and leaves the alpha-only controls usable; context loss also switches back and pauses playback.

Current verification: 43 local processor tests passed, and 60 Playwright checks passed, including 20 transition checks across Chromium/WebKit desktop/mobile configurations. Browser checks confirm exact endpoints, opaque frames, and zero changed pixels outside the actual alpha mask at 1%, 25%, 50%, 75% and 99%. They also exercise an unavailable-WebGL fallback. Frontend type checking and the production build passed; the experiment remains excluded from the production bundle. Performance on a physical phone and recovery after actual GPU context loss have not been verified.

### Fixed mouth interior — October 19, 2026

Owner review identified artificial tooth movement: the initial morph stretched and blended the mouth interior along with the lips. The exporter now identifies the 18 cavity triangles explicitly. Their fragments sample the existing open-mouth texture at fixed canvas coordinates, while the interpolated inner lip contour controls visibility. They do not stretch the teeth or blend closed lip pixels into the cavity. A 0.8-pixel transition along the inner contour softens the interface with the moving lip texture. The original outer alpha mask and exact endpoint paths remain unchanged.

This is a fixed interior for the existing frontal A experiment, not a separate anatomical jaw or lower-teeth model. No new image generation, dependency or public feature was added. Browser regression checks compare four visible upper-tooth reference pixels at 25%, 50%, 75% and 99% opening, requiring exact equality. Existing checks continue to protect the endpoints and every pixel outside the edit mask. The cached texture is unchanged; only geometry metadata and rendering behavior changed. The development preview can run directly in Vite when Docker is stopped; its silent playback does not require the backend.

Verification for this correction: all 24 transition checks passed across Chromium/WebKit desktop/mobile configurations, all 43 local Python tests passed, and frontend type checking and the production build passed. The checks include upper-tooth pixel stability, unchanged endpoints and outside-mask pixels. Periodic playback assertions now poll at 50 ms to avoid repeatedly sampling the same phase of a 500 ms pose hold. The API-dependent browser suite was not rerun for this correction because only the standalone preview was running. The in-app preview was reloaded to replace the cached renderer and left playing for owner review.

## Verified checks

Local checks confirmed detection on the authorized 1792 × 2400 portrait, a 320 × 177 preliminary mouth region, and 41 passing processor tests. They cover alignment recovery, changed pose rejection, no/multiple face rejection, opening detection, exact preservation outside the mask, reconstruction from the cropped overlay, PNG/WebP RGBA equality, immutable provider inputs for all three accepted MIME types, correct provider-cache extensions and byte preservation, credential handling, and a single HTTP request on a server failure.

The default Linux processor test image passed 23 validation tests and skipped the optional preparation module. MediaPipe inference and preparation tests have been exercised on local Windows Python 3.12; Linux preparation dependencies are not yet validated for deployment. Real generation quality is recorded in the individual experiment and still requires visual review before a pose is accepted.

## First successful generation — October 17, 2026

After the owner enabled billing, `.tools/viseme-a-05` received one A-viseme image from one direct API request. The request took 12.318 seconds and returned an 896 × 1200 JPEG, aligned and composed onto the immutable 1792 × 2400 base. The cached provider image can be reused without another paid request.

| Observed check | Result |
| --- | --- |
| Detected base geometry | One face, 478 landmarks, 52 expression coefficients |
| Stable-anchor alignment error | 2.933 pixels RMSE on the base canvas |
| Mouth opening / base mouth width | Base 0.00283; aligned A 0.16763 |
| Transparent crop | 320 × 210 at canvas coordinate (740, 998) |
| Changed pixels outside mask | Zero; independently checked against exported files |
| Overlay reconstruction | Exact match to `composed-a.png` when alpha-composited onto `base.png` |
| PNG / lossless WebP | Identical decoded RGBA pixels; 94,273 / 59,020 bytes |
| Original source hash | Unchanged |

The assistant's static visual review found a natural visible opening without an obvious hard mask edge. The mouth is softer than the source because the generated image was upscaled by 2×. This is a usable first static experiment; owner acceptance, animation transitions and broader quality calibration remain pending. No additional request was made to improve resolution.

The provider reported 1,388 input tokens, 1,373 output tokens and 82 thought tokens (2,843 total), including 1,120 image output tokens. These are recorded usage figures, not a verified invoice or measured billing charge. The complete metrics and checks are in the local `run.json`.

## Earlier unsuccessful requests

The expanded multi-pose experiment, shared dentition, review decisions and current browser controls are documented in [viseme-batch.md](viseme-batch.md). This document retains the original A preparation history.

Before billing was enabled, the configured credential could list models but a direct generation request returned HTTP 429 with a zero-request Free Tier limit. This blocker was resolved for the successful request above. Failed request directories remain separate and must not be silently retried.

Earlier PNG-format requests were rejected with HTTP 400. An SDK-based request was interrupted after it stalled without returning an image. The implementation now uses one direct REST request with redirects and connection retries disabled, verified with a failing mock HTTP transport. The local records preserve these separate attempts; provider billing has not been measured.
