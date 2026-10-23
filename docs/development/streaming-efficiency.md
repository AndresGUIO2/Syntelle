# Local streaming efficiency

The October 21, 2026 follow-up keeps the existing WebRTC architecture, 720 × 960 scene, 30 fps target, 2.5 Mbps video ceiling and source-bound character resources. It introduces no paid request, new dependency, schema, public endpoint or deployment.

## Confirmed deployment constraint

The subsequent [CPU-only Linux follow-up](express-cpu-streaming.md) supplies the relevant measured optimization, rejected concurrency counts and public cost reference for Express. The NVIDIA measurements in this document remain historical local evidence.

The owner confirmed **Azure Container Apps Express**, which provides CPU compute and excludes GPU workloads; see the [official Express feature matrix](https://learn.microsoft.com/azure/container-apps/express-overview). All deployment sizing and economic conclusions must use CPU-only measurements. Do not assume a GPU workload profile, provision a separate GPU host or move rendering back to the consumer without a separately authorized architecture decision.

The 96% worker CPU reduction below relies on the local NVIDIA GPU and **does not apply to Express**. The earlier suggested 4–8-session range was a proposed test ladder, not measured capacity or a deployment estimate. The software-only intermediate report (`acceptance-final.json`) showed benefits from capture scheduling/H.264, but it is still a Windows sample, not a Linux Express capacity test. Full pinned Chromium can run with software rendering; selecting it does not create hardware acceleration on Express.

The existing private browser renderer is a local proof, not a demonstrated economical CPU-only media service. Continuous per-user rendering and encoding remain unvalidated for this deployment. Preserve the owner-selected server-rendered video contract and all local evidence; evaluate Linux CPU-only performance and actual media connectivity before committing cloud capacity or cost claims. No runtime rollback or new infrastructure was performed by this documentation correction.

## Changes

The private render host launches the full **pinned Chromium** with `channel: 'chromium'`, still headless. The former Playwright default launched its separate headless shell, which used SwiftShader on this Windows host. Full Chromium reports NVIDIA RTX 3090/D3D11 WebGL and enabled GPU compositing/rasterization. This establishes GPU rendering on this host, not a guaranteed hardware encoder or GPU availability on another deployment. Playwright documents the distinction in its [browser guide](https://playwright.dev/docs/browsers#chromium-new-headless-mode). Install the full Chromium for the existing Playwright version when preparing another host; there is no silent switch to an installed system browser or automatic dependency installation.

The character renderer accepts an optional private capture schedule. It draws at most at the target cadence and publishes that completed draw immediately. The preceding independent capture loop is removed. Animation and native mouth timing still follow their existing clocks; late frames use current state rather than replaying a queue. Client-rendered demo/laboratory contexts do not opt in and retain their prior schedule.

H.264 codecs are preferred in negotiation because they used substantially less video data in the local comparison. All other browser-provided codecs remain available; a VP8-only receiver was exercised successfully. The existing encoder identities are retained when applying bitrate/framerate limits. Codec choice is host/content dependent: this does not claim that H.264 always beats VP8, VP9 or AV1.

Unchanged pictures are not repeatedly captured. A genuinely frozen scene retains its last decoded image and the existing media connection; normal idle blinking, breathing and head motion continue at the existing target cadence. Audio, recognition, cancellation and provider budgets are unchanged.

## Bounded evidence

Evidence and before/current sources are in `.tools/stream-efficiency-52/`. The retained exploratory codec runs are separate from the final comparison. `acceptance-baseline.json` uses the old private build and headless shell; `acceptance-gpu.json` uses the final build and full Chromium. Both use separate receiver processes and the same original authorized Ava recording, restored from phase 12. The final comparison samples four seconds of idle and approximately 3.75 seconds of speech, then checks a frozen frame, resumption and release. These are short serial measurements, not sustained capacity or physical-device tests.

| Measurement | Previous | Final |
| --- | ---: | ---: |
| Idle video data | 2,325 kbps | 372 kbps |
| Speech video data | 2,444 kbps | 635 kbps |
| Idle worker CPU, core equivalents | 3.49 | 0.30 |
| Speech worker CPU, core equivalents | 6.36 | 0.25 |
| Speech receiver CPU, core equivalents | 0.164 | 0.134 |
| Received idle cadence | 29.17 fps | 29.37 fps |
| Received speech cadence | 24.10 fps | 29.73 fps |
| Speech frame-gap 95th percentile | 66.7 ms | 50.0 ms |
| Speech freezes / dropped decoded frames | 0 / 0 | 0 / 0 |
| Frozen-frame face PSNR | 42.05 dB | 41.41 dB |

CPU is summed from CDP process CPU-time deltas for the isolated worker browser, excluding the receiver and Node control process. Core equivalents are CPU seconds divided by elapsed seconds; they are not a percentage of the whole machine. Most of the CPU reduction comes from moving software rendering to the available GPU. GPU energy, VRAM, peak memory and physical battery were not measured. Receiver CPU is a desktop headless-shell sample, not a phone battery claim. Video data excludes audio, transport overhead and relay costs.

Resolution and original character/material bytes remain unchanged. Source-versus-decoded frozen-frame PSNR falls by less than 1 dB in both full-frame and face regions; inspected images show no obvious new face-detail artifact. This is bounded internal inspection, not proof of lossless video, temporal quality equivalence or owner acceptance.

The final timing marker measures video approximately 15 ms before audio. Earlier version-3 A/B reports show a contaminated 284 ms marker because their audio probe picked up the preceding speech tail. Preserve those reports but exclude their marker fields from conclusions. Version 4 drains that tail before measuring; the separate `transport-followup.json` also measured −18 ms. Markers are transport checks, not phonetic accuracy validation.

`compatibility-gpu-vp8.json` verifies actual VP8-only negotiation, decoded cached speech, pause/resume and resource release with the final host. No fixed bitrate/framerate reduction is imposed on moving idle or speech. A strict universal 30 fps guarantee, network-loss behavior, cloud capacity, physical battery, iPhone acceptance and public TURN remain unverified.

The final two-second post-pause sample receives 27.82 fps with zero freezes/dropped frames and approximately 30 captures/s. The archive's first check mistakenly required 28 fps instead of the browser probe's existing 25 fps functional resume threshold; its failed check and source are retained. The corrected archive uses the same functional threshold and records the actual cadence. This is not a sustained 30 fps acceptance claim. After installing/restarting the local worker, a separate actual Go/worker/player idle check negotiated H.264 at 720 × 960, received approximately 30 fps and reported no page errors or paid calls.

## Reproduction

Use the repository's existing Node 24 and pinned Playwright installation. Build the private worker using its existing build command, then run:

```powershell
.tools/node-v24.22.0-win-x64/node.exe services/render-worker/scripts/review-efficiency.mjs --speech .tools/stream-efficiency-52/cached-ava-en.json --output .tools/stream-efficiency-52/new-review.json
```

`--directory` selects a preserved private build. `--receiver-codec video/VP8` exercises negotiation compatibility. `--codec` is an isolated probe override, not a product setting. The probe connects only a private loopback worker and local receiver; it never calls the API or providers. Historical shell results must be replayed with their recorded host source, rather than assuming the current GPU host is the old baseline.

Freeze and restore this follow-up with the generic PostgreSQL preparation archive. Do not run a historical experiment finalizer. Preserve the exploratory records, earlier builds and untouched original audio; secrets and active browser sessions are not archive inputs.

## Why retain 30 fps

For this restrained conversational portrait, stable 30 fps is the current efficiency target. Raising it to 60 doubles the frame opportunities and halves the per-frame time budget; the resulting encode load, data rate and device energy depend on content and hardware. A future 60 fps option needs a visible benefit demonstrated against measured cost on target devices. This follow-up does not add such a mode or claim a 60 fps test.
