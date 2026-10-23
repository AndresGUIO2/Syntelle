# Blinking, motion and background candidate

This is a local development candidate for Syntelle, cafe and woodland. Open `http://127.0.0.1:5173/?trial=motion&character=syntelle` and use the character links. The preceding mouth trial remains available and links to this candidate. Neither the production bundle nor public upload preparation was expanded.

## Resources and requests

The owner supplied Light Studio, Dark Lounge and Natural Space. Their original PNG bytes are retained in `.tools/motion-09/backgrounds/originals/`; browser images and thumbnails are lossless WebP. External generation settings were not supplied and are not invented. Background selection and blur (0–20 CSS pixels, initially zero) are entirely local. Blur applies to the background layer only; coverage includes overscan to avoid transparent blur edges.

The separate PostgreSQL budget is `authorized-motion-development-2026-10-19`, bound to `docs/characters/motion-development.json`: at most six images and zero contour analyses. Actual usage is **three successful image requests**, one per character, with no corrections or transport retries. Original source bytes are the first image in every request; an automatically measured square face crop is the second. The current model is `gemini-nano-banana-2.1`, 1K square. Raw responses are JPEG; extracted layers are exported to PNG and lossless WebP. Existing viseme budgets, source generations and oral packages remain unchanged.

Syntelle's complete blink and geometric intermediates were inspected before preparing the other two characters. The browser gate records the eye-resource hash and its evidence. Each character retains its exact prompt, JSON request/response, usage, timestamps, original input, raw output, alignment, detected geometry, eye masks and exports. `docs/prompts/closed-eyes-v1.txt` is the reusable prompt template. No manually drawn masks or character-specific placement coordinates were introduced.

## Automatic preparation

`app.prepare_motion` accepts a manifest referencing an immutable original and its existing oral package. Source hashes must agree. Stable MediaPipe face anchors exclude moving eye and mouth points. The closed-eye result is aligned to the original, then only the automatically derived eye edit masks are used. Original RGB is preserved exactly outside those masks.

Open eye interiors remain at their original coordinates. Moving upper/lower coverage reveals or hides that original iris and pupil; it does not compress or dissolve them. Eyelid skin is mapped separately from a rigid strip around the lash line. Closed source texture supplies the endpoint. The skin textures interpolate locally; this is a restrained portrait blink, not a reconstructed anatomical eyelid. Fine crease/lash texture can still differ at magnified intermediate positions and needs owner review.

Foreground preparation uses the pinned MediaPipe SelfieMulticlass model, its six semantic classes, a semantic GrabCut trimap, guided edge coverage and fractional-edge color decontamination. Interior opaque RGB stays unchanged. The connected component owning the detected nose excludes disconnected furniture. Light, dark and checkerboard comparisons are retained; hair wisps, woodland ears and clothing were inspected. This remains portrait matting, with possible loss of very fine translucent strands.

Relative depth uses the official Hugging Face conversion of **Depth Anything V2 Small**, CPU, with the reference shortest input side 518 (aspect-preserving multiples of 14). Revision is `5426e4f0f36572d16453bbda7a8389317b1bef99`; all configuration and weight SHA-256 hashes are pinned. Torch bicubic upsampling is recorded rather than described as the repository's exact OpenCV implementation. Small is the Apache-2.0 model; larger model variants are not substituted. Final recorded inference was about 1.45–1.57 seconds per image with two CPU threads. The preparation process's peak working set was about 913 MB; this is a process-lifetime peak, not a per-image incremental allocation. No model runs during browser playback.

Segmentation and depth caches bind source RGB, algorithm hash and model hashes; every output file is checked before reuse. Changed cached bytes cause an explicit failure. Replaying all three generation stages without `--request` reused the original responses and left the paid count at three.

## Rendering and controls

The motion package is optional and references its oral package build hash. A separate background catalog contains no character or provider configuration. Eye and mouth patches are composed onto one transparent character texture before deformation. The full texture uploads once; expression changes upload small rectangular regions. Autonomous movement changes geometry uniforms, not full-resolution textures.

The 64×64 mesh uses cached depth and detected face anatomy. Bounds are yaw 1°, pitch/roll 0.6° and torso breathing of 0.6% of face height over five seconds. Breathing has zero influence on the face and tapers to zero at the bottom crop boundary. All 81 combinations of bounded movement extrema have positive triangle orientation for each character (663,552 triangle checks per package).

Blink timing is 80 ms closing, 40 ms closed and 140 ms opening, with seeded random 4–7 second intervals. The trial provides independent blink/movement toggles, pause/resume, Blink once, manual eyelid closure, the original mouth pose controls and a persistent background/blur preference. Reduced motion disables autonomous playback but permits manual still inspection. Visibility suspension stops the animation clock and mouth playback; resuming starts from the suspended clock without including hidden time. Resources and listeners are released on disposal, and graphics failure is shown explicitly with a link to the preceding trial.

WebKit exposed a graphics-context exhaustion bug during repeated pose switching. The candidate now rebinds mouth textures and geometry on the existing compositor. All 81 pose selections reuse exactly three contexts (body, eyes, mouth), with identical endpoints regardless of source pose. The preceding compositor's rendering equations are unchanged.

## Verification and limits

- 144 processor tests passed, including PostgreSQL integration tests, the independent six-image motion allowance, exact original request bytes, prompt/configuration persistence, failed-request recovery without retries, cache integrity, fixed exposed iris pixels, exact closed coverage, outside-mask preservation, dark opaque foreground and bounded mesh geometry.
- Frontend type checking and production build passed. Development experiments remain excluded from the production bundle.
- The final **68 Chromium/WebKit desktop/mobile checks passed**, covering the motion candidate, current oral corpus and preserved completed package. They include 81-pair compositor reuse and deterministic visibility suspension. Background switching/blur leaves the paused character canvas byte-identical. Reduced motion and explicit graphics failure were checked. The complete result is `browser-final-checks-68.json`; earlier 44/8-check reports remain as history.
- Browser exports cover 1,215 combinations per character: all 81 source/target pairs, five mouth positions (including the reported 35% and 41% frames) and open/half/closed eyelids. Eye-mask exterior RGB, exposed original eye pixels, closed source interior and cross-pair endpoint deltas were all zero. Native-resolution neutral character rendering matches the foreground within one channel value. Display downsampling uses GPU linear filtering and is not compared to Canvas2D's different area filter.
- CPU masks, browser blink sequences/contact sheets, endpoint images, representative maximum motion frames and the three-background/checkerboard review remain in `.tools/motion-09/`. Automated invariants support inspection; they do not constitute owner visual approval.

Local rendering measurements are stored separately in `performance-chromium.json` and `performance-webkit.json`. These use active mouth playback, blinking/movement and maximum background blur. Chromium measured about 60 fps. WebKit measured about 56–57 fps for Syntelle and 62 fps for cafe/woodland on desktop; the strict 60 fps desktop target is therefore still unmet for Syntelle in this local WebKit run. Mobile viewport measurements exceeded 60 fps on the same desktop hardware and do not establish physical-device performance. GPU-completion timings for eye updates and deformation are also recorded by the export command. **Real-iPhone testing remains required for launch.** The candidate stays explicitly selectable; it is not promoted over the old default from these measurements.

The complete source/prompt/configuration snapshot, model files, actual preparation bytes, browser evidence and request usage are archived in PostgreSQL by `app.finalize_motion`. It restores the complete evidence snapshot into a separate directory and verifies every file hash. See `.tools/motion-09/audit.json` and `.tools/motion-09-postgres-restore.json`. No credentials enter those records or browser assets. No public API or database schema was added.

The old demonstration is preserved. This candidate does not complete owner acceptance, held-out character validation, public preparation, speech, gaze tracking or cloud deployment.

## Cached reproduction

With PostgreSQL and Vite running, from `services/processor`:

```powershell
uv run --frozen --extra preparation python -m app.prepare_motion --config ../../docs/characters/motion-development.json --output ../../.tools/motion-09 --cases syntelle cafe woodland --phase generate
```

Without `--request`, that command can only reuse matching received outputs. Free stages use the same arguments with `--phase eyes`, `segment`, `depth`, `package` or `install`, in that order. Changing the original or oral package requires a separate preparation version; do not overwrite evidence or reset the budget.

From `apps/web`:

```powershell
node scripts/export-motion-review.mjs syntelle
node scripts/export-motion-review.mjs cafe
node scripts/export-motion-review.mjs woodland
npx playwright test motion.spec.ts corpus-oral.spec.ts --workers=1
node scripts/measure-motion.mjs
node scripts/measure-motion.mjs --webkit
npm run build
```

Finally, from `services/processor`, `uv run --frozen --extra preparation python -m app.finalize_motion` audits and archives the evidence without provider requests.

References: [MediaPipe Image Segmenter](https://developers.google.com/edge/mediapipe/solutions/vision/image_segmenter), [MediaPipe Face Landmarker](https://developers.google.com/edge/mediapipe/solutions/vision/face_landmarker), [Depth Anything V2](https://github.com/DepthAnything/Depth-Anything-V2).
