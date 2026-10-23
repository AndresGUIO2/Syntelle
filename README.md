# Syntelle

Syntelle is a conversational character platform by Colors Studio. The public frontend is now a **backend-only video player** at [the local root](http://127.0.0.1:5173/). The existing [stream URL](http://127.0.0.1:5173/?view=stream) opens the same workspace.

Vue displays the latest microphone-only chat interface and decodes remote WebRTC video/audio. The private native Go worker animates and renders Syntelle; the Go API owns conversation, voice, accounts and stream coordination. FastAPI prepares character resources. No character renderer, geometry or full character textures are included in the public build, and backend failure never selects a browser renderer.

See [backend-only player](docs/development/backend-only-player.md) for the route/UI migration and verification, [native rendering](docs/development/native-rendering.md) for the worker, and [accounts and streaming](docs/development/accounts-and-streaming.md) for local service setup. Optional identity/email delivery still requires provider configuration. Public deployment, physical-device acceptance and arbitrary portrait preparation remain incomplete.

Historical client demos, oral/motion laboratories and source resources remain preserved for internal comparison. Their former query parameters no longer activate client rendering in the application. Client-demo export modes and the private Chromium render worker are retired; normal and stream-player builds both enforce the private-renderer boundary. Playwright/Chromium is used only for browser tests and offline media receivers. Native rendering comparison snapshots remain in the private evidence archive.

Start with the [documentation index](docs/README.md) and [repository structure](docs/development/repository-structure.md).

## Run locally

Preparation evidence is archived in [local PostgreSQL](docs/preparation/preparation-database.md), including original/generated image bytes, prompts, configuration, masks, failures and immutable snapshots. The database runs through the optional `preparation` Compose profile; its credentials remain in ignored local configuration. The image-file validation endpoint still discards visitor uploads and does not write them to this development archive.

The [automatic oral development candidates](docs/experiments/oral/automatic-oral-corpus.md) preserve the earlier three-character preparation evidence. Visual acceptance and held-out preparation remain incomplete; the public streaming workspace currently selects Syntelle only.

From PowerShell in the repository root:

```powershell
.\dev start
```

The command starts Docker Desktop if needed, PostgreSQL, processor, API, web and the private native worker. It waits for readiness and returns the terminal; all services continue in Docker. Repeating it reuses the connected worker and current images. Open [Syntelle](http://127.0.0.1:5173/). The API health check is at [localhost:8080/healthz](http://localhost:8080/healthz). Only Vue and Go are published, bound to localhost. FastAPI is internal to the Compose network. Existing private `.env` configuration and prepared native resources are required. Idle startup makes no model, speech or image request. See [local development commands](docs/development/local-dev.md) for prerequisites, cleanup and failure behavior.

Source files are baked into the images. After code/dependency changes, explicitly rebuild and start. Stop preserves database volumes; status makes no provider request:

```powershell
.\dev start --build
.\dev status
.\dev stop
```

`.\dev clean` removes recognized obsolete Syntelle test/build resources. Add `--cache` to trim unused shared Docker build cache to 2 GB. No command deletes volumes or unrelated project images. Builds use stable image tags, clean project-owned dangling images and trim unused build cache; ordinary startup does not rebuild existing images. Runtime logs rotate at 5 MB per file, two files per container.

## Image validation

The original image validator remains an API capability; its former upload page is retired from the public workspace. The browser and Go enforce a 5,000,000-byte file limit, inclusive. Pillow verifies actual JPEG, PNG, or WebP content, independent of extensions and claimed MIME types, decodes the image, and applies EXIF orientation before reporting dimensions. Pillow's decoder safety protections also reject exceptionally large pixel counts. A valid image file is not a facial compatibility result.

Uploads are discarded after validation. No visitor image or filename is saved to a server database or asset directory. The preview lives only in the current page; only the selected theme is retained locally. Uploaded-file temporary handles are closed on both success and failure.

`POST /api/v1/images/validate` accepts one multipart field named `image`. Success returns `format`, `mime_type`, `width`, `height`, and `size_bytes`. Errors have an `error` object with `code` and `message`; processor failures return 503 without exposing internal details.

## Checks

The API executable is `services/api/cmd/api`; HTTP, conversation and speech code live in focused internal packages. Rendering has its own Go module and image in `services/render-worker`, with shared wire/settings contracts in `packages/render-contract`. See [independent service boundaries](docs/development/renderer-service-boundary.md) and [Go service organization](docs/development/go-service.md). Historical sources and provider budgets remain unchanged.

Frontend type checking and production build:

```powershell
docker compose run --rm web npm run build
```

Go tests from the repository root:

```powershell
docker build --target build -f services/api/Dockerfile -t syntelle-api-checks .
docker run --rm syntelle-api-checks go test ./...
```

Python tests, using a disposable container with development dependencies:

```powershell
docker build --target test -t syntelle-processor-checks ./services/processor
docker run --rm syntelle-processor-checks
```

For browser tests, use Node 24.22.0 on the host while the stack is running. From `apps/web`:

```powershell
npm ci
npx playwright install chromium webkit
npm run test:e2e
```

The default browser suite verifies the backend player with isolated transport/provider fixtures in Chromium and WebKit, on desktop and mobile. Historical client-rendering suites remain as evidence and are excluded from this public-player suite. Windows WebKit transport is mocked because that build lacks WebRTC; a separate Chromium check covers real local idle video. No physical-device approval is implied.

## Project references

- [Natural head motion and Dragon HD local trial](docs/experiments/motion/natural-head-speech.md), including personal Azure setup, actual native-viseme verification and remaining performance limits.
- [Local blinking, motion and background candidate](docs/experiments/motion/motion-trial.md), including the three supplied backgrounds and background-only blur.
- [Product and scope](docs/product/product.md) and [MVP specification](specs/001-syntelle-demo/spec.md).
- [Technical plan](specs/001-syntelle-demo/plan.md) and [full MVP tasks](specs/001-syntelle-demo/tasks.md).
- [Brand colors](docs/product/brand.md), [design references](docs/design/design-reference.md), and [Stitch brief](docs/prompts/stitch-mockups.md).

The desired sample follows the broad visual treatment of the protected example and needs an owned or authorized image when facial processing begins. The protected image and its animation assets are not distributed here. The bundled DM Sans font's license is in [the font notice](apps/web/public/licenses/dm-sans-OFL.txt).
