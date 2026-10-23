# ISPC and libyuv CPU trial

The subsequent [libyuv color validation](libyuv-color-validation.md) qualifies a separate bounded BT.601 conversion contract and measures sustained libyuv savings. The initial failures below remain historical evidence; libyuv remains optional while expanded decoded-quality and peak-memory gates are open.

October 22, 2026. Both integrations are implemented as explicit Linux experiments. Neither is selected by the standard build or installed into the running worker. The trial does not establish a consistent total-worker CPU or memory improvement, and failed attempts remain preserved.

## Implementation

Go continues to own sessions, animation, frame buffers, H.264/Opus and Pion. Build tag `ispc` replaces only the final 720 x 960 bilinear resampling/composition pass. Packed RGBA loads reduce gathers; double precision, interpolation order, existing sampling axes, premultiplied transparency and rounding remain unchanged. There is no intermediate image copy. Native calls process 32-row bands with cancellation checks at the existing boundaries. Unaligned destinations and byte-stride rows use the scalar path. Native code retains no Go pointers.

This does not vectorize triangle rasterization, source-texture sampling inside triangles, oral geometry or expression deformation. It is a batch-friendly first integration, avoiding a native call for every sampled pixel.

Build tag `libyuv` replaces RGBA-to-I420 conversion with one `ABGRToI420` call over the existing source and encoder planes. The ABGR name describes libyuv's little-endian packed word; its memory byte order matches Go RGBA. Standard libyuv BT.601 studio-range conversion has different coefficient/averaging rounding from the existing implementation. Exact color tests fail; they were not weakened. Red Y changes from 81 to 82, red Cr from 240 to 239, and the tested skin Cr from 148 to 147. This is a compatibility failure, not evidence that a viewer would necessarily perceive those changes.

The [dependency lock](../../services/render-worker/native/simd-libraries.lock.json) pins ISPC 1.32.0 by release SHA-256 and libyuv by commit. ISPC uses SSE2/AVX2 CPU dispatch, `-O2`, disabled FMA contraction and no fast math. libyuv is a static Release build without JPEG. The experimental image includes its license. Normal `native.Dockerfile`, launcher defaults, assets, API and public player are unchanged.

## Measurements

Isolated same-process microbenchmarks ran twice for 100 iterations on Linux amd64, Ryzen 7 9700X. Both implementations allocate zero bytes per operation:

| Operation | Existing | Candidate | Interpretation |
| --- | ---: | ---: | --- |
| Final composition fixture | 5.73-6.32 ms | ISPC 2.95-2.97 ms | About half the time for this synthetic pass |
| RGBA-to-I420 fixture | 0.99-1.01 ms | libyuv 0.0748-0.0756 ms | About 13 times faster; different rounding |

The initial ISPC byte-gather kernel was slower (6.88-6.91 ms versus 5.60-5.63 ms). Its source and measurements are retained as rejected evidence. The packed kernel above is the current experiment.

Media comparisons used one worker limited to four CPU cores, CPU affinity 0-3 and 1 GiB memory. The receiver used separate cores 4-7, four CPU limit and 350 MiB. Each variant had a fresh container, identical immutable assets and cached speech. There were no overlapping builds. Short speech windows were 3.5 seconds; sustained windows were 45 seconds with 12 cached speech parts. After the first sustained scalar/ISPC pair, exactly one reverse-order confirmation was declared and run. No failed run was discarded.

CPU is total worker cgroup CPU-seconds per elapsed second, expressed in cores. Peak memory includes the complete worker cgroup through release. FPS is the receiver's displayed-frame measurement; decoded-frame FPS is separately retained in JSON.

| Run | Idle cores | Speech cores | Peak MiB | Received fps | Speech p95 gap ms | Decoded face dB | Media gates |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| Scalar short | 0.752 | 0.852 | 151.96 | 28.77 | 50.1 | 40.99 | Pass |
| ISPC short | 0.664 | 0.803 | 116.26 | 27.94 | 66.6 | 40.61 | Fail fps |
| libyuv short | 0.695 | 0.811 | 155.36 | 28.53 | 66.6 | 40.33 | Pass media; color tests fail |
| Combined short | 0.681 | 0.887 | 116.11 | 27.66 | 83.3 | 40.30 | Fail fps/gaps; color tests fail |
| Scalar sustained | 0.754 | 0.868 | 179.72 | 29.32 | 50.1 | 40.98 | Pass |
| ISPC sustained | 0.759 | 0.847 | 148.02 | 28.70 | 50.1 | 40.96 | Pass |
| ISPC confirmation | 0.738 | 0.900 | 181.33 | 28.85 | 50.1 | 40.61 | Pass |
| Scalar confirmation | 0.799 | 0.896 | 152.02 | 28.81 | 50.1 | 40.96 | Pass |

ISPC total speech CPU decreased 2.44% in the first sustained pair, then increased 0.39% in the reverse-order confirmation. CPU per rendered frame decreased 2.01%, then increased 0.85%. Peak memory decreased 17.64%, then increased 19.29%. These results do not demonstrate a repeatable CPU or memory benefit, and the no-memory-increase requirement is not met consistently.

Keep rendering and encoding separate: the full render call averaged 25.47 to 24.73 ms/frame in the first sustained pair and 26.41 to 26.36 ms/frame in the confirmation. Video encoding including color conversion averaged 2.29 to 2.36 ms/frame and 2.33 to 2.53 ms/frame. Those are elapsed stage times, not per-stage CPU accounting. ISPC changes no encoder code; no encoding speedup is established. libyuv's short encode-stage measurement was 1.38 ms/frame versus scalar's 2.39, but no sustained libyuv CPU gain is qualified because its exact conversion contract failed.

Two bounded 57-frame CPU profiles include offline PNG export and background changes. In the scalar profile, composition accounts for 0.26 sampled CPU-seconds, triangle rendering for 0.82 seconds, and background creation for 0.87 seconds; PNG export accounts for 2.06 seconds of the 4.27-second sample. The ISPC profile attributes 0.14 seconds to `runtime.cgocall`, 0.86 seconds to triangles and 0.90 seconds to background creation. These profiles identify remaining rendering work but are not representative media CPU percentages; native kernel time is not separately symbolized and PNG export is absent from streaming.

## Quality and behavior

All four variants produced byte-identical PNGs for the same 19 recorded scene states and their 38 mouth/eye component images. Minimum unencoded face PSNR against the retained browser reference is 54.37 dB, above the 45 dB gate. The safe ISPC wrapper also preserves padded, byte-stride and unaligned output layouts in focused tests. The later fixed-state profile verifies the same 19 scene images after that wrapper change.

Every decoded quality sample exceeded 40 dB. Controls passed in all eight media runs: pause retained the audio position, resume advanced it, interruption advanced the generation and silenced obsolete audio, reconnection completed, and cleanup left zero active sessions. No receiver drops, freezes, cgroup throttling or OOM were recorded. Existing exact libyuv color tests and both failed short media runs remain failures.

Inspection covered a dental transition, half blink, head turn and a source/decoded scene. No new fixed-frame seams or halos were observed; current H.264 compression remains visible. This is not owner approval, phonetic validation or a physical-device review. The media receiver exercises native H.264/Opus and the existing control envelopes; the public Vue player was not rerun for this slice.

Default native focused tests pass. ISPC renderer/media race tests and vet pass. No paid provider calls, asset regeneration, cloud changes, concurrency sizing or battery claims are involved.

## Reproduction and evidence

From the repository root, explicitly select a private experimental image:

```powershell
docker build --platform linux/amd64 -f services/render-worker/simd.Dockerfile --build-arg EXPERIMENT_TAGS=nativecodecs,ispc -t syntelle-render-native:ispc-experiment .
```

Use `nativecodecs` for the scalar experiment, `nativecodecs,libyuv` for the failed color candidate, or `nativecodecs,ispc,libyuv` for the combined candidate. These commands do not install or start a worker. Do not select these experiments through the normal launcher until their outstanding gates are resolved.

The complete trial is `.tools/simd-trial-60/`: original source, tested binaries/image identities, source/dependency hashes and licenses, libyuv source archive, native libraries, rejected kernel, fixed frames, all eight media reports, decoded images, raw benchmarks, profiles, scripts, comparison and verification records. The final experimental Docker recipe additionally copies the libyuv license and stops on any build failure; measured images were built from the frozen baseline plus the recorded binaries.

`archive-evidence.py` stores the complete evidence, source and immutable inputs through PostgreSQL `app.preparation.archive.Archive`, restores a fresh directory and compares every file hash. Its snapshot identity and restoration outcome are recorded separately in `.tools/simd-trial-60-archive.json`. This archive does not rerun any historical finalizer or change provider budgets.
