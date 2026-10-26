# Character video cost and 720-high candidate

The worker sends a premultiplied character color image beside an animated alpha mask. The browser supplies the selected background and presents both the existing 1440 × 960 stream and the new 1080 × 720 stream on a 720 × 960 canvas. The 720-high format contains two 540 × 720 halves; its internal body raster is also smaller. The 960-high path remains available. No character, speech, oral, eye, or foreground asset was regenerated.

## Measurement method

The offline `render-probe` replays the same cached character package and cached 1.5-second speech clip without a provider call. It can output an animated mask, a first-frame frozen mask, or color only. Frozen and color-only are diagnostic formats, not live product options. `review-cpu.mjs` records worker CPU usage and throttling, render stages, CUDA events, encoder call time, receiver FPS/frame gaps, WebRTC video bytes, audio packets and concealment, and a decoded-frame quality check. The encoder metric is the complete NVENC surface/encode call, not an isolated hardware engine timer; stages can overlap.

The exploratory matrix includes 1, 2, 4 and 8 sessions for FP64 at 960-high animated and 720-high animated/frozen/color-only, plus 1 and 8 sessions for 720-high FP32. Its initial eight-session 30-second window contained only one cached speech part, so it is **not** the sustained voice comparison. We repeated all five eight-session variants with 32 queued cached parts, filling the full 30-second measurement. Reports and representative decoded/composited frames are retained in `.tools/video-cost-96/`. The tracked `services/render-worker/scripts/video-cost/run-one.ps1` reproduces a run with `-SpeechMs 30000 -SpeechParts 32`; build the native worker's `build` Docker target as `syntelle-render-build:video-cost` first. Each run uses a separate 4-CPU, 2-GiB Docker container with an RTX 3090 and NVENC. The receiver uses eight local Chromium WebRTC connections.

The exploratory matrix below reports mean receiver FPS / total encoded video Mbps. It compares the mask formats at every requested session count, but short cached speech and receiver startup variance make the sustained eight-session table the capacity evidence.

| Sessions | 960 animated | 720 animated | 720 frozen | 720 color only |
| ---: | ---: | ---: | ---: | ---: |
| 1 | 25.96 / 2.19 | 26.14 / 1.78 | 28.95 / 1.64 | 29.93 / 2.40 |
| 2 | 28.34 / 4.34 | 29.36 / 3.55 | 29.42 / 3.31 | 28.27 / 4.78 |
| 4 | 29.02 / 8.79 | 29.11 / 7.21 | 29.19 / 6.66 | 29.31 / 9.68 |
| 8 | 28.90 / 14.83 | 29.50 / 11.74 | 29.71 / 10.95 | 29.60 / 19.57 |

### Sustained eight-session result

| Format | Receiver FPS mean / minimum | Worst p95 frame gap | Video kbps total | CPU cores / throttled seconds | Render / encode ms per frame | Audio packets lost / concealed samples |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| 960 animated FP64 | 26.81 / 26.21 | 66.6 ms | 15,241 | 3.73 / 8.50 | 17.80 / 12.80 | 0 / 210 |
| 720 animated FP64 | 29.62 / 28.92 | 49.9 ms | 12,850 | 2.46 / 0.12 | 8.64 / 5.16 | 0 / 0 |
| 720 animated FP32 | 29.80 / 29.52 | 49.9 ms | 12,813 | 2.12 / 0 | 7.05 / 4.78 | 0 / 0 |
| 720 frozen FP64 | 29.44 / 28.38 | 66.5 ms | 12,064 | 2.87 / 0.15 | 10.43 / 6.19 | 0 / 3,620 |
| 720 color only FP64 | 29.72 / 29.50 | 49.9 ms | 19,795 | 3.39 / 0.30 | 12.66 / 5.44 | 0 / 617 |

All eight receivers got about 1,500 audio packets during each 30-second run. These are single runs, and receiver scheduling varies; the color-only and frozen timings should not be read as exact kernel savings. The color-only stream unexpectedly encoded **more** bytes despite having half the packed width, so encoded size cannot be estimated from pixel count alone. Neither mask diagnostic produced a sustained render gain over the animated 720 stream. The mask stays animated, and a smaller mask representation is not justified by these measurements. A frozen contour also cannot follow head and hair motion; color only cannot place the character over the selected background.

In the selected 720-high FP32 run, each worker session captured and encoded 29.98–30.02 FPS. Chromium decoded 29.95–29.98 FPS and presented 29.52–29.92 FPS, with zero reported dropped frames and freezes. The p95 presented-frame gap was about 50 ms. This locates the remaining local cadence difference after encoding; it does not isolate browser scheduling, WebRTC jitter, or the product's WebGL compositor because this receiver probe displays the raw video track. Measuring the complete player is the next local performance check.

The container had a 2,048 MiB RAM limit. Its measured cgroup peaks were 1,323.5 MiB for 960-high FP64, 1,268.3 MiB for 720-high FP64, and 1,267.5 MiB for 720-high FP32: a 56.0 MiB (4.2%) reduction from the baseline to the selected candidate, leaving about 780 MiB below the limit. All three runs reported zero `memory.max` and OOM events. These figures are container RAM, including its charged cache; GPU VRAM was not captured in this benchmark.

At 960-high, CUDA body geometry averaged 5.41 ms/frame, composition 2.75 ms, and conversion 1.09 ms. CPU geometry took 1.92 ms and geometry upload 3.09 ms. CPU throttling reached 8.50 seconds in the 30-second interval. At 720-high FP64, the same stages were 3.48, 1.16, 0.45, 1.66 and 0.99 ms respectively, with near-zero throttling. Thus both pixel processing and CPU quota contribute; GPU utilization alone did not identify a mask bottleneck. The 720-high change reduced measured total render time by 51%, encoder-call time by 60%, total video bytes by 16%, and CPU consumption by 34% relative to the sustained baseline.

The first FP32 candidate limited single precision to CUDA body geometry; the other operations retained their preceding behavior. It reduced 720-high body time to 2.46 ms/frame and total render time by another 18% in that run. A source-bound 19-state comparison against FP64 gave a minimum 89.71 dB PSNR and maximum channel difference of one in the raw frames. The later body-kernel optimization below supersedes this geometry-only FP32 candidate. Decoded/composited static frames for the current and candidate formats were inspected; the moving hair edge has not received owner visual approval. The selected 720-high local launcher and Modal candidate use FP32; the existing 960-high local path retains FP64. Local `--gpu-precision fp64` remains the explicit rollback. The browser and worker can still use the 960-high stream.

## CUDA body-kernel follow-up

The body kernel was the largest CUDA stage in the sustained eight-stream run. It tests triangle coverage for each body pixel, interpolates texture coordinates, and samples the character/expression textures. The initial FP32 geometry path still ran texture sampling and expression inversion in FP64. On the local RTX 3090, moving only those body operations to FP32 lowered the measured kernel time substantially. The FP64 body kernel remains separate and selectable with `--gpu-precision fp64`; compose, patches, and conversion retain their preceding precision.

We also tested conservative triangle/tile rejection. It removed only about 2.8% of tile assignments and increased CPU geometry time and reduced receiver FPS, so that change was reverted. Precomputing expression activity once per frame lowered body time slightly but did not improve full-frame latency on its own; it remains part of the FP32 candidate because it preserves the same boolean result.

The offline probe used the same cached assets and 32 cached speech parts, eight streams for 30 seconds, 4 CPU cores and 2 GiB RAM. A Docker volume held an exact copy of the asset package so repeated runs did not depend on slow Windows bind-mount reads. Representative sustained runs with the same volume:

| 720-high format | CUDA body ms/frame | Total render ms/frame | Encoder call ms/frame | Encoded frames / 30 s | Worker CPU cores | Peak RAM |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| FP32 geometry, FP64 texture sampling | 2.19 | 5.20 | 3.63 | 7,205 | 1.67 | 1,427.6 MiB |
| FP32 geometry and texture sampling | 0.81 | 2.77 | 2.71 | 7,206 | 1.08 | 1,299.0 MiB |

Both transmitted about 60.7 MB of video in that window. The FP32 body candidate passed the probe's receiver FPS, quality, audio, release, and control gates, with speech receiver FPS between 29.71 and 29.95. One earlier FP32 body run fell to about 24 FPS while encoder calls rose to 6.44 ms and recorded 0.95 s of CPU throttling; its raw result is preserved. The subsequent candidate run and a following baseline run had no throttling and completed near 30 FPS. This variance means the good result does not establish a strict eight-stream capacity guarantee.

The 19 source-bound recorded states compared the full FP32 body against the FP64 body at 720-high: minimum PSNR 89.78 dB and maximum raw channel difference one. This is a pixel check, not moving-hair owner approval. The selected local image is `syntelle-render-native:kernel-fp32-fragment`, with the same 720-high transport and browser compositor. The measurements, including the rejected tile test and slow first FP32 body run, are retained in `.tools/kernel-cost-97/`. No provider call or cloud deployment was made.

The optimized image is active in the local worker as `syntelle-render-native:kernel-fp32-fragment`. The launcher selected `--video-height 720 --gpu-precision fp32` and a Docker volume containing the verified package; the API reported the worker available with zero sessions after the switch. The preceding image remains available locally as `syntelle-render-native:video-cost-candidate` for comparison.

## Cloud routing and remaining gates

Modal configuration now permits multiple T4 containers, with two sessions per container and a 720-high stream. The API reserves distinct durable allocation IDs, binds each session to one connected worker, routes its commands only to that worker, and closes only that worker's sessions on disconnect. It does not infer capacity from VRAM or from the number of allowed Modal containers. Terraform retains `SYNTELLE_RENDER_CAPACITY=2` as the cloud admission default; the existing `runtime` override can raise the total session budget after a T4 qualification without raising the two-session limit on each container. No Modal image was deployed and no T4 run was performed. A real two-stream T4 test is required before claiming cloud capacity; increase admission only if its FPS, audio and quality pass. The local eight-stream result is near 30 FPS but does not prove a strict sustained 30 FPS target on every receiver, nor physical-device/browser approval.
