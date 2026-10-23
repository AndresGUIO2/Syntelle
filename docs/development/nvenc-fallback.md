# NVIDIA encoding with automatic software fallback

Current follow-up: [native GPU rendering and WSL2](native-gpu-rendering.md) adds CUDA rasterization, enables Docker GPU access and records a newer packed-frame comparison. The results below preserve the phase-63 opaque-frame experiment. Current NVENC level is 3.1 for 720 x 960 and 4.0 for packed 1440 x 960; 30 fps, P5 and the 2.5 Mbps cap remain unchanged.

The standard native Linux amd64 image includes NVENC. `SYNTELLE_VIDEO_ENCODER=auto` is the default: try NVIDIA H.264, otherwise create the existing OpenH264 encoder. A failed GPU session switches permanently to OpenH264, re-encodes the current frame as an IDR with fresh SPS/PPS, and retains the same Pion track and media clock. Failed GPU packets are discarded. No subprocess, browser, frame queue or provider request is added.

The CPU renderer, prepared resources, libyuv conversion, 720 x 960 resolution, 30 fps target, 2.5 Mbps video rate setting and Opus remain unchanged. NVENC accelerates encoding only. It uses baseline H.264 level 3.1 matching SDP, limited-range BT.601 with BT.709 primaries/transfer, P5/low-latency VBR, a one-second VBV budget, no B-frames/lookahead, immediate output and forced IDRs. VBV is rate control, not an application queue. Conversion writes directly into reusable C-owned I420 planes; packets are copied once into Go-owned bytes for Pion retention.

## Selection and dependencies

The launcher checks Docker NVIDIA access with a network-disabled, unprivileged `--gpus all` probe bounded to ten seconds. It cleans the probe on either outcome. A usable runtime receives `NVIDIA_DRIVER_CAPABILITIES=compute,video,utility`. Encoder initialization then verifies actual driver/device/session support, including capacity failures.

```powershell
npm --prefix services/render-worker run build
npm --prefix services/render-worker run start -- --detach
```

`--video-encoder openh264` selects software explicitly. `--video-encoder nvenc` requires NVENC and fails instead of concealing hardware errors; use it for qualification. The private environment variable accepts the same values. Browser commands cannot choose the backend. Private structured logs record selection and fallback without native driver strings or media contents.

The build pins FFmpeg 8.0.3 at `8ae0b34901ba60a802f183ee75a250a9fc3e09a5` and nv-codec-headers 13.0.19.0 at `e844e5b26f46bb77479f063029595293aa8f812d`. Only shared libavcodec/libavutil and the H.264 NVENC encoder are built; no FFmpeg executable or CUDA toolkit is installed. Driver libraries are discovered at runtime. Configuration, hashes and licenses are under `/usr/local/share/syntelle/`. These headers require NVIDIA driver 570 or newer; absent/older drivers select software in automatic mode. Existing OpenH264/Opus/Pion pins remain unchanged.

## Bounded checks, October 22, 2026

Evidence is `.tools/nvenc-63/`, including rejected settings and failed verification attempts. No paid calls were made.

- RTX 3090, driver 610.88, Linux/WSL: three recorded scenes (mouth transition, half blink, head turn), 30 frames each through software, NVENC and an injected GPU-to-software transition. All 270 decoded frames survive, including the replacement reference chain. Profile, level, color, immediate output, keyframes and retained-packet ownership are asserted. Mean face PSNR is 38.30 dB software versus 39.84 dB NVENC; every matched GPU frame improves by at least 0.73 dB. Minimums are 36.52 and 38.08 dB. This preserves relative quality but does **not** close the historical every-frame 40 dB gate.
- Initial two-frame-VBV/P4/CBR output lowered first-frame quality; it was rejected and preserved. Final mouth and switch frames were visually inspected. Geometry and assets are unchanged.
- Real H.264/Opus/Pion checks, startup and mid-stream fallback, focused Go race/vet and eight launcher tests pass. The GPU rejects the old ownership test's 128 x 128 fixture; automatic mode correctly uses software there. The recorded-frame test checks GPU ownership at the actual 720 x 960 size. A deliberately forced-NVENC run exposing this unsupported small size remains as failed evidence.
- Two brief receiver comparisons replay cached speech and check controls/reconnection/release. Final WSL runs restrict CPU affinity and GOMAXPROCS to four. Speech receives 27.30 fps software and 27.60 fps NVENC; neither meets 28 fps. There are no decoder drops/freezes. The final timing/control aggregate also misses its gate (NVENC video/audio offset 81.5 ms); earlier unrestricted runs pass controls but miss cadence. These results do **not** establish complete fluency qualification.
- GNU time measures process peak RSS because WSL cgroup memory fields are unavailable: 128.38 MiB software versus 230.51 MiB NVENC. Whole-run user+system CPU is 9.88 versus 9.86 seconds: no meaningful total CPU saving is proven. Receiver cgroup counters describe WSL rather than an isolated worker and must not be reported as worker CPU. GPU encoding can increase memory and transfer overhead at this resolution.
- Docker Hyper-V cannot load `libnvidia-ml.so.1` on this workstation, so normal local startup selects OpenH264. Real GPU checks ran in WSL. A separate Docker public-player check could not create its missing cached receiver image; its isolated database was cleaned. Docker GPU/player qualification remains incomplete.
- The installed automatic-mode image passes a separate five-second check through the actual local public player: 29.90 fps, p95 gap 66.6 ms, zero dropped frames, no browser errors and explicit session release. Logs confirm OpenH264 fallback. This idle check does not qualify sustained speech or Docker NVENC.

The existing rendering/cadence shortfall remains separate work. Deployment capacity and a universal no-gap hardware-failure guarantee are not established.

Sources: [NVIDIA integration](https://docs.nvidia.com/video-technologies/video-codec-sdk/13.0/ffmpeg-with-nvidia-gpu/index.html), [pinned NVENC source](https://github.com/FFmpeg/FFmpeg/blob/8ae0b34901ba60a802f183ee75a250a9fc3e09a5/libavcodec/nvenc.c), [driver requirement](https://github.com/FFmpeg/nv-codec-headers/blob/e844e5b26f46bb77479f063029595293aa8f812d/README).
