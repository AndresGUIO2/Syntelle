# Native worker efficiency follow-up

The current worker is native-only. Its [standard build and start commands](../../services/render-worker/README.md) no longer build or select a Chromium renderer. Browser receiver tests and TypeScript fixture exporters remain. The image-selection and comparison details below record this experiment at the time; its historical sources and measurements remain preserved after retiring the unused browser worker.

Three changes preserve Syntelle's character, animation, speech, controls and 720 x 960 / 30 fps target: bounded speech preparation outside rendering, less repeated raster/composition work, and removal of redundant encoded-payload copies. The bounded Linux comparison shows lower peak memory and codec allocations. It does **not** establish a sustained CPU reduction.

This compares the existing native worker with its follow-up, not Chromium. The earlier browser replacement and its separate CPU/memory gains remain in [native rendering](native-rendering.md). Concurrency, cloud costs, deployment and battery measurements are outside this change. No asset generation, provider request, new dependency or codec-setting change is involved.

## Implementation

Each session owns one cancellable FIFO preparation lane. It normalizes PCM, analyzes acoustic cues, builds the viseme timeline and parses optional directed performance before a clip becomes playable. The temporary float PCM is released after analysis. Ready clips share immutable plans; mutable performance admission remains on the existing video owner and media clock. Audio waits for ready analysis, and completing receipt cannot prematurely end an unprepared turn.

The existing 32-part / 32 MiB PCM bounds include playing, ready, preparing and interrupted in-flight clips. Interruption invalidates old generations immediately; canceled work cannot publish a late plan. Its PCM reservation remains until the analysis unwinds. Session teardown cancels and joins the single preparation lane. There is no unbounded goroutine or preparation queue.

Rendering retains the original intermediate body raster, perspective/depth coverage, premultiplied materials and final bilinear sampling. It passes immutable frame state by pointer per triangle, hoists the exact source-aligned sampling condition to avoid unused perspective products, and bounds four compositor texel views once per pixel. No extra frame buffer or approximation is introduced.

The pinned mediadevices v0.10.1 OpenH264 reader already returns a Go-owned `C.GoBytes` result. Its Opus reader allocates its output in Go. Both release callbacks are no-ops, and later reads/close do not invalidate their arrays. Removing the second `append` copy preserves encoder interfaces, color metadata and delayed SPS/PPS ownership. Pion packetization and retransmission retain their own established ownership rules; this is not an end-to-end zero-copy claim. Exact pinned source bytes/hashes and real-codec ownership tests are retained in the evidence.

## Bounded Linux comparison

The fixed final short order is before / after / after / before: four seconds of animated idle and 3.5 seconds of cached speech per run. A separate planned pair replays twelve cached speech parts for 45 seconds. Both workers use the same readonly Linux asset volume, four-CPU quota/affinity and 1 GiB cap; the Chromium receiver uses four other logical processors and a 350 MiB cap. This task's builds and other benchmarks were stopped during measurement. Separate concurrent frontend/transport work was discovered afterward, so exclusive use of the entire host cannot be established; this further limits small timing claims.

Windows file sharing blocked new Docker bind mounts during this iteration. Both workers therefore receive the same cached assets through an immutable Linux-native volume and binaries/scripts through Docker API copies. This task did not restart user services or Docker. A later external Docker interruption occurred after all measurements; it did not invalidate their completed reports. The launcher has an explicit `--assets-volume` option for this existing-volume case; it never creates a volume or copies assets implicitly.

The sustained pair is the clearest resource result; peak memory includes marker, pause, interruption, reconnection and cleanup after speech:

| Sustained cached-speech metric | Previous native | Follow-up |
| --- | ---: | ---: |
| Total worker CPU cores | 0.830 | 0.827 |
| Complete peak memory | 158.09 MiB | 144.54 MiB |
| Received speech frames/s | 29.52 | 29.59 |
| p95 frame gap | 50.1 ms | 50.1 ms |
| Decoded drops / freezes | 0 / 0 | 0 / 0 |
| Decoded face PSNR | 40.58 dB | 40.62 dB |

Peak memory is **8.6% lower in this bounded pair**. The approximately 0.27% CPU difference is within measurement variation and does not support a sustained CPU-saving claim. The short ABBA set shows approximately 9% lower mean speech CPU, but the longer pair does not confirm that gain. One preceding-worker short run fails presentation fluency at 26.53 fps; it remains failed evidence. Both follow-up short runs pass, at 29.66 / 28.31 received speech fps and 50.1 ms p95 gaps. Do not describe the baseline failure or the short CPU numbers as a universal throughput improvement.

Stage counters measure elapsed wall time, **not stage CPU accounting**. Sustained rendering is 24.47 -> 24.32 ms/frame and video encoding (including RGB-to-I420 conversion) is 2.087 -> 2.105 ms/frame. Neither difference establishes a material stage speedup. Full-worker cgroup CPU includes rendering, encoding and transport; overlapping wall durations cannot be added to derive CPU.

The bounded 100-encode real-codec allocation comparison uses identical payloads and the same native encoder plus/minus the redundant copy:

| Per encode | Previous extra copy | Owned output |
| --- | ---: | ---: |
| H.264 allocated bytes | 16,725 | 10,210 |
| H.264 allocations | 3 | 2 |
| Opus allocated bytes | 3,261 | 3,168 |
| Opus allocations | 6 | 5 |

H.264 allocation volume falls about **39%** in that fixture. Remaining upstream allocations are included. Encoder timing is effectively unchanged; allocation savings are not equivalent to a 39% reduction in total worker memory or CPU.

## Quality, behavior and retained failures

All nineteen final Linux scene PNGs and thirty-eight mouth/eye component PNGs are byte-exact against the previous native worker. The browser-reference minimum unencoded face PSNR remains **54.37 dB**, exceeding the 45 dB requirement. The final sustained decoded minimum is **40.62 dB**, exceeding 40 dB. The same poses, dental ownership, hair edges, blink, gaze, expressions and head deformation remain present.

The 3,281 original TypeScript animation fixtures, focused adapter/media tests, scoped vet, Linux race checks and real native peer test pass. Deterministic regressions cover delayed preparation, audio-ready gating, FIFO completion, borrowed-envelope ownership, admission timing, cancellation, retained memory reservations and complete shutdown.

The final actual Vue/API/native flow passes with real H.264/Opus and zero decoded drops, browser errors or private-asset requests. Both injected signaling losses preserve the session and recover in 638 / 612 ms; fresh-video samples pass 28 fps. UI interruption mutes in 48 ms, and already-buffered receiver audio becomes silent in approximately 301 ms. Provider-fixture counts remain model 0 / cached speech 1 / recognition 0 after both reconnects; the fixture is not a provider call. Explicit stop leaves zero sessions. All three fixture containers exit normally without OOM and are removed; isolated PostgreSQL databases are independently verified absent, and credential scans are clean.

An initial candidate failed a short 28 fps gate and its first real-player reconnect measured 27.80 fps. It was not activated. Linux profiling found no reliable gain from incremental triangle coverage; that implementation was removed. Full affine interpolant stepping, a horizontal composition row cache and bounded/unrolled texture-sampler variants also failed their performance comparisons and were rejected. The retained final renderer is simpler. Failed reports, source/binaries, profiles and exact decisions remain alongside the final results; thresholds were not lowered.

The worker selector and existing API/player envelopes remain unchanged. The tested image is `syntelle-render-native:efficiency-57` (`a4be6cdc...`); the preceding native image and initial rejected follow-up remain explicitly available. All candidate quality/player checks pass. The active installation was not replaced: simultaneous backend-only player work added separate Docker-to-Windows media-port wiring after this image was built. Preserve that work and integrate its completed transport fix separately rather than overwrite it with this earlier image. An initial combined activation command was rejected by automatic review with only `blocked by policy`; no active worker was stopped. The temporary local image tag change was restored. Owner visual acceptance and physical-device/public-upload validation remain separate.

## Evidence and reproduction

Complete evidence lives in `.tools/native-efficiency-57/`, including the original/final run plans, all measurements, source snapshots, immutable inputs, native-library identities, pixel comparisons and codec ownership sources. `tested-api-source/` contains the exact 125 Go/module files verified for the tested image; the later working-tree transport additions are outside its qualification. The generic PostgreSQL archive and byte-verified restoration are recorded in `.tools/native-efficiency-57-archive.json`. Do not rerun a historical character or rendering finalizer against this change.

The frozen before binaries/image are under `before-linux/` and `syntelle-render-native:before-efficiency-57`. The final build uses `runtime/build-native.mjs`, canonical Go sources and the same pinned cached compiler/runtime, with no Windows bind mounts. `run-linux-review.py` runs one isolated comparison with an explicit fresh label; `final-run-plan.json` defines the fixed order and unchanged thresholds. `compare-results.py` summarizes all retained samples. Reproduction must use new output labels/directories rather than overwrite frozen evidence or make paid requests.
