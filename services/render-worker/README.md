# Private Syntelle render worker

The worker owns its independent Go module, commands and CPU/CUDA/media code in this directory. See [service boundaries and independent builds](../../docs/development/renderer-service-boundary.md). Both image builds use the repository root as context and copy only their service plus the shared contracts.

The selected cloud host is **Modal** (October 22, 2026), with zero warm GPU containers. This is a deployment decision, not a deployed adapter: the local launcher still requires a loopback API. Native CUDA/NVENC availability, authenticated remote control, session routing, ICE/STUN/TURN connectivity and scale-down need validation. See [renderer deployment requirements](../../specs/001-syntelle-demo/deployment-options.md#selected-renderer-host). Current local commands/assets remain unchanged; no GPU invocation is part of this documentation update.

This local service reuses the accepted Syntelle RGB-D engine. It publishes rendered character RGB and alpha together with native speech over WebRTC. The browser owns background composition and blur; all character rendering remains private. The public player receives no character source textures, meshes or animation implementation. See [browser backgrounds](../../docs/development/frontend-background.md) for the current media contract and bounded checks.

The native Go worker selects CUDA rendering when available, with CPU fallback, and publishes Pion H.264/Opus in Linux Docker behind the existing Go API and public WebRTC player. `build` builds the native image and `start` launches it. The former Chromium render worker, private web entry and browser-comparison launch commands are retired. There is no browser fallback.

The worker authenticates with `SYNTELLE_RENDER_WORKER_TOKEN`. The launcher reads only that token and the render API/capacity settings from the private root `.env`; the native container receives only the token. Provider credentials and the full environment file are never mounted or forwarded to it. The API independently enforces admission and ownership. Worker transport reconnection never resubmits model or synthesis commands.

## Native Linux worker

For the complete local stack, prefer `.\dev start` from the repository root. It reuses a healthy worker, starts the required services, waits for readiness and returns the terminal. `.\dev stop` stops the owned worker and services while preserving databases. See [local development commands](../../docs/development/local-dev.md). The lower-level launcher below remains available for explicit asset/image inputs; append `--detach` to return after renderer readiness rather than follow logs. A failed detached startup removes the newly created worker. Runtime logs rotate at 5 MB with two files.

The normal local image name is always `syntelle-render-native:local`, and the container name is always `syntelle-render-native-local`. Activate verified updates under that stable image tag and use the normal launcher without `--image`. Experiment labels may remain in archived build evidence; do not use them for the owner's running local container.

Prepare the existing source-bound resources offline from `services/processor`:

```powershell
uv run --frozen python -m app.workflows.rendering.export_native --output ../../.tools/native-render-56/package
```

This exports cached textures and geometry into private raw buffers. It makes no provider request and preserves the original source-file package identity. The launcher requires this prepared package; it never regenerates or substitutes assets.

With Node 24.22 and the existing Docker API service running, use the following commands from the repository root:

```powershell
npm --prefix services/render-worker run build
npm --prefix services/render-worker run start -- --dry-run
npm --prefix services/render-worker run start
```

The launcher discovers the existing Compose `api` container and shares its network namespace so the worker reaches the API at `http://127.0.0.1:8080`. It publishes no control port. The local launcher selects UDP 8081, which Compose publishes on `127.0.0.1` only for encrypted WebRTC media; see [backend-only player](../../docs/development/backend-only-player.md). Use `--local-media-port 0` explicitly for direct container-network ICE. `--api-container <name>` selects another existing local API container; `--assets <directory>` and `--image <name>` select explicit local inputs. `SYNTELLE_RENDER_API_URL` accepts only a loopback origin. The URL must use the same API port inside the selected container.

When a verified private package already exists at the root of a Docker volume, use `--assets-volume` to avoid Windows bind mounts:

```powershell
npm --prefix services/render-worker run start -- --assets-volume syntelle-native-efficiency-57-assets --dry-run
npm --prefix services/render-worker run start -- --assets-volume syntelle-native-efficiency-57-assets
```

This option is mutually exclusive with `--assets`. The launcher verifies the volume exists and mounts it read-only at the same private asset path, with automatic image-to-volume copying disabled. It never creates or populates a volume. The default asset-directory bind remains unchanged.

Capacity defaults to the API's current status. An explicit `--capacity` or `SYNTELLE_RENDER_CAPACITY` must match it; the launcher does not change admission or claim that the configured capacity was benchmarked. Stop an already connected worker explicitly before starting its replacement. `--dry-run` prints safe Docker arguments without exposing the token or starting a worker. Ctrl+C stops only the container created by that launcher.

The native container uses a non-root account, read-only filesystem and assets, four CPU limit and 1 GiB memory limit. Each session owns its scene, H.264/Opus encoders and WebRTC peer. Source PCM16 mono at 24 kHz drives both speech animation and media timestamps; pause emits silence while retaining the speech position, and interruption invalidates queued speech. Output is 720 × 960 at 30 fps with a 2.5 Mbps H.264 cap. The default is qualified by bounded one-session Linux measurements; public media routing, deployment sizing and 50-user capacity require separate measurements.

See [native rendering](../../docs/development/native-rendering.md) and the [efficiency follow-up](../../docs/development/native-efficiency.md) for retained appearance, media, CPU and player checks. API capacity remains separate from measured concurrency. Runtime identity and safe launcher verification are retained in `.tools/native-render-56/runtime/`.

`native.Dockerfile` pins Go 1.27.2 and Debian Bookworm by image digest. It copies only the C++/GCC runtime libraries and CA bundle from the pinned builder, avoiding mutable apt package resolution and the Azure Speech SDK. Go module checksums and explicit archive SHA-256 checks cover Pion WebRTC 4.2.25 and mediadevices 0.10.1's bundled OpenH264 2.1.1/Opus 1.3.1. Build versions, archive/runtime hashes and package versions are retained in `/usr/local/share/syntelle/native-build.txt`; the runtime contains no Go compiler or browser. The supported build target is Linux amd64.

For this workstation's existing public dependency/compiler cache, the equivalent explicit build is:

```powershell
docker build --platform linux/amd64 --build-arg GO_BUILD_IMAGE=syntelle-native-media-build:56 -f services/render-worker/native.Dockerfile -t syntelle-render-native:local .
```

The override and its concrete image identity are recorded with local verification evidence; clean builds use the digest-pinned Go SDK by default.

## Focused offline checks

```powershell
.tools/node-v24.22.0-win-x64/node.exe --test services/render-worker/tests/*.test.mjs
cd services/render-worker
go test ./internal/nativeanimation ./internal/nativeadapter ./internal/nativerender ./internal/nativemedia
```

Playwright/Chromium remains a test dependency for the public player and native media receiver; it is absent from the worker runtime. Native TypeScript fixture exporters remain for deterministic animation checks. The bounded Linux receiver plays authorized cached WAV audio, checks interruption/release and uses a private development-only marker/tone for transport offset. That offset is not a phonetic-accuracy measurement. Receiver audio is muted at the hardware output while an analyser verifies decoded samples. No provider request is made.

Historical browser measurements and exact implementation snapshots remain in `.tools/native-render-56/`, `.tools/native-efficiency-57/` and their verified PostgreSQL snapshots 518 and 521. These records document the comparison at that time; they are not current browser-worker startup instructions.

The removal record and preserved obsolete source/generated bundle are in `.tools/native-cleanup-58/`; its PostgreSQL snapshot and byte-verified restoration are recorded in `.tools/native-cleanup-58-archive.json`.

The [ISPC experiment](../../docs/development/cpu-simd-trial.md) remains optional through `simd.Dockerfile`. The owner subsequently selected libyuv as the standard converter; the [color validation](../../docs/development/libyuv-color-validation.md) preserves exact scalar tests, bounded BT.601 differences and about 3.5% less sustained worker CPU. Historical every-frame decoded-quality and peak-memory gates remain open; selection does not erase failed evidence.

The standard image includes [NVENC with automatic OpenH264 fallback](../../docs/development/nvenc-fallback.md) and [native CUDA rendering with CPU fallback](../../docs/development/native-gpu-rendering.md). Startup defaults to `--renderer auto --video-encoder auto` and checks Docker GPU access. Explicit `--renderer cpu|gpu` and `--video-encoder openh264|nvenc` retain comparison modes. Minimal pinned FFmpeg shared libraries and embedded PTX run in process; the runtime includes no encoder executable, compiler or CUDA toolkit. Docker on this workstation now exposes its RTX 3090 through WSL2. Raw GPU frames match CPU bytes, and bounded Linux speech uses 33.7% less total CPU with CUDA/NVENC at about 29 received fps. Memory rises and the actual Windows player's 26-27.5 fps checks still miss cadence; full qualification remains open.

The current [GPU-resident follow-up](../../docs/development/gpu-resident-media.md) supersedes the transfers and Windows cadence above: mouth, eye and skin pixels now execute in CUDA and render directly into NVENC-owned CUDA I420 surfaces. Explicit snapshots/software fallback retain readback. Raw-clock media pacing avoids this workstation's kernel clock-rate slewing; the bounded actual Windows player presents 30.01 fps. At matched real 30 fps, Linux worker speech CPU falls from 0.540 to 0.374 cores and peak memory from 308.70 to 244.20 MiB, with byte-identical controlled clips. `--gpu-pipeline host` keeps the preceding patches/transfers for comparison; default `device` selects the optimized path. Historical every-frame codec-quality and sustained/physical-device gates remain separate.


Immutable CUDA materials and sampling geometry are now shared by exact content within each worker. The [shared-assets follow-up](../../docs/development/shared-gpu-assets.md) records GPU/host memory, CPU, quality, cadence and cleanup evidence. Working patches, projected geometry and encoder surfaces stay private. Sharing is enabled by default; the offline comparison can set SYNTELLE_GPU_SHARING=off.
