# CPU-only Express streaming: bounded Linux measurements

The October 21, 2026 follow-up optimizes the existing private renderer for the owner-confirmed **Azure Container Apps Express** target. It does not deploy cloud infrastructure or replace server-rendered video with consumer rendering. All media/preparation/provider inputs remain cached and unchanged; verification made zero paid requests.

This is the preserved Chromium comparison reference. The subsequent [native Go worker](native-rendering.md) removes the per-session browser and records separate quality, behavior and resource gates without expanding this change into deployment or concurrency sizing.

## Implemented changes

`completedMorph.ts` now uses the existing signed-distance aperture directly for semantic packages, instead of also evaluating a redundant polygon boundary. Pixels with zero aperture coverage skip cavity, enamel and shadow work. Pixels without enamel skip enamel lighting. The nearest lip-edge calculation takes the minimum squared distance before one square root, and the existing gamma-space lighting is factored into one scalar power rather than twelve channel powers. These are algebraic/work-avoidance changes, not new dental masks, colors, geometry or textures.

The private scene computes its existing face-relative placement once and shares the visible source rectangle with the perspective renderer. A scissor rectangle, enlarged by two body-raster pixels for final bilinear filtering, prevents fragment work outside the delivered picture. Output remains **720 × 960**, targeting **30 fps**, with the existing 2.5 Mbps video ceiling. Client-rendered contexts do not opt into this clip. Native audio clocks, movement, gaze, blinks, expression settings and all source materials remain unchanged.

H.264-first negotiation and the phase-52 shared render/capture schedule remain in place. No SwiftShader thread override was promoted: an isolated two-thread experiment did not restore two-session fluency and remains rejected evidence.

## Test method and limits

Evidence is `.tools/express-cpu-54/`. Tests use Ubuntu Noble, the existing Playwright **1.64.1 / Chromium 156.0.8078.4**, and the existing **Node 24.22.0** binary copied from the cached project image. No browser or dependency version was changed. Chromium reports **SwiftShader**; hardware GPU rendering, video encoding and decoding are unavailable in the worker.

The worker and receiver run in **separate Linux containers**. Worker cgroup CPU usage includes Node, Chromium, software rasterization and encoding; receiver decoding is excluded. The main worker has a four-vCPU quota and affinity to four logical processors. A separate trial lowers only the quota to two vCPU. Samples use four seconds of idle and 3.5 seconds of original cached Ava speech. Two-session tests start speech on both sessions before the measurement; start skew remains part of this short test.

Docker Desktop provides approximately **1.916 GiB total VM memory** on this machine. To avoid disturbing existing project services, the test worker has a stricter **1 GiB memory cap**, and the receiver has a separate 350 MiB cap. No tested worker hit an OOM event. These are local CPU-quota measurements, not Azure hardware benchmarks. The cloud cost reference uses a valid Consumption allocation of **4 vCPU / 8 GiB**, not the deliberately tighter local memory cap. Express inherits Consumption billing; see [Express](https://learn.microsoft.com/azure/container-apps/express-overview), [resource combinations](https://learn.microsoft.com/azure/container-apps/containers) and [billing](https://learn.microsoft.com/azure/container-apps/billing).

Acceptance gates were defined in `review-cpu.mjs`: at least 28 received fps in both phases, speech p95 frame gaps at most 66.8 ms, no speech freezes or decoded drops, actual decoded audio, unchanged dimensions, face source-versus-decoded PSNR at least 40 dB, no page errors and complete session release. A target is not an exact 30 fps guarantee.

| Variant | Worker quota | Sessions | Idle fps per session | Speech fps per session | Speech worker CPU cores | Result |
| --- | ---: | ---: | --- | --- | ---: | --- |
| Preceding renderer | 4 vCPU | 1 | 28.92 | 25.25 | 2.95 | Speech fails |
| Final optimized renderer | 4 vCPU | 1 | 29.93 | 29.36 | 2.09 | Pass |
| Final optimized renderer | 4 vCPU | 2 | 27.50 / 28.00 | 23.10 / 22.54 | 3.79 | Fails |
| Rejected two-thread override | 4 vCPU | 2 | 26.52 / 25.79 | 24.12 / 22.72 | 3.81 | Fails |
| Final optimized renderer | 2 vCPU | 1 | 28.61 | 22.43 | 2.02 | Speech fails |

Speech CPU fell **29.04%**, while delivered speech cadence improved. Idle CPU fell approximately **21.23%**. The passing worker used about **560 MiB** during speech; its cumulative peak was about **689 MiB**. Memory reduction is not claimed. The two-session peak was about 746 MiB, so its rejected cadence is not an observed memory-limit failure. No four/eight-session run was made after two sessions failed the quality gates.

The supported conclusion is **one locally qualified session per four-vCPU worker**, not an Azure concurrency guarantee or a maximum for a different renderer. Two sessions are explicitly not qualified. `services/render-worker/express-cpu.profile.json` records this sizing reference and a session capacity of one, to be applied consistently to Go and the worker if this CPU candidate is used. It does not change the preceding local GPU trial's global configuration or implement multi-worker routing.

## Quality and timing

The separate no-network Linux pixel check compares the original and optimized mouth renderers for `Rest-A`, `A-FV` and `EI-U` at 0, 0.5 and 1. It also compares unclipped/clipped final scenes at center and both phase-18 head-turn bounds with breathing. All twelve cases have **zero differing visible channels**, including alpha in mouth samples. This is a focused equivalence check rather than an exhaustive pose/browser matrix.

The passing end-to-end sample has face PSNR **41.48 dB**, compared with **41.67 dB** for the original codec sample; video encoding is lossy. No speech freezes or decoded drops were observed. The separate same-container transport probe measures video **49 ms before audio**, decoded cached speech onset at **128 ms** and interrupt control at **11.4 ms**. That probe shares its CPU quota with its receiver, so its aggregate 26.97 fps is not substituted for the separate-container capacity test. Marker timing is not phonetic validation or a physical speaker/microphone claim.

## Public cost reference

Read-only Azure Retail Prices API results are preserved as `azure-retail-eastus.json` and `azure-bandwidth-eastus.json`. The reference is **East US, USD, public pay-as-you-go rates**, as observed on October 21, 2026:

- Active vCPU: **US$0.000024 / vCPU-second**.
- Active memory: **US$0.000003 / GiB-second**.
- Internet egress using Microsoft Global Network: **US$0.087 / GB** at the first paid tier.

The [pricing page](https://azure.microsoft.com/pricing/details/container-apps/) confirms that Express follows Consumption pricing. The exact numeric meters come from the [Retail Prices API](https://prices.azure.com/api/retail/prices); public rates are a reference, not a subscription quote.

For one active **4-vCPU / 8-GiB** replica:

`(4 × 0.000024 + 8 × 0.000003) × 3600 = US$0.432 per replica-hour`.

At the measured one-session capacity and full single-slot occupancy, compute is therefore approximately **US$0.432 per connected user-hour**, **US$0.0072 per connected minute**, or **US$0.216 per 30-minute session**. Charging is based on allocated resources, not just the observed 2.09 CPU cores. Animated idle remains active compute even when no AI request is running.

The final samples send about 249 kbps of video in animated idle and 994 kbps during speech. Using decimal payload GB and the first paid egress tier adds approximately **US$0.010–0.039 per user-hour**, yielding a reference **US$0.442–0.471 per user-hour for worker compute plus video payload**. This does not include audio, protocol overhead, TURN relay, shared Go/FastAPI/PostgreSQL resources, recognition, model calls or TTS. Free grants, taxes, discounts, cold starts and spare warm capacity are also excluded. A real bill depends on region, subscription, occupancy and usage mix.

The pipeline is measurably better, but its current per-user private-browser/software-WebGL design still has low CPU density. The measured results do not justify claiming cheap high-concurrency service on Express. Native CPU rendering or a separately validated reuse strategy would be a further implementation, not an assumed benefit of this change.

The subsequent [CPU rendering diagnosis](cpu-rendering-diagnosis.md) attributes approximately 85% of worker CPU to the software graphics process. A deliberately frozen-graphics control isolates this cost but is not a usable optimization. The one-session result is a four-vCPU worker qualification, not the entire development PC's capacity.

## Reproduction and retained failures

`cpu-probe-host.mjs` and `review-cpu.mjs` are **offline test tools**, not public HTTP services. The probe control endpoint must stay on the dedicated unpublished Docker network. The staged image contains only selected runtime files/builds, pinned existing Playwright packages and cached audio; no `.env`, user microphone input, provider credentials or live API connection is supplied.

The original Docker `--internal` network had no default route, and ICE did not produce usable media connections. Failed diagnostics are retained. The successful dedicated bridge has a default route; private browser requests remain restricted to its asset host, and test traffic uses only the probe and media peers. Probe-only mDNS suppression is not installed in the product. Earlier fixture corrections include stale relay delivery between repeated IDs and inlined quality images rejected by the unchanged CSP; unique per-run IDs and allowed blob fixtures correct those checks. The first coverage-only optimization still failed speech cadence and remains as `first-optimization-1.json`.

Build the existing private worker, then use the preserved Dockerfile/staged inputs in `.tools/express-cpu-54/linux/`. Run the worker with `--cpus 4 --cpuset-cpus 0-3 --memory 1g`, mount the selected preserved build read-only, and run the receiver in a separate container on logical CPUs 4–7. Supply `--sessions 1` or `2`; preserve failed reports rather than overwriting them. The full commands, source files, builds, pricing inputs, rejected configurations and cached audio are archived through the generic PostgreSQL preparation archive and byte-verified restoration. Do not rerun a historical motion finalizer.

No GPU host, provider request, public deployment or consumer-rendering switch is introduced. Public WebRTC connectivity through Express's HTTP-only ingress, TURN costs, single-worker routing/scaling, sustained/noisy-neighbor behavior, physical devices and owner acceptance remain pending.
