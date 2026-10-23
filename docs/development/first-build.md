# First local build

Implement the authorized first milestone: upload an image in Vue, validate its content through Go and FastAPI, and display the result. Use the [design references](../design/design-reference.md) for composition and the [graphite and violet colors](../product/brand.md) for both themes. No paid APIs or cloud resources are required.

Status: implemented and verified locally on October 16, 2026. The Compose stack is running at [localhost:5173](http://localhost:5173).

## Scope

- `apps/web`: Vue 3, TypeScript, Vite, Pinia, npm, and plain CSS.
- `services/api`: Go 1.27.2 with `net/http`.
- `services/processor`: Python 3.12, FastAPI, Pydantic, Pillow, and uv.
- Docker Compose with Node 24.22.0; publish web and Go only on localhost and keep FastAPI internal.
- Validate one image up to 5,000,000 bytes. Determine JPEG, PNG, or WebP from actual content, regardless of filename or claimed MIME type, and return orientation-corrected dimensions.
- Discard server-side uploads after each request. Facial compatibility, character creation, animation, and conversation remain later milestones.

## Component map

| Component or module | Responsibility and contract |
| --- | --- |
| `App` | Compose the header and workspace; forward the header upload action to the workspace's file picker. |
| `AppHeader` | Show the Syntelle wordmark, original Colors Studio company logo, theme control, and upload action; emit `uploadImage`. |
| `ThemeToggle` and theme store | Read and change the shared light/dark preference, retained locally when storage is available. |
| `ImageValidationWorkspace` | Compose the portrait stage and adjacent setup/conversation panel; connect upload and result events to the request composable and expose `chooseImage` for the header action. |
| `ImageUpload` | Accept browsing or a dropped file; emit a selected `File` or selection error and expose the native file-picker action. |
| `ImagePreview` | Receive an optional validated preview URL and filename and display the image or an empty preview. |
| `ValidationResult` | Receive request status, metadata, and error; display accessible loading, success, and error feedback. |
| `ConversationPreview` | Show the mockup's composer in an explicitly unavailable, disabled state without accepting text or recording audio. |
| `useImageValidation` | Own the selected file, request, cancellation, result, and preview URL lifetime. |

## HTTP contract

`POST /api/v1/images/validate` accepts one multipart part named `image`. Go forwards it to `POST /internal/v1/images/validate` on FastAPI. Both services expose `GET /healthz`.

Success returns `format`, `mime_type`, `width`, `height`, and `size_bytes`. Errors return an `error` object with a stable `code` and a readable `message`. Distinguish oversized, unsupported, corrupted, invalid multipart, and temporarily unavailable processing. Do not expose internal connection errors.

## Required verification

Check all three accepted formats, the exact byte boundary, oversized files, corrupted content, and misleading filenames. Verify orientation correction, multipart handling, browser error states, and processor failure. Run frontend type checking and production build, Go tests, and Python tests. Exercise Chromium and WebKit at desktop and mobile sizes in both themes. Real-iPhone verification remains a launch requirement.

Record only observed results after implementation. The full MVP task list must not be marked complete for this smaller milestone.

## Observed verification

| Check | Result |
| --- | --- |
| Docker Compose build and startup | All three images built and services started; processor health check passed. |
| Published ports | Vue on `127.0.0.1:5173`, Go on `127.0.0.1:8080`; FastAPI has no published host port. |
| Frontend type checking and production build | Passed with Node 24.22.0, including the build inside the Linux container. |
| Go tests | Passed natively and in the Linux test image with Go 1.27.2. |
| Python tests | 23 passed natively and in the Linux test image; the container uses Python 3.12.15. |
| Browser and API suite against Compose | 36 passed: Chromium and WebKit, desktop and 390 px mobile layouts, light/dark themes, plus public API contract cases. |
| Actual processor failure and retry | Four browser checks passed with the Compose processor stopped. Requests returned the public 503 error, and retry controls sent another request. |
| Recovery | The processor was restarted, its health check passed, and a new public API upload returned the correct PNG metadata. |
| Image cases | JPEG, PNG, WebP, exact 5,000,000-byte acceptance, oversized rejection, corrupt content, disguised extensions/MIME, and orientation correction passed. Python tests cover all eight EXIF orientations. |
| Interaction and layout | Keyboard file selection, drag/drop, multiple-file rejection, loading, preview removal, cancellation of superseded requests, and retained theme choice passed. Both themes were visually inspected on desktop and mobile; the empty mobile layout keeps the upload action within the viewport. |
| Original references | Four supplied HTML files were preserved byte-for-byte. The rejected portrait URLs and protected character assets are absent from application source. |

The Python test client emits an upstream Starlette deprecation warning about its HTTPX fallback; this does not fail the suite. Playwright WebKit runs on Windows and does not replace verification on a real iPhone.

## Mockup alignment follow-up — October 17, 2026

Reworked the interface around the supplied desktop composition: compact 64 px header, 1:1.15 portrait and conversation panels of equal height, and a composer anchored at the bottom of the right panel. The working upload and validation flow occupies that panel while conversation is explicitly unavailable and its controls are disabled. The mobile layout uses natural page scrolling. The header upload action opens the same native file picker as the upload panel.

Added the owner's original Colors Studio wordmark and compact symbol. The logo sits directly on the header without an added background: the white-lettered wordmark is used on dark desktop headers, only the symbol on light desktop headers, and the symbol on mobile. SHA-256 checks confirmed that both application SVG files are byte-identical to the supplied originals. Syntelle retains its separate typographic product wordmark and jade/ink themes.

Verified the final revision with frontend type checking and production build, 40 passing browser/API checks against Compose, and four passing processor-failure/retry checks. The processor was restored and a new upload returned the expected PNG metadata. Visually inspected desktop and mobile screenshots in both themes. Additional Chromium and WebKit checks at 320, 600, 801, and 1024 px found no horizontal overflow and confirmed that the header upload action remained visible. No full MVP tasks were marked complete by this visual follow-up.

The subsequent logo-transparency correction passed frontend type checking and production build, plus eight existing branding/theme checks in Chromium and WebKit on desktop and mobile. Desktop screenshots confirmed that the added rectangle is gone in dark mode and that the supplied symbol and company name remain readable in light mode.

At the owner's request, the separate visible company-name label was subsequently removed. The symbol retains its accessible name through the image's alternative text.

## Neutral palette and desktop overflow correction

The owner replaced the jade palette with neutral graphite/warm-white surfaces and a soft violet accent. Current colors and contrast calculations are in [Brand and interface colors](../product/brand.md); the original mockups and prompts remain unchanged. The application uses semantic accent tokens rather than color-specific variable names.

The desktop document previously exceeded the viewport by one pixel because the header's 64 px inner area also had a 1 px border, while the main area subtracted only 64 px. The application shell now allocates the actual header and remaining viewport space with CSS Grid. The header includes its border within its declared height, and short desktop windows use the setup panel's internal scroll area. Mobile retains natural document scrolling.

Frontend type checking and production build passed. All 40 browser/API checks passed, including desktop document-overflow regression checks at 943 px and 600 px heights and access to the mobile footer. Chromium and WebKit both reported a 943 px document within a 943 px viewport at 1900 px width. Reviewed light/dark desktop screenshots and recalculated all 40 color contrast pairs from the CSS tokens successfully.

## Local setup choice

The initial Windows frontend workspace mount stalled during container creation. The working Compose setup runs source baked into its images, without host mounts or dependency volumes. Rebuild after editing source. This keeps the first run dependent only on Docker Desktop. Temporary native Go, Node, and Python servers used for verification were stopped after the container stack replaced them.

## Remaining milestones

Real-iPhone verification is deferred to launch. Facial compatibility, the owned or authorized semi-realistic sample character, generation, depth, animation, conversation, quotas, owner settings, and cloud deployment are not implemented by this slice. Their full MVP tasks remain open.
