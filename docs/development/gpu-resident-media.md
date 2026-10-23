# GPU-resident character media

Follow-up: [shared immutable GPU assets](shared-gpu-assets.md) replaces repeated session uploads of read-only character inputs while retaining private working/frame buffers and independent animation.

The standard native worker now keeps character pixels on GPU through NVENC. Mouth shading, teeth/cavity composition, eyes/gaze/blinks, prepared skin mixing, expression sampling, RGB-D rasterization, alpha packing and BT.601 conversion run in CUDA. Only compressed H.264 bytes return to Go for Pion. The existing CPU/OpenH264 fallback, immutable resources, private envelopes, player controls, packed 1440 x 960 transport, presented 720 x 960, 30 fps target and 2.5 Mbps cap remain.

## Ownership and transfers

NVENC owns a two-surface CUDA I420 pool in the same retained primary CUDA context as the renderer. The video loop borrows one surface, renders/converts into it, synchronizes that session's CUDA stream and submits the surface directly. There is no normal RGBA device-to-host readback, host libyuv conversion or host-to-device image upload in this path. The compressed packet is copied once into Go-owned storage because Pion may retain it.

Prepared textures and oral/eye/skin companions upload once per scene. Mouth and eye kernels run only when their state or skin base changes. CPU code retains animation/control, validated mouth vertex construction, projected body geometry/ordered tile bins, Opus and WebRTC packet handling. Small geometry/state uploads remain; this is not a claim of zero CPU or zero transfers of any kind. The CPU reference shares the same oral geometry/state preparation, while pixel shading executes on GPU. GPU double precision, filtering, coverage, alpha quantization and source-atop composition preserve the reference pixels. Range-reduced double lighting avoids an additional CUDA toolkit dependency.

Each session owns a nonblocking CUDA stream and reusable device buffers. No new media goroutine or frame queue is introduced. Cancellation and generation checks retain the existing publication boundary. On GPU-render failure, CPU rebuilds the current state's patches. On NVENC failure, a readback of that current rendered frame supplies a fresh software IDR; partial GPU packets are discarded. Explicit inspection snapshots may read back RGBA. Cleanup serializes inspection with scene/device release, closes encoders and frees buffers/modules/context references after the media loops finish.

`--renderer auto|gpu|cpu` and `--video-encoder auto|nvenc|openh264` remain independent. The private comparison option `--gpu-pipeline host` preserves the preceding CPU patches, RGBA readback, libyuv conversion and NVENC upload with the same animation/clock. Default `device` uses GPU surfaces. `SYNTELLE_GPU_PIPELINE` accepts the same values through the local launcher. This is a development comparison setting, not a public player control. `syntelle-render-native:before-gpu-resident-67` preserves the preceding installed image.

## Windows cadence correction

The previous Windows shortfall was reproducible independently of pixel processing: the worker generated approximately 30 frames per adjusted Linux second while Windows received 26-27.5 per real second. Persistent subprocess measurements show approximately 1.11 Windows seconds per Linux adjusted second. Kernel `adjtimex` reported tick values around 9,100-9,400 microseconds rather than the nominal 10,000, with smaller additional frequency corrections. The responsible time-service adjustment was not identified.

Changing to the available Hyper-V shared-page, Hyper-V MSR and ACPI clock sources did not remove the adjusted-clock rate mismatch. Those diagnostic changes were reverted to the original `tsc` source; no persistent WSL configuration, kernel rate or system time was changed.

`internal/nativeclock` now schedules audio and video against Linux `CLOCK_MONOTONIC_RAW`. A reusable cancellable timer translates each raw deadline to Go's adjusted timer clock using a read-only `adjtimex` rate sample, refreshed once per raw second. Early wakeups sleep again; late obsolete deadlines are skipped without building a queue. PCM blocks, viseme interpolation and video use the same raw media progression. Other operating systems retain their normal monotonic clock. Existing synthetic-time tests retain their explicitly constructed clock state. The existing pinned `golang.org/x/sys v0.41.0` supplies Linux calls; no dependency version changes.

Worker stage/accounting durations and the offline probe use raw seconds. Comparing process CPU against adjusted receiver seconds had also biased the earlier absolute CPU figures. The final comparison below uses cgroup CPU divided by worker raw elapsed time.

Relevant primary references: [Linux monotonic clock behavior](https://man7.org/linux/man-pages/man2/clock_gettime.2.html), [kernel clock adjustments](https://man7.org/linux/man-pages/man2/adjtimex.2.html), [Hyper-V clocks](https://www.kernel.org/doc/html/v6.12/virt/hyperv/clocks.html), [FFmpeg CUDA context API](https://ffmpeg.org/doxygen/8.0/hwcontext__cuda_8h_source.html), [NVIDIA encoder resource interfaces](https://docs.nvidia.com/video-technologies/video-codec-sdk/13.1/nvenc-video-encoder-api-prog-guide/index.html). The implementation retains the existing pinned FFmpeg 8.0.3/nv-codec-headers sources, not the newer documentation versions as dependencies.

## Bounded verification, October 22, 2026

Evidence is `.tools/gpu-resident-67/`. No asset generation, paid provider call, deployment or concurrency/admission change is involved. All speech comes from the existing cache.

- All 38 recorded CPU/CUDA RGBA comparisons (19 states in each output format) have zero byte differences, including mouth transitions, teeth, eyes/gaze, expressions, head/breath motion, hair alpha and framing. Both the preceding compatibility-failure path and failure before any CPU patches were prepared rebuild the exact current CPU frame.
- Three fixed scenes, 30 frames per path: GPU I420 is byte identical to libyuv I420, the normal device path leaves the supplied host RGBA buffer untouched, and the complete direct-surface H.264 clips are byte identical to upload-based NVENC clips (180 total encoded frames). This establishes no added conversion/codec loss for these fixtures. Raw and decoded sampled mouth/eye edges were inspected; owner/device acceptance remains separate.
- Focused Go race/vet checks cover clock, animation, adapter, renderer, media and worker/probe wiring. Portable non-GPU package tests and eight launcher tests pass. Surface initialization/encode failure tests discard partial packets, release hardware and require a fresh current software IDR. The raw pacer has bounded deadline/cancellation checks.
- Actual Windows Chromium/D3D11 public player: the preceding host path presents 26.56 fps, device surfaces alone 27.02, and raw-clock scheduling 29.94. The final five-second public-player/control check presents 30.01 fps, p95 gap 33.4 ms, maximum gap 38.9 ms, zero dropped frames and no browser errors. Background changes, sound control, PiP, continued PiP frames and explicit session release pass. This is a bounded idle/control check, not sustained-load or phone qualification.

Final matched resource/media comparison: the same Linux Docker/WSL2 worker image/source/assets, RTX 3090 driver 610.88, four-CPU limit/affinity `0-3`, 1 GiB worker memory, Windows Chromium/D3D11 receiver over explicit host-loopback HTTP/SRTP, one session, four seconds animated idle and eight seconds cached speech in three parts plus controls. Both comparison modes use the corrected raw clock, isolating rendering/transfer changes at the same real frame rate. Worker CPU is isolated cgroup usage per raw second; stage values are wall time, not stage CPU attribution.

| Measurement | Host patches/transfers reference | GPU pixels/direct NVENC |
| --- | ---: | ---: |
| Idle total worker CPU, cores | 0.300 | 0.218 |
| Speech total worker CPU, cores | 0.540 | 0.374 |
| Speech peak cgroup memory, MiB | 308.70 | 244.20 |
| Complete-lifecycle peak, MiB | 324.85 | 249.51 |
| Speech render call, ms/frame | 15.07 | 9.94 |
| Speech encoder call, ms/frame | 6.41 | 5.17 |
| Received speech fps | 29.81 | 29.85 |
| p95 speech frame gap, ms | 38.99 | 38.89 |

This is 30.6% less speech CPU and 20.9% less speech peak memory (23.2% less lifecycle peak). Rendering call time falls 34.0%; encoder call time falls 19.4%. Conversion now belongs to the GPU render call, so these stage percentages must not be labeled rendering/encoding CPU savings. Total worker CPU is measured independently. Idle CPU falls 27.4%.

Both final modes pass cadence, sampled decoded quality, pause/resume, interruption, audio/video marker offset, reconnection and complete release; neither drops decoder frames or freezes. Candidate marker offset is +63.2 ms versus +62.2 ms reference. Interruption reaches silence in 3.1 ms; reconnection is 338 ms. All 407 candidate frames captured by the speech-end snapshot used device surfaces; repeated video timestamps are zero. Sampled face PSNR is 41.93 dB candidate and 43.66 dB reference; these live snapshots have different encoder histories and do not replace the byte-identical controlled clips.

Preserve initial and failed checks. Before correcting pacing, both Linux-contained receiver comparisons pass controls. After raw pacing, that receiver's own adjusted/null-audio clock misses the marker-offset gate (-150/-234 ms); the final receiver was moved to Windows, where clock units match actual playback and both paths pass. Earlier process-per-call sleep measurements included Docker startup/exit overhead; persistent streaming samples confirm the rate mismatch. An initial fixture test failed because the raw OS clock bypassed synthetic frozen time; explicitly constructed test sessions now retain their original deterministic clock. None of these failures are erased or relabeled as passing.

The inherited every-frame decoded 40 dB gate remains open: this change retains the same codec/bitrate and its controlled compressed output. No 60 fps, full-load capacity, arbitrary-character preparation, real-phone or public-deployment claim is made. Complete selected source, frames, compressed clips, measurements, failed checks and build/clock evidence are archived in PostgreSQL with hash-verified restoration; the result is `.tools/gpu-resident-67-archive.json`.

During finalization, another local change rebuilt the active worker and installed a newer expression package. That update is preserved. The measured package manifest is `17676b6f0fb3156a6050b1dbc1197061776bfafc9536caf86ca90e757b8da359`; the newer active package is `e45252eb9dae7fcaa489008cd87351e1a3eaafd474df636442e6436cb86b6a9f`. An additional 38 CPU/CUDA comparisons on that active package also have zero byte differences, including prepared skin/joint articulation, and its public Windows player/control check passes at 29.94 fps, p95 gap 38.9 ms, zero drops/errors. The active image retains exactly the qualified CUDA source/PTX hashes and codec libraries. `runtime.json` distinguishes the measured image from this concurrent rebuild; `measurement-source/` preserves the measured implementation separately. Resource percentages above belong to the frozen matched benchmark, not a new capacity claim for the later package.
