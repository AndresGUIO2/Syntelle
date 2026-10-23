# libyuv color and sustained-media follow-up

October 22, 2026. The owner authorized continued evaluation after the [initial SIMD trial](cpu-simd-trial.md). libyuv remains a viable, explicitly selectable candidate. Its bounded color contract and sustained media checks now pass; scalar remains the default because expanded decoded-quality and peak-memory gates remain open.

## Color contract

The initial exact-byte tests conflated one historical implementation's coefficients with the display contract. They are retained as `TestScalarI420PreservesKnownDisplayColors` and `TestScalarI420AveragesChromaWithoutChangingLuma`, calling `scalarI420` with all original expected bytes and display-inverse assertions unchanged. The phase-60 failed logs remain immutable. This is an explicit test-contract revision, not a claim that libyuv became byte-identical.

The selected converter now has separate tests against an independent floating-point BT.601 studio-range matrix. Before measurement, the follow-up declared a maximum error of two quantization levels for luma and 2x2 averaged chroma. Black/white luma and neutral chroma remain exact; legal studio ranges remain enforced. Solid color grids and mixed chroma cover 131,072 input pixels. libyuv also verifies padded rows, unchanged source bytes/padding, empty frames and rejection of truncated/invalid native buffers. Slice lengths, strides and native-int bounds are validated before C receives borrowed buffers. No per-frame conversion allocation or retained Go pointer is introduced.

All 19 original fixed scenes were converted by both implementations and reconstructed with the same independent limited-range inverse and chroma expansion. The largest actual I420 difference was **one level**. Minimum reconstructed face PSNR between candidate and scalar was **46.69 dB**, above the predeclared 45 dB requirement. The original rendered RGBA inputs were verified by SHA-256 and remain unchanged. This comparison measures conversion differences; it is distinct from comparing a 4:2:0 reconstruction with the source RGB image.

## Bounded sustained comparison

Four fresh-container Linux runs used a predeclared scalar/libyuv/libyuv/scalar order. Each replayed 45 seconds of the same cached speech, after the existing four-second idle check. ISPC was disabled. The worker used four CPU limit, CPU affinity 0-3 and 1 GiB memory; the receiver used separate cores 4-7 and 350 MiB. There were no overlapping builds or decoder analyses during the media measurements. All attempts are preserved.

| Run | Idle CPU cores | Speech CPU cores | Peak MiB | Received fps | Speech p95 gap ms | Decoded face dB |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| Scalar A | 0.731 | 0.841 | 174.02 | 29.57 | 50.1 | 40.61 |
| libyuv A | 0.733 | 0.810 | 147.48 | 29.64 | 33.4 | 40.59 |
| libyuv B | 0.712 | 0.807 | 175.22 | 29.70 | 50.0 | 40.32 |
| Scalar B | 0.736 | 0.834 | 143.81 | 29.21 | 50.0 | 41.00 |

Mean total speech worker CPU decreased from **0.838 to 0.809 cores, 3.46%**; reductions occurred in both pairings. CPU per rendered frame decreased **3.58%**. Mean video encode stage, including conversion, fell from **2.19 to 1.27 ms/frame, 42.00% less elapsed time**. Mean full-render time stayed near 24.5 ms/frame; libyuv does not change rendering. Stage values are elapsed wall time, not isolated encoder CPU. The earlier approximately 13x conversion microbenchmark is not a total-worker speedup.

Peak memory ranged from **143.81-174.02 MiB** for scalar and **147.48-175.22 MiB** for libyuv. The highest candidate peak was 1.20 MiB above the highest scalar peak. There is no demonstrated memory saving or consistently satisfied no-peak-increase gate.

All four media runs passed received fps >=28, p95 gaps <=66.8 ms, decoded face >=40 dB, audio/control timing, pause/resume, interruption-generation invalidation, reconnection and complete release. No receiver drops, freezes, throttling or OOM occurred. This receiver exercises the existing native media/control envelopes; this slice did not rerun the public Vue player or physical devices.

## Expanded decoded-quality finding

For every fixed scene, each implementation encoded a fresh 30-frame H.264 clip with the unchanged production 720 x 960, 30 fps, 2.5 Mbps, single-thread OpenH264 settings. The corpus contains 1,140 decoded frames across 38 clips. Initial encoding and steady-frame quality are both retained rather than selecting only favorable frames.

The independent OpenCV 5.0.0/FFmpeg decoder found minimum face PSNR **36.79 dB scalar / 36.48 dB libyuv**. A second decoder check used Chromium software WebCodecs plus Canvas2D display RGB on the exact same cached clips. Its minimums were **37.37 / 37.23 dB**, and its worst final frames were **38.43 / 38.24 dB**. Both implementations miss the original 40 dB requirement on this expanded fixed-state corpus. The passing single-snapshot streaming measurements therefore do not establish that requirement for every pose/frame.

The small conversion difference remains measurable, and the shared decoded-quality shortfall also exists in scalar. No bitrate, encoder quality, resolution, sampling, pose, frame count or threshold was changed to make this follow-up pass. The first decoder and Chromium differ in RGB conversion and cannot be treated as interchangeable numerical measurements. The exact quality reports and source/decoded images are retained for a separate encoder-quality correction.

Inspection of the original, scalar decoded and libyuv decoded closed-mouth scene, plus the Chromium-decoded A/FV midpoint pair, found the same visible compression family and a slight candidate color difference, with no new rendering geometry seam. The initial fixed-scene corpus already covers dental transitions, blinks, gaze, expressions and head movement. This is internal bounded inspection, not owner or physical-device approval.

## Current selection and reproduction

Keep `libyuv` as an optional build tag. Do not reject it solely because it differs from scalar by one code value. Do not enable it by default until the expanded decoded-quality requirement and peak-memory condition are resolved. Go continues to own sessions, buffers, audio clocks, animation and Pion. There are no provider calls, source-asset changes, new image generations, cloud changes or concurrency claims.

From the repository root, build an explicit experimental image without replacing or starting the running worker:

```powershell
docker build --platform linux/amd64 -f services/render-worker/simd.Dockerfile --build-arg EXPERIMENT_TAGS=nativecodecs,libyuv -t syntelle-render-native:libyuv-experiment .
```

Inside the experimental SDK with pinned native headers/libraries, run `go test -tags=nativecodecs,libyuv ./internal/nativemedia`; the default scalar suite, libyuv race suite and libyuv vet pass. The explicit `integration` fixture test accepts private PNG input/output directories through `SYNTELLE_COLOR_FIXTURES` and `SYNTELLE_COLOR_OUTPUT`. It requires a fresh output directory, 19 recorded scenes and never contacts a service. It is an offline corpus-export test, not a provider/database integration test.

Complete evidence is `.tools/libyuv-validation-61/`: the predeclared plan, original and tested sources, old exact failures, all four media runs, raw I420 planes, 38 H.264 clips, every-frame metrics from two decoders, decoded PNGs, dependency/compiler identities and reproduction scripts. The first source-copy error and decoder-report serialization correction are preserved and described; neither changed the reported thresholds or encoder configuration. Test fixtures and native runtime images use the same source/toolchain, and image/binary hashes identify each variant.

`archive-evidence.py` archives this follow-up through the generic PostgreSQL `Archive` interface, restores a fresh directory and verifies every file hash. `.tools/libyuv-validation-61-archive.json` holds the snapshot identity and verified restoration result. Historical phase-60 evidence and snapshot 554 are not overwritten. The active worker and standard `native.Dockerfile` remain unchanged.
