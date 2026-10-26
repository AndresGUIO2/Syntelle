# Natural speech gestures

Phase 90 improves the existing nod, shake, head tilt, emphasis and bilateral brow cues in the local native player. It adds no gesture vocabulary, images, face masks, models or provider requests. Prepared emotion, oral, gaze, foreground and dental resources remain unchanged.

## Behavior

The original phase-90 `performance-intent/3` used 820 ms nod cycles and 1,160 ms shake cycles; phase 95 below supersedes those durations. The existing quintic easing follows an asymmetric stroke and return. The shake's second excursion is 82% of the first. Tilt briefly holds before returning; emphasis is a smaller, shorter nod. Automatic repetitions receive deterministic, slight timing/strength variation and a stable initial turn direction derived from the text and cue index. They never exceed the admitted window, repetition count or prepared range. Manual previews retain the user's exact intensity without this variation.

The previous 600 ms phrase-entry blend attenuated early gestures a second time. The new path samples each gesture directly on emitted PCM time while releasing the previous phrase's residual over 240 ms. It also removes a second smoothing filter from new bilateral brow accents. Brow entry and phrase boundaries ease for 120 ms; interruption and automatic-expression disable release for 240 ms. Reduced motion immediately shows the manual baseline. Pause still freezes animation time.

The director continues producing dialogue and optional semantic cues in one response. Prompt guidance favors sparse gestures on meaningful points: agreement/disagreement for nod/shake, an occasional questioning tilt and a brief emphasis/brow accent. Application validation retains the existing anchor, amount, count, spacing, cancellation and geometry limits. No model-provided geometry is admitted.

The API aligns the approximate gesture peak with the existing acoustic anchor and records `audio-energy/2-approximate-gesture-peaks`. This remains approximate text/acoustic timing, not word timestamps. A cue must fit its phrase; an oversized optional sequence is omitted without losing spoken text. Protocols 1 and 2 keep their preceding binding, curves, duration constants and timing evidence. Manual preview busy timers match the new cycles.

## Resources and verification

The unchanged package is `.tools/emotion-readability-87/package-v3/`, hash `184576c2856c20038d3d573f6f10eb90029abade6918addbdbf90bb59801840b`, with the same source-bound profile and mask corrections. Its prepared bounds remain yaw 3 degrees, pitch 4 degrees and roll 1.2 degrees. Existing complete resources remain preserved in PostgreSQL snapshot 640; parent source/UI state is snapshot 642.

Evidence and reproduction scripts are in `.tools/speech-gestures-90/`:

- Native and API builds pass their tests, including deterministic trajectories, first-phrase strength, acoustic peak alignment, interruption, pause, reduced motion, manual controls and legacy compatibility. Focused native race checks pass.
- Nineteen recorded states at 720 and 1,440 pixels produce 38 byte-identical CPU/CUDA comparisons, with exact fallback. These include resting, MBP, FV and open-vowel combinations with brows and head gestures.
- The isolated PostgreSQL player fixture replays eight alternating directed/quiet phrases using cached 1,500 ms PCM. The completed measured run and a separate 22-frame visual run pass. Actual two-cycle manual nod/shake and tilt previews pass. No microphone or real provider is used.
- Across all fixture runs, including two stopped harness checks, counters record four fake model responses, 32 cached speech preparations and zero recognition calls. The first harness incorrectly expected a completion event per phrase rather than per reply; the second passed speech/cadence checks but used malformed `.85` range-input text. Both failures remain archived.
- The measured Chromium run presents 363 frames at 29.88 fps, with a 50 ms p95 gap and a 66.7 ms maximum gap. No screenshots or recording run during that measurement. A preceding run measured 29.47 fps with the same p95. The strict p95 below 50 ms requirement remains unqualified; this does not establish a general stutter fix or device performance approval.
- Four focused Chromium/WebKit control fixtures, Vue type checking, scoped lint, player/production builds and six production-boundary checks pass. Actual native playback is checked in Chromium; WebKit fixtures do not establish native audio/device qualification.

Normal-size player samples and focused oral/face comparisons were inspected internally. No new mouth/cheek seam was identified in this bounded check. Owner visual acceptance, expression readability across characters and physical-device qualification remain pending.

Initial build failures are retained. The API's ordinary dependency download failed on Docker DNS; the successful local recipe reused cached modules and the already cached Speech SDK. A Windows node_modules bind and an incorrect web build context also failed before the successful cached Linux web build. No new dependency or SDK installation was needed.

## Activation and rollback

The verified images are activated as `syntelle-render-native:local`, `syntelle-api:latest` and `syntelle-web:latest`. The worker retains `syntelle-render-native-local`; the normal entry remains `.\dev start` at `http://127.0.0.1:5173/`. Package/profile selection and credentials are unchanged. The installed check uses gesture preview only, with microphone and conversational/synthesis commands forbidden.

`activation.json` records image identities and the three `before-speech-gestures-90` rollback tags. To roll back, restore those tags to the stable image names, restart the owned native worker and run the normal launcher. For source rollback, use `before-source/` and the explicit introduced-file list; do not reset unrelated working-tree changes. The initial post-edit native admission backup is retained separately as rejected backup evidence; the correct preceding source was recovered byte-for-byte from phase 86.

The generic PostgreSQL archive stores sources, cached inputs, comparisons, runtime records and failed checks. `.tools/speech-gestures-90-archive.json` records the final snapshot and a complete byte-verified restoration. Historical preparation finalizers are not used.

## Faster nods and shakes (phase 95)

The owner found both gestures much too slow. The current shared natural-motion timing is 420 ms per nod and 580 ms per complete shake, approximately twice the preceding speed. Speech binding, approximate acoustic peak alignment, native admission and manual previews use the same duration constants. Frontend preview busy timers use the new durations plus the existing 200 ms control margin.

The curves, repetition variation, semantic allowances, amplitude and prepared geometry remain unchanged. First peaks occur at approximately 143 ms for nods and 139 ms for shakes before automatic repetition variation. Protocols 1/2 retain their historical speech constants and behavior. Emotion regulation, masks, oral/contact constraints, backgrounds and Ava synthesis are unchanged. This is gesture timing, not a frame-rate correction.

Bounded verification uses `.tools/gesture-tempo-95/`:

- Shared/API/native builds and focused animation, adapter, media and conversation tests pass. Numeric preview checks verify both opposing shake excursions before 410 ms and baseline restoration by 650 ms. Continuous trajectories remain inside prepared bounds and advance on the frame clock. The initial cadence test incorrectly treated the already completed fourth nod as a frozen gesture; its retained failure and corrected active-window sampling are included.
- Eight Chromium/WebKit desktop/mobile control checks pass, including reduced motion and microphone-free previews. Vue type checking, scoped lint, player/production builds and six production-boundary checks pass.
- An isolated PostgreSQL fixture completes one eight-phrase reply using cached PCM: one fake director response, eight cached speech preparations, zero recognition and zero real provider calls. Manual two-cycle nod/shake previews also pass. A separate frame-callback capture inspects both complete manual sequences without paid services.
- The bounded player check presents 356 frames at 29.26 fps, with a 50 ms p95 gap and a 100.03 ms maximum gap while separate control fixtures also run. These measurements do not qualify the strict frame-gap goal or sustained/device performance. Visual capture is separate from this measurement.

The active resource remains `.tools/disgust-readability-93/package-v6/`, package hash `e203544db20918f95c96fb545831ba187934e04fed24b132cd982d51a41e54e9`, preserved completely in parent snapshot 646. No resources, shader or rasterization changes require a new pixel-comparison matrix. Previous CPU/CUDA and mask evidence remain applicable to those unchanged paths; owner visual acceptance and physical-device qualification remain pending.

Stable image/container names and the normal development launcher are retained. `.tools/gesture-tempo-95/activation.json` records the image identities and `before-gesture-tempo-95` rollback tags. Restore those tags to their stable names and run `.\dev start` to roll back. The generic archive record `.tools/gesture-tempo-95-archive.json` records source/evidence preservation and byte-verified restoration; no historical finalizer is used. No paid calls, generations, dependencies, model installations, schema changes or cloud deployment are involved.
