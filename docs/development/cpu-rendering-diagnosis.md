# CPU rendering diagnosis

The subsequent [native Go implementation and measurements](native-rendering.md) follow this diagnosis. The frozen control below remains diagnostic evidence, never a selectable replacement for animated video.

The October 21, 2026 follow-up addresses the owner's concern that only one user can run on their PC. The previous result qualified one session in a **four-vCPU Linux worker**, not the entire Ryzen 7 9700X machine (eight physical cores / sixteen logical processors). That worker deliberately used software graphics to represent the CPU-only deployment constraint. The result cannot establish the PC's maximum capacity or Azure's maximum capacity.

## Bounded diagnostic

Evidence is `.tools/render-profile-55/`, reusing the frozen phase-54 optimized build, pinned Linux image, cached audio, separate receiver, four-worker-CPU affinity/quota and one-GiB worker cap. No production code or resource limit changed. Two short one-session runs were made; this is not a fifty-session test.

The first run instruments canvas API call durations and reads Chromium's cumulative per-process CPU counters at the sample boundaries. It retains the actual animated 720 × 960 H.264 media and original speech clock. The worker uses 1.568 CPU cores in animated idle and 2.149 during speech, receiving 29.83 and 28.76 fps respectively. The software `GPU` process accounts for approximately **84.8% of total worker CPU** in both phases. Chromium reports SwiftShader, so this process name does not indicate physical GPU use.

The existing WebRTC encoder counter records about **1.36 ms of encoding time per speech frame**. That counter is elapsed encoder time, not a complete process CPU accounting metric; it must not be subtracted from multi-threaded CPU time. The process split and the controlled diagnostic below support rendering/composition as the principal bottleneck, rather than network transmission or the encoder alone.

The second run deliberately stops WebGL draw/clear calls after initialization. It keeps scene canvas copies, peer transport, audio, and other controls active. Worker CPU falls to **0.219 cores idle / 0.249 speech**, approximately 88.4% below the fully animated speech sample. The picture is frozen: this is an ablation to identify cost, **not an optimization, capacity qualification, preserved animation, or release candidate**. Automatic media gates alone would miss that loss; `qualifies_capacity: false` and `animation_intentionally_disabled: true` are explicit in the diagnostic report.

Canvas call durations include waits for asynchronous graphics completion and cannot independently identify shader CPU cost. The final scene's two copies per frame account for around 419 ms of blocking call time per second in the speech sample; this overlaps graphics execution and is not an additional 419 ms of CPU. The character passes through separate mouth/eye WebGL contexts, 2D patch compositing, texture uploads, a perspective body context and a 2D scene/capture canvas. The body is 765 × 1024, with 16,641 prepared vertices, 20,546 opaque triangles and 2,960 boundary triangles. These are concrete places to investigate, not evidence that geometry can be decimated without visible change.

## Implication for fifty sessions

The current implementation retains a browser-rendering pipeline designed for GPU execution and repeats animated graphics work per session on CPU. Small transport/codec improvements cannot address most of the measured cost. The owner is right to reject the current density as the final economical design for fifty simultaneous CPU-only sessions.

A subsequent implementation should remove or amortize the expensive graphics path: evaluate a renderer designed for CPU operation, eliminate intermediate readback/composition where possible, and consider sharing only genuinely identical idle frames. Independent speech, timing, settings and interrupt state must remain independent. Reuse is not assumed to make fifty simultaneously speaking users cheap, and a frozen animation must never be promoted as an efficiency improvement. Client rendering and GPU hosting remain outside the confirmed architecture.

The fifty-session objective remains **unvalidated**. Per-worker density, total service orchestration and public media connectivity are separate requirements. Merely increasing the capacity constant or scaling the current per-session design does not solve the density problem. No public deployment, paid provider request, asset regeneration or new dependency is involved in this diagnosis.

The probe scripts, full raw counters, frozen-image control, decoded/source images, source copies and asset-preservation hashes are archived using the generic PostgreSQL preparation archive with byte-verified restoration. Phase-54 evidence remains intact. These unpublished diagnostic endpoints are not product APIs; the temporary containers/network are removed after measurement.
