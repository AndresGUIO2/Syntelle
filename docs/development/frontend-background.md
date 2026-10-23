# Browser background composition

The owner authorized moving background processing to the frontend on October 22,
2026. Syntelle's animation, speech analysis, oral/eye composition, gaze, expressions,
RGB-D deformation and character rasterization remain in the private Go worker.
The public receiver combines already rendered character pixels with a background;
it receives no character source textures, geometry, landmarks or animation code.

## Media contract

The live worker emits `premultiplied-alpha-sbs/1`: a single 1440 × 960 H.264 frame
contains 720 × 960 premultiplied character RGB on the left and matching full-size
grayscale alpha on the right. Encoding both halves together preserves their frame
identity through loss, decode and reconnection. The original 2.5 Mbps cap and 30 fps
target remain unchanged. Packed transport negotiates H.264 level 4.0. This is twice
the encoded pixel area, not a demonstrated reduction in bandwidth or total CPU.

`nativeadapter.LoadForeground` loads character resources without loading background
textures. The packed renderer neither resizes/blurs backgrounds nor samples them
per frame. Its bilinear character placement and premultiplied coverage remain the
same. `nativerender.Load`, `nativeadapter.Load`, and 720 × 960 opaque output remain
available for offline reference tools, not as a public playback fallback.

The native `ready` event includes `video_layout`, transport dimensions, and
`display_width: 720` / `display_height: 960`. The receiver checks actual decoded
dimensions and reports an outdated worker instead of exposing the packed halves.
Update the web image and native worker together; old clients cannot display the
new layout correctly.

## Browser responsibilities

- `StreamWorkspace.vue` owns preferences and passes background/blur to the player.
  Background-only edits update locally without a backend configure command. The
  existing settings schema still stores those preferences for saved configurations.
- `RemotePlayer.vue` retains sound, suppression, autoplay handling and PiP controls.
- `useBackgroundVideo.ts` owns the muted source decoder, output capture track,
  background loading and cleanup. Original incoming audio tracks are passed through
  unchanged, including tracks that arrive after video. Upstream owns those tracks;
  this composable stops only its own capture track.
- `backgroundCompositor.ts` owns one background-only WebGL compositor. Background
  cover fitting and cached separable Gaussian blur run on preference changes. Each
  decoded source frame contributes rendered RGB plus alpha to an opaque 720 × 960
  presentation canvas. Manual capture requests publish each completed draw; using
  a separate automatic 30 fps capture gate lost frames and was rejected. Browsers
  without manual requests use a 60 fps capture ceiling, bounded by source redraws.

The visible video and its PiP window use this same composed capture stream, with
the original audio tracks. No microphone or provider request is replayed on a
background change. The three authorized full background images have independent
public copies in `src/assets/backgrounds/`; the explicit development/build boundary
allows only those images and receiver modules. Private character resources remain
blocked. There is no new model, segmentation, dependency, provider call or schema.

## Bounded verification and limits

Go tests check fractional coverage reconstruction against the preceding compositor,
transparent borders and cancellation. Three actual prepared states cover neutral,
speech/half-blink and head-turn/full-blink; changing server background settings
does not change any packed byte, and no background texture is loaded. These checks
reuse immutable cached resources.

Browser checks cover opaque pixel preservation, fractional hair coverage, background
blur, local-only edits, existing sound/visibility/PiP controls, and an actual moving
packed-pixel fixture with original audio-track identity. Chromium and Windows WebKit
verify composition math; Windows WebKit has no actual MediaStream/canvas capture,
so that full playback fixture is explicitly skipped there.

A real Windows Chromium → local API → Linux OpenH264 worker idle check passes at
approximately 29.6 presented fps, 50 ms p95 frame gaps and zero decoded drops. It
changes background/blur, preserves mute preference and opens actual PiP. Its
screenshot was inspected. This is a brief idle functional check, not sustained
speech sync/quality, real Safari/iPhone, OS background scheduling, concurrency,
energy or backend CPU savings validation. No paid speech was requested.

Evidence and rejected first checks are preserved in `.tools/frontend-background-65/`.
The previous local image is `syntelle-render-native:before-frontend-background-65`;
the new image is `syntelle-render-native:frontend-background-65` and selected locally
through `syntelle-render-native:local`. Use the generic PostgreSQL archive/restore
workflow for this evidence, never a historical motion or oral finalizer.
