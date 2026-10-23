# Native GPU rendering and Docker WSL2

Current follow-up: [GPU-resident media](gpu-resident-media.md) moves mouth, eyes and skin to GPU, removes normal image readback/conversion/upload and corrects Windows cadence using a raw media clock. The phase-66 results below remain historical evidence.

The local native worker can rasterize the existing character with CUDA and encode with NVENC. Both have independent CPU fallbacks. The private API, animation/media clock, cached character resources, public player and 30 fps target remain in place. This is a measured improvement, not complete rendering or player qualification.

## Current path

Go retains animation, source mouth/eye patches, projected vertices and ordered triangle bins. Two embedded CUDA kernels rasterize RGB-D geometry, evaluate the existing expression displacement, sample textures, depth-test opaque fragments, composite premultiplied fringes and produce the final frame. Geometry is binned into 16 x 16 pixel tiles. The original double-precision calculations, top-left coverage, filtering and byte quantization are retained; fast math and floating-point contraction are disabled.

Each scene uploads its subject and immutable companion textures once, reuses device/frame buffers and uploads mouth/eye regions only when their state changes. CUDA primary-context ownership is reference counted and initialization is serialized. Calls push/pop the context within each native call; no persistent Go thread affinity or additional media goroutine is required. Blocking GPU completion avoids busy CPU polling. Scene cleanup releases device buffers, modules and its context reference after the existing media loops exit.

`SYNTELLE_RENDERER=auto|gpu|cpu` and launcher `--renderer` select rendering. Automatic mode tries CUDA, uses CPU when unavailable and redraws the same current state on CPU after a device failure. Explicit `gpu` fails instead of concealing a qualification failure. A cancelled frame is not replayed. Stats and readiness report the selected renderer. Encoder selection remains independent: `SYNTELLE_VIDEO_ENCODER=auto|nvenc|openh264` / `--video-encoder`.

The current live contract remains `premultiplied-alpha-sbs/1`: one 1440 x 960 H.264 frame contains 720 x 960 RGB and alpha halves. The browser presents 720 x 960 and owns backgrounds/blur/PiP. Opaque 720 x 960 offline comparisons remain supported. NVENC uses baseline level 4.0 for packed output and 3.1 for the opaque comparator; the P5/low-latency settings and 2.5 Mbps cap remain unchanged.

The pipeline still reads RGBA from GPU to host, converts through CPU libyuv, then uploads I420 for NVENC. It is **not zero copy**. Encoding-call measurements include conversion/upload; they are not hardware-encoder execution time alone. Native GPU rasterization also does not move all mouth/eye preparation or geometry work off CPU.

## Docker migration

This workstation's Docker Hyper-V backend could not expose NVIDIA libraries. Docker Desktop now uses its WSL2 engine, kernel `6.6.87.2-microsoft-standard-WSL2`, and can access the RTX 3090 with Windows driver 610.88. The launcher requests `--gpus all` and `compute,video,utility` capabilities when rendering or encoding may use GPU. No privileged container or driver installation was needed.

All 15 original Hyper-V volumes were copied and verified for file hashes, Unix modes, owners and symlink targets. Every original image ID was retained. Existing WSL images/volumes were preserved; the colliding original `postgres:16-alpine` image is available as `syntelle-migrated-66/postgres:16-alpine`. Original Hyper-V storage and cold image/volume backups remain intact. Reports and reproduction scripts are in `.tools/docker-wsl-66/`; the private backup directory includes database contents and Docker settings and is intentionally excluded from the generic evidence snapshot.

For explicit local comparison, stop the owned worker and run the launcher with the existing private asset volume:

```powershell
node services/render-worker/scripts/start-local.mjs --assets-volume syntelle-native-efficiency-57-assets --renderer gpu --video-encoder nvenc --detach
```

Use `--renderer cpu --video-encoder openh264` for the CPU reference, or omit both options for automatic fallback. The standard image includes both paths. `syntelle-render-native:before-gpu-66` preserves the preceding installed worker. This change does not size deployment or change admission capacity.

## Dependencies and build

`services/render-worker/native.Dockerfile` compiles portable `sm_60` PTX with Clang 14.0.6 (`1:14.0.6-12`) and embeds it into the native Go executable. The runtime dynamically loads the host CUDA driver through the existing pinned nv-codec-headers interface. It contains no CUDA toolkit, compiler or browser. Existing Go/Pion/OpenH264/Opus/libyuv/FFmpeg pins remain; [gpu-libraries.lock.json](../../services/render-worker/native/gpu-libraries.lock.json) records the GPU compiler/API contract. `/usr/local/share/syntelle/gpu-build.txt` records compiler identity and source/PTX hashes.

```powershell
docker build --platform linux/amd64 -f services/render-worker/native.Dockerfile -t syntelle-render-native:local .
```

The measured local build supplied `--build-arg GPU_BUILD_IMAGE=syntelle-gpu-build:66` to reuse the verified Clang SDK. Other stages use the digest-pinned default Go SDK. This compiler-cache override and concrete image identities are retained in the evidence. The default kernel stage installs the same pinned Clang version.

## Bounded checks, October 22, 2026

Evidence is `.tools/gpu-render-66/`. All speech is cached and all tests make zero paid/provider calls. Preserve failed checks alongside successful results.

Raw frames: 19 recorded mouth, dental, hair/fringe, blinking, gaze, smile/brow, breathing, head and background states in each of the 720 and 1440 output formats are **byte identical** between CPU and CUDA. This includes alpha. The 720 CPU reference is also byte identical to the preceding frozen CPU implementation. Injected device failure after corrupting a real GPU output reproduces the complete current CPU frame. The blocking-wait build repeats this exact comparison successfully. The finite 168.13 dB values in JSON use an epsilon for zero error; report these results as zero byte differences, not a measured finite quality margin. Visual inspection of raw smile and decoded mouth/edges found no new renderer seams in these sampled frames; owner acceptance remains separate.

Same-condition Linux media comparison: one worker session, 4 CPU limit/affinity, 1 GiB memory, a separate 4-CPU/512 MiB Chromium receiver, internal Docker network, 4 seconds animated idle plus 8 seconds cached speech in three parts. The same frozen animation source and assets are used for every variant; `benchmark-source/` preserves that source separately from later concurrent animation changes in the workspace. Measurements below use isolated worker cgroup counters. Stage times are elapsed wall time, not stage CPU attribution.

| Speech measurement | CPU + OpenH264 | CUDA + OpenH264 | CUDA + NVENC |
| --- | ---: | ---: | ---: |
| Total worker CPU, cores | 0.899 | 0.549 | 0.596 |
| CPU reduction | baseline | 38.9% | 33.7% |
| Render call, ms/frame | 24.04 | 12.50 | 15.38 |
| Encode/conversion call, ms/frame | 2.13 | 2.52 | 6.00 |
| Speech peak cgroup memory, MiB | 122.43 | 219.88 | 302.00 |
| Lifecycle peak cgroup memory, MiB | 137.93 | 235.69 | 318.25 |
| Received fps | 28.97 | 29.35 | 28.98 |
| p95 frame gap, ms | 50.1 | 50.0 | 50.1 |

Idle CPU falls from 0.729 cores to 0.346 with CUDA/OpenH264 (52.5%) and 0.326 with CUDA/NVENC (55.3%). All three have no decoder drops/freezes, pass the cadence, interruption, pause/resume, transport-offset, reconnect and explicit-release checks, and leave zero active sessions. CUDA/NVENC's sampled transport offset is -3.5 ms. One CUDA/OpenH264 quality snapshot is 39.78 dB and misses the 40 dB gate; CUDA/NVENC's snapshot is 41.70 dB. The earlier busy-wait measurements are preserved under `before-blocking/` rather than overwritten.

Encoding qualification uses three fixed packed states, 30 frames per codec (180 decoded frames). Mean face PSNR is 37.39 dB OpenH264 and 39.51 dB NVENC; minima are 35.83 and 36.25 dB. Three early frames per state are 0.28-0.66 dB below the matched software frame. All frames decode, but this does **not** establish every-frame 40 dB or every-frame relative improvement. Raw renderer equivalence does not eliminate codec loss. No bitrate/preset change was made to hide that limitation.

The actual public Windows player is not fully qualified: the initial five-second idle check is 27.18 fps; the blocking-wait build is 26.26 fps, both with no decoder drops and p95 gaps of 66.7 ms. Background, mute, PiP and release checks pass. The default headless browser uses SwiftShader, but a separate full Chromium/D3D11 check confirms the RTX 3090 and still shows 27.53 fps for both received and presented video. This isolates the shortfall before background composition; it does not establish the cause. No packets are lost and decoding is brief. Private worker metrics show approximately 30 generated frames per media second. Do not erase these failed cadence checks or claim the full local player is fixed.

Focused native package race/vet checks, recorded-frame/fault tests, packed-codec tests and eight launcher tests pass. A build exposed an unrelated fixture mismatch after the concurrent expressive animation profile became the default; the historical adapter fixture now explicitly selects its original profile, consistent with the animation fixture tests. Production expression settings are preserved. Source and both build results are retained.

The original 50%-less-CPU-in-both-phases/no-memory-increase gate remains unmet: speech CPU improves by 33.7% with CUDA/NVENC and memory rises. The 30 fps target is an explicit worker limit, not the RTX's hardware ceiling. No 60 fps claim, sustained-load capacity, real-phone acceptance or complete zero-copy implementation is established. Remaining work is GPU-resident conversion/encoder interop and the independently observed Windows delivery/timing shortfall; neither is proven solved here.

A bounded two-second animated-mouth renderer CPU profile is retained in `cuda-render.pprof` and `cuda-render-profile.log`. Its opaque 720 comparator takes 10.24 ms/frame; it is not another live-media measurement. Approximately 39.7% cumulative CPU samples are in oral triangle shading and 38.3% flat samples are attributed to the native-call boundary; triangle bin construction is 8.7% cumulative. Native-call samples do not distinguish driver work, transfers or kernels, and cumulative shares overlap. This supports investigating the still-CPU mouth patches and native transfers before expanding the rewrite. It does not justify attributing all remaining CPU to Go or NVENC.

Complete selected source, recorded states/frames, clips, measurements, dependency/build metadata and sanitized migration evidence are archived through the generic PostgreSQL preparation archive with byte-verified restoration. Private database backups and credentials are excluded. The archive result is recorded in `.tools/gpu-render-66-archive.json`.
