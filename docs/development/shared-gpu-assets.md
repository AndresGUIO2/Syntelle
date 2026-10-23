# Shared immutable GPU assets

The native CUDA worker shares immutable device allocations between sessions in the same process and CUDA primary context. Exact SHA-256 content identity plus byte length and context identify an allocation; character names and mutable package paths do not. Equal assets from separate loaded packages can share; changed material bytes cannot reuse an older allocation.

Shared inputs are the expression field/material, original foreground, oral texture atlas, prepared smile/laugh atlases, eye atlas, original iris samples, eyelid curves and output sampling axes. The last two are immutable sampling geometry. Source mesh topology and rig data already belong to the shared immutable CPU `Assets`; projected triangles, changing mouth geometry and tile bins depend on each session's state and remain private.

Each session keeps its own working subject/skin patches, body/output buffers, background, CUDA stream, encoder surfaces, animation/audio clocks and media transport. No conversation frames, expression state or audio are shared. Rasterization, precision, sampling, colors, framing, codec settings and the 30 fps target are unchanged.

Acquisition publishes a device pointer only after its first upload completes. A native mutex deduplicates initialization and protects reference counts; rendering takes no cache lock. Shared slots reject upload/reallocation and writable slots reject sharing. Closing a session drains its stream before releasing its leases. The last lease frees the allocation immediately, without an idle cache, background goroutine or retention timer. Lost-context cleanup retires CPU cache entries even when CUDA frees cannot run. CPU rendering and automatic current-frame fallback retain their prior behavior.

Atlas identities are computed once per immutable CPU asset set. Later sessions look up the device allocation before constructing contiguous host data. A per-atlas cold-load mutex prevents simultaneous first sessions from rebuilding the same temporary host atlas. Only small content identities remain cached after upload, so this does not retain an additional host atlas.

Sharing is the default. The private environment variable `SYNTELLE_GPU_SHARING=off` retains per-session immutable uploads for an offline comparison using the same executable. It is not a player control or admission setting. Sharing is scoped to one worker process/device; separate containers do not share this cache.

## Bounded verification

Evidence lives in `.tools/gpu-sharing-68/`. Existing prepared character assets and cached speech are reused, with zero provider requests or asset generation. The fixture supports an explicit bounded `render-probe --sessions 1|2|4`; its default remains one. Production admission is unchanged.

- Four simultaneous renderers use divergent mouth, blink, gaze, expression and head states. All 76 corresponding frames match the private-allocation path byte for byte. Closing the first owner leaves surviving outputs unchanged; final close releases every shared lease/allocation.
- Content changes remain separate even when dimensions are equal. Immutable overwrites and sharing a writable subject buffer are rejected. Eight concurrent callers complete 48 acquire/release lifecycles without retained leases.
- The existing 38 CPU/CUDA reference frames still match byte for byte. Three controlled clips preserve the preceding byte-exact I420 and H.264 comparisons (180 encoded frames), including direct NVENC surface use and the untouched host RGBA sentinel. Current-frame CPU fallback passes.
- Focused race checks cover the shared lifecycle and rendering behavior. Go race detection does not instrument the CUDA driver or C mutex; actual concurrent device acquisition/rendering/release checks complement it.

Renderer device allocation accounting excludes CUDA context/module/driver overhead and NVENC surfaces. It must not be presented as whole-process VRAM or host RAM. Resource and player measurements are recorded separately below.

Initial harness attempts are preserved: the offline probe initially admitted only one session, and the existing control sequence explicitly supports a single session. These did not establish a four-session result; the bounded fixture and comparison invocation were corrected without changing production capacity.

## Renderer device memory

The same prepared character, fixed recorded states and packed 1440 x 960 output give the following allocation totals. Immutable inputs total 150,786,080 bytes (143.80 MiB), uploaded once for all active sessions. Small differences in private geometry capacity depend on the recorded state.

| Active sessions | Private immutable uploads, MiB | Shared immutable inputs, MiB |
| --- | ---: | ---: |
| 1 | 173.61 | 173.61 |
| 2 | 347.22 | 203.42 |
| 3 | 520.83 | 233.22 |
| 4 | 694.44 | 263.04 |

At four sessions this saves 431.40 MiB (62.1%) of renderer allocations. The shared immutable part stays at 11 allocations / 143.80 MiB; each extra session adds approximately 29.8 MiB of private renderer buffers rather than another 143.80 MiB of material data. Encoder/context overhead and host RAM remain separate.

## Final player/resource comparison

One Linux Docker/WSL2 worker on the RTX 3090, driver 610.88, four CPU limit/affinity `0-3`, 2 GiB memory limit, four independent Windows Chromium/D3D11 receivers, fixed read-only prepared assets, four seconds animated idle plus eight seconds of cached speech in three parts. Both variants use the same frozen-source executable and native libraries, GPU rendering/direct NVENC and the existing raw media clock. The only runtime difference is `SYNTELLE_GPU_SHARING=off|on`.

| Measurement, four sessions | Per-session immutable uploads | Shared inputs and atlas lookup |
| --- | ---: | ---: |
| Renderer device allocations, MiB | 694.44 | 263.04 |
| Speech total worker CPU, cores | 1.907 | 1.053 |
| Speech peak cgroup RAM, MiB | 1,315.49 | 745.61 |
| Idle total worker CPU, cores | 1.770 | 1.060 |
| Speech render call, wall ms/frame | 14.38 | 6.94 |
| Speech encode call, wall ms/frame | 7.50 | 4.12 |
| Received speech fps, range | 29.57-29.81 | 29.68-30.06 |
| Worst session p95 frame gap, ms | 38.84 | 38.99 |

The final bounded run measures 44.8% less speech CPU and 43.3% less peak cgroup RAM. These short lifecycle/steady playback windows include effects of initial allocation and garbage collection; they are not a sustained capacity result or a per-stage CPU attribution. All four receivers report zero dropped frames/freezes; sampled decoded face PSNR is at least 42.73 dB for the candidate. Every session is released. GPU/context/module/NVENC allocations beyond the renderer table are not included in its 62.1% device-memory reduction.

The installed public Windows player also passes at 29.84 fps, p95 gap 38.9 ms, maximum 61.1 ms, zero drops/errors. Background changes, sound, PiP/continued frames and explicit session release pass. The current prepared assets are preserved. Shared-resource release and current-frame CPU fallback pass the focused native/race tests.

Preserve earlier measurements. The 1 GiB run reached its cgroup limit and is not the final RAM comparison. Initial cache implementations still rebuilt contiguous host atlases, with variable CPU/RAM regressions despite reduced device memory and preserved cadence. CPU/allocation profiling identified the repeated atlas allocator; the final loader bypasses it on hits. A simultaneous workspace build restored an earlier Go loader during intermediate checks. Qualification now uses a frozen build context and verifies that its SDK/installed worker binary hashes match; earlier reports are not relabeled as final results. Profiles, exact source, frames, clips, measurements and failed harness checks are retained.

The default local worker is `syntelle-render-native:gpu-sharing-68`; earlier images, including `before-sharing-68-final`, remain available. The generic PostgreSQL snapshot and complete hash-verified restoration are recorded in `.tools/gpu-sharing-68-archive.json`. No paid calls, asset regeneration, codec/bitrate reduction, cloud deployment or production admission change is included. Historical every-frame decoded-quality and physical-device gates remain separate.
