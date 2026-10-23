# Independent API and renderer modules

The renderer lives entirely in `services/render-worker`. It has its own Go module (`syntelle/render-worker`), dependency lock, commands, native libraries, tests and container build. The API module (`syntelle/api`) contains coordination, accounts/persistence, conversation/provider integration and signaling relay. It imports no rendering, Pion, codec or GPU implementation.

```text
packages/render-contract/         Dependency-free Go wire/settings contract
services/api/
  cmd/api/                        HTTP/WebSocket/provider coordination
  internal/                       Accounts, conversations, speech and streaming relay
  go.mod, go.sum, Dockerfile       Independent API artifact
services/render-worker/
  cmd/render-worker/              Private worker executable
  cmd/render-probe/               Bounded offline receiver/measurement host
  cmd/render-frame/               Recorded-state pixel exporter
  internal/nativeadapter/         Scene binding
  internal/nativeanimation/       Deterministic behavior
  internal/nativeclock/           Shared raw media clock
  internal/nativeframe/           Internal borrowed GPU surface descriptor
  internal/nativemedia/           Audio/video codecs and Pion transport
  internal/nativerender/          CPU/CUDA rendering and shared GPU inputs
  native/                         Pinned native build inputs and libraries
  go.mod, go.sum, native.Dockerfile Independent worker artifact
```

`packages/render-contract` contains the existing `Settings`, defaults/validation, control `Envelope`, signaling types and viseme shape. These declarations are moved without changing fields, JSON tags, defaults or validation rules. API compatibility aliases keep existing account/streaming call sites stable. Both services depend only on this small common module; neither depends on the other's Go module or internal packages. Local `replace` directives locate the common source inside this monorepo. No root Go workspace or runtime package manager is needed.

## Build and local deployment

Run image builds from the repository root:

```powershell
docker build -f services/api/Dockerfile -t syntelle-api .
docker build --platform linux/amd64 -f services/render-worker/native.Dockerfile -t syntelle-render-native:local .
```

Each Dockerfile has an explicit context allowlist: its own service plus `packages/render-contract`. API builds do not copy renderer code; worker builds do not copy API code. Private environment files, cached characters, `.tools`, provider credentials, frontend dependencies and the processor are excluded from both contexts. Native dependency/compiler versions and codec settings remain pinned as before. The worker build uses a temporary two-module workspace only for Go 1.27.2 dependency verification; directory-replaced contracts otherwise cause that compiler's verifier to request a nonexistent downloaded zip hash. The workspace is removed before the normal build.

The images can be released and restarted independently. Compose now uses the repository root as the API build context; the existing local worker launcher and root development commands use the corresponding worker build context. Only the API receives provider/database configuration. Only the worker receives character assets and its media/GPU runtime inputs. There is no new in-process renderer endpoint or additional frame/audio hop.

This extraction preserves the current network/access behavior. The owner-only local launcher still uses loopback control in the API container's network namespace, and WebRTC uses the existing media mapping. A future deployment into different remote service instances must explicitly configure private control addressing, TLS/access policy and ICE reachability; this source/build separation does not silently relax the existing loopback restriction or deploy cloud infrastructure.

From each Go module, run its tests separately:

```powershell
cd packages/render-contract
go test ./...
cd ../../services/api
go test ./...
cd ../render-worker
go test ./...
```

The worker's production GPU/codec tests run in the pinned Linux build, with the same explicit tags as before. Browser fixture generation now writes `services/render-worker/internal/nativeanimation/testdata/animation-fixtures.json`. Historical `.tools` evidence and source snapshots keep their original paths; they are not rewritten or regenerated.

## Verification

Evidence is `.tools/renderer-split-71/`. Relocation records map every original file to its new location. C/CUDA code, prepared fixtures, native library versions, shader compiler settings, shared-device resource lifetime, animation calculations and media cadence are unchanged. Tests cover exact JSON contracts, module boundaries, both independent image builds, focused API coordination and full worker native/race checks. Recorded frames and cached speech/player checks verify the resulting media; no paid provider request or image/asset regeneration is involved.

Complete sources, relocation records and bounded measurements use the generic PostgreSQL archive with verified restoration, recorded in `.tools/renderer-split-71-archive.json`.

## Bounded results

Both independent images build and their tests pass. The API's compiled dependency list contains no native renderer or Pion packages; the worker's contains no API/accounts, PostgreSQL, OAuth or Azure Speech provider packages. Sixteen launcher/development/boundary checks pass. Shared-contract JSON fixtures and API/worker race checks pass in Linux; portable Windows tests pass. Windows race mode requires the unavailable host C toolchain, so those attempted race commands are retained separately and Linux supplies the actual race checks.

All 76 CPU/GPU PNG outputs from 38 recorded states match the installed reference files exactly. C/CUDA source and fixture bytes are unchanged. The pixel renderer, projection, mouth/eyes, clock, zero-copy NVENC path and native library versions retain their preceding implementation.

One bounded serial comparison uses the same RTX 3090, four-CPU affinity/limit, 2 GiB memory limit, read-only assets, Windows Chromium/D3D11 receiver, one session, four seconds animated idle and eight seconds cached speech in three parts. Comparison reference is the previously installed GPU-sharing image; candidate is the independent worker module.

| Measurement | Installed reference | Independent worker |
| --- | ---: | ---: |
| Speech worker CPU, cores | 0.395 | 0.388 |
| Speech peak cgroup RAM, MiB | 575.94 | 565.88 |
| Received speech fps | 29.85 | 29.84 |
| p95 frame gap, ms | 39.00 | 38.88 |
| Render call, wall ms/frame | 10.72 | 10.53 |
| Encoder call, wall ms/frame | 5.71 | 5.52 |

This shows no material regression in the short comparison; the small CPU/RAM differences are not a new optimization claim. Both runs have zero dropped frames/freezes, sampled face quality above 40 dB, synchronized cached audio, pause/resume, interruption, reconnection and complete release. Stage durations are wall time rather than per-stage CPU attribution. The installed API plus independent worker also passes the actual public player/control check at 29.94 fps, p95 gap 33.4 ms and zero dropped frames/errors.

The current filesystem initially lacked the host-atlas lookup present in the installed reference image. Its omission raised peak RAM in the first candidate comparison. That initial result is preserved under `initial-missing-cache/`; the qualified atlas loader was recovered from the previously archived source without changing pixels or other later work. The new bounded allocation assertion rejects full host-atlas reconstruction on subsequent shared scenes. GPU sharing tests also cover simultaneous cold initialization, divergent conversations, changed content, write protection and final-owner release.

Installed artifacts are `syntelle-api:split-71` and `syntelle-render-native:split-71`, with the normal stable tags updated. Local network/auth/admission and prepared assets remain unchanged; no paid provider call or cloud deployment was made.
