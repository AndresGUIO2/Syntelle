# Native CPU rendering

The current worker is native-only: `npm run build` and `npm start` in `services/render-worker` build and launch the Go image. The former Chromium worker, private web entry and comparison launchers have been retired. Playwright/Chromium remains for public-player and native-receiver tests, and TypeScript animation exporters remain for deterministic fixtures. Historical browser code and results are preserved in `.tools/native-render-56/`, `.tools/native-efficiency-57/` and verified PostgreSQL snapshots 518 and 521.

The subsequent [native efficiency follow-up](native-efficiency.md) retains these appearance/media guarantees while moving speech analysis off the rendering path and removing redundant encoded-payload copies. Its native-before/after results are separate from the historical Chromium comparison below.

This follow-up replaces the private per-session Chromium/SwiftShader path with a native Go rendering/media candidate for the existing Syntelle character. Its scope is removing measured browser/graphics overhead while preserving current appearance and behavior. Concurrency sizing and cloud deployment are outside this iteration. It does not establish a fifty-user capacity, cost forecast, public media route or visual acceptance.

The local private worker selected native after the quality, behavior and resource gates below passed. It retains the existing public player/API. Follow the [worker README](../../services/render-worker/README.md) for the pinned minimal image, cached asset export and launcher. The following implementation and comparison measurements describe that replacement; removing the unused reference runtime does not change their recorded results.

## Implementation

The public Vue player and Go conversation/account APIs retain their responsibilities. FastAPI remains preparation-only. The new private worker owns CPU rendering, PCM-timed character state, H.264/Opus encoding and WebRTC media through a native Go path. It reuses the existing worker control messages; it does not send character textures or geometry to the consumer.

The character bundle is a deterministic CPU-ready export of cached resources: original oral materials/masks, foreground, eyes, gaze ownership, expression field/smile material, RGB-D geometry and supplied backgrounds. Complete source JSON, resource hashes and original build identifiers remain in the manifest. No image regeneration, provider request, new calibration or manual mask is required. Original resources remain untouched; the new export is separate and private.

`nativeanimation` ports the current TypeScript state math: seeded conversational head/eye attention, spontaneous/contextual/double blinks, asymmetric breathing, Azure viseme interpolation, acoustic speech cues, directed gesture admission and closed-smile/brow behavior. The private catalog's directed performance policy remains active; audio does not invent semantic gestures, and unavailable laughter is not admitted.

`nativerender` composes mouth/eye updates with the prepared dense perspective mesh and premultiplied boundary handling. The initial parity candidate preserves the preceding body-raster size and final 720 × 960 scene sampling; a native rewrite must not silently change sampling, opacity, tooth ownership or speech timing in the name of optimization. Pixel differences and any rejected intermediate candidates remain evidence for review.

`nativeadapter` loads shared read-only assets/configuration once. Each scene owns its mutable renderer, state directors and current prepared speech. It caches a clip by immutable PCM identity, retains the completed plan for expression/gesture release, and clears it when the media generation changes. Waiting speech settles the closed smile before articulation. Pause freezes animation elapsed time while the media transport's timestamp clock continues; speech position remains driven by emitted PCM samples.

The export retains the API's original prepared-file-list character hash (`89b7208bb1f9d0390283260f0bfd3cd055dd8666dc77bdca52eb82ef28f572a2`, 126 files), rather than inventing a renderer-specific identity. The native loader verifies that identity, raw texture/geometry checksums and original package bindings. Public create/signaling messages therefore keep the same character identity when changing workers.

The existing Go module pins Pion WebRTC v4.2.25 and mediadevices v0.10.1, including its vendored OpenH264/Opus native archives. The [pinned OpenH264 interface](https://pkg.go.dev/github.com/pion/mediadevices@v0.10.1/pkg/codec/openh264) supplies frame-skip, threading and keyframe controls. Native codec builds require CGO and the `nativecodecs` tag. The portable build returns an explicit unavailable-codec error; it does not substitute a browser or codec. RGB is converted into reusable limited-range BT.601 I420; H.264 VUI declares that matrix and range explicitly, avoiding the browser's unspecified-color interpretation.

Each session owns one reusable output frame, I420 encoder buffer and bounded phrase queue. It recomputes mouth/eye patches only when their input states change. Generation checks linearize interruption against final RTP publication, and obsolete encoded video forces the following keyframe. One PCM media epoch supplies audio, visemes, animation sampling and RTP timestamps. Reconnection replaces the peer and requests a fresh keyframe; teardown joins media/control goroutines and releases encoders and session buffers.

## Bounded verification

`export-animation-fixtures.mjs` executes the original TypeScript directors, viseme mapping and acoustic planners using cached speech and explicit recorded nanosecond timestamps. Its 3,281 frames cover ninety seconds of simulated idle (including the seeded double blink), speech, head/gaze behavior, manual smile/brow controls, waiting, toggles, reduced motion and pause/resume. This is a short deterministic computation, not a ninety-second browser benchmark.

The native state tests match the original TypeScript values within approximately 8.9 × 10⁻¹⁶. A second replay crosses the media/scene adapter interface using the same recorded input timestamps; its maximum difference is below 10⁻⁷, including conversion of speech positions to nanosecond durations. Cached PCM acoustic/directed plans and performance-admission cooldowns are also compared with the original TypeScript. Focused adapter checks cover pause-clock separation, preparing each immutable clip once, completion release, interruption invalidation and waiting-smile suppression.

These state tests do not prove pixel equivalence, browser media playback, sustained frame rate or a CPU reduction. Fixed browser/native scene comparisons and a separate-receiver Linux media replay must be recorded before reporting those outcomes. Owner visual review remains pending. A frozen image or dropped animation feature is not an acceptable efficiency improvement.

### Pixels and media

The nineteen fixed Linux browser/native frame states pass the unencoded face gate, with a minimum **54.37 dB** against the required 45 dB. Mouth endpoints and neutral/full-blink eye results are byte-exact over covered pixels. Intermediate mouth/eye sampling differs by small rounding amounts. Independent side-by-side inspection of transitions, teeth, hair, blinks, gaze, expressions and head poses found no new seam, halo or articulation defect. A three-pixel bright boundary dot in the negative-head browser reference is absent from the native rendering; the retained enlarged comparison explains that difference. This internal inspection does not establish owner acceptance.

The fixed final media set is two fresh, sequential runs, each using four seconds of animated idle and 3.5 seconds of cached speech. Both native runs pass every media gate. Speech presentation is **28.81 / 28.51 fps**, p95 frame gaps **50.0 / 50.1 ms**, with zero decoded drops/freezes. Minimum decoded face PSNR is **40.58 dB**, exceeding 40 dB. Output remains H.264, 720 × 960, with actual decoded Opus audio. The offline marker/tone measures video minus audio at **33.0 / 30.9 ms**. Pause advances speech position by zero; interruption advances the generation, and explicit stop leaves zero sessions. Probe peer reconnection takes **409 / 359 ms**.

A one-frame deadline pacer holds early completed encoded frames until the next send deadline. It keeps one frame in flight and preserves the original PCM-clock sampling and RTP timestamp. Cancellation exits the timer; the generation check still rejects obsolete output. This removes render-duration variation from early delivery, at up to one frame of extra send latency. The preceding v5 report's 27.39 presentation fps remains a failed result rather than being discarded.

### Total resources and individual stages

Both workers use the same four-CPU quota/affinity and 1 GiB worker cap on the existing Linux Docker VM; the receiver runs separately with four other logical processors and a 350 MiB cap. No build or other media test overlaps these samples. The qualified uninstrumented Chromium reference retains animated Syntelle and cached speech. These are bounded local measurements, without concurrency sizing, cloud deployment or battery measurements.

| Total worker metric | Qualified Chromium reference | Native fixed runs | Conservative reduction |
| --- | ---: | ---: | ---: |
| Animated idle CPU cores | 1.554 | 0.717 / 0.712 | 53.9% |
| Speech CPU cores | 2.181 | 0.858 / 0.863 | 60.4% |
| Peak memory | 767.0 MiB | 133.3 / 128.2 MiB | 82.6% |

Native peak memory includes the complete marker/pause/interrupt/reconnect/cleanup sequence; steady speech peaks are lower. CPU means actual total cgroup CPU across the worker, including rendering, encoding and transport. Neither worker hits an OOM or CPU-throttling event.

The optional stage probe measures elapsed wall time separately, **not stage CPU accounting**. The browser's asynchronous/multithreaded graphics work can overlap its enclosing draw callback. Native video encoding includes RGB-to-I420 conversion, while the standard Chromium encoder counter has its own internal boundary. These durations must not be added together or subtracted from total CPU.

| Stage wall time per frame | Instrumented Chromium reference | Native fixed runs |
| --- | ---: | ---: |
| Idle frame production | 17.13 ms | 20.68 / 20.56 ms |
| Speech frame production | 18.88 ms | 25.12 / 25.63 ms |
| Idle video encode | 1.47 ms | 2.01 / 2.00 ms |
| Speech video encode | 1.37 ms | 2.27 / 2.23 ms |

The native implementation saves total CPU and memory; it does **not** improve these individual elapsed stage durations. Native Opus encoding takes approximately 0.18 ms per idle block and 0.22 ms per speech block; no equivalent Chromium audio-encode counter is available. The stage-instrumented reference reported approximately 27 presentation callbacks/s despite approximately 30 decoded frames/s, so it remains failed media evidence. Resource acceptance conservatively uses the fully qualified reference with lower CPU consumption, rather than treating that diagnostic run as a qualification.

### Retained corrections

Early native media candidates failed decoded color PSNR because RGB/I420 range and unspecified matrix metadata disagreed. Limited-range conversion plus explicit SPS VUI fixes the mismatch without increasing the 2.5 Mbps target. The earlier pause snapshot advanced by one 20 ms block; entering pause now snapshots the emitted cursor immediately. A probe-only reconnect initially reused its old browser peer; it now follows the real player's fresh-peer recovery behavior.

The real-player proof also exposed an existing Go signaling ownership bug: a reused `Envelope.Data` backing buffer could change after enqueueing when the next ICE message arrived. A fresh envelope per read preserves the queued payload without adding frame/audio copies or changing the public protocol. Failed reports and sanitized transport diagnostics remain alongside the regression and final proof.

The deterministic regression exercises the actual player handler with its worker queue deliberately held: the old code corrupts the first queued signal, and the corrected code preserves both. Streaming unit tests, vet and the focused Linux race test pass. The final canonical API/minimal-runtime/public-Vue proof receives actual H.264/Opus at **29.77 speech fps**, with zero drops, browser errors or private-asset requests. Both injected signaling losses preserve the same session, recover in **647 / 652 ms**, and resume at **29.82 / 29.81 fps**. UI interruption mutes in **41 ms**; already-buffered receiver audio becomes silent in approximately **294 ms**. Provider-fixture counts remain model 0 / cached speech 1 / recognition 0 after both reconnects, and explicit stop leaves zero sessions. The isolated database is removed; fixture containers exit normally without OOM. The cached speech fixture is not an actual provider request.

The raw reports, fixed run plan, stage method, reference/native PNGs, source copies, runtime/library hashes and all rejected intermediate results are retained in `.tools/native-render-56/`. `comparison.json` calculates the acceptance checks; `.tools/native-render-56-archive.json` records the generic PostgreSQL snapshot and complete byte-verified restoration. Original phase-54/55 evidence remains unchanged.

## Offline reproduction

From the repository root, export reference state fixtures using the existing pinned Node/runtime dependencies:

```powershell
.tools/node-v24.22.0-win-x64/node.exe services/render-worker/scripts/export-animation-fixtures.mjs
```

From `services/processor`, create a separate native resource directory from cached source bytes:

```text
uv run --frozen --extra preparation python -m app.workflows.rendering.export_native --output ../../.tools/native-render-56/package-replay
```

From `services/render-worker`, use the cached Go 1.27.2 toolchain for focused state/adapter/render checks:

```text
go test ./internal/nativeanimation ./internal/nativeadapter ./internal/nativerender
```

The Linux media build uses the `nativecodecs` tag and the pinned native encoder dependencies; the portable test build deliberately does not silently select an alternate encoder. In that prepared Linux environment, replay fixed states with `cmd/render-frame --package <private-package> --states <recorded-states.json> --output <new-output-directory> --components`. Compare them with the retained browser reference using `services/render-worker/scripts/review-native-pixels.py --reference <reference-directory> --candidate <new-output-directory> --output <new-report.json>`.

For actual media, run `cmd/render-probe --assets <private-package>` on loopback, or use its explicit unpublished-container-network option. Use the existing bounded receiver with exactly one session and a distinct output report; never publish the probe endpoint. Exact retained Linux build/receiver commands and final artifact paths are recorded with the final evidence for this iteration. Do not run paid speech/model calls or historical motion finalizers to reproduce this change.


## Browser backgrounds (October 22, 2026)

The owner authorized moving background composition and blur to the frontend while
keeping all Syntelle character processing in the backend. The current live worker
sends a 1440 x 960 H.264 frame with premultiplied character RGB and full-size alpha
side by side; the public receiver produces the 720 x 960 presentation and PiP video.
This supersedes the preceding complete-opaque-scene contract. Character source
materials, geometry and animation code remain private. The original bitrate cap,
media clock, audio tracks, provider policy and prepared assets remain unchanged.
See [browser background composition](frontend-background.md) for implementation, rejected checks,
bounded validation and remaining performance/device limits.
