# Protected local demo builds

Syntelle has two explicit static review profiles. Both retain the existing Vue character renderer, motion planning, coordinates, speech playback and three current characters. There is no geometry streaming, new API endpoint, frontend server layer or deployment. MediaPipe remains part of private Python preparation.

Normal development remains readable and keeps its laboratory URLs. The existing `npm run build` still produces the image-validation application without the animated demo or prepared resources. The demo profiles ignore development URL switches, including `trial`, `character`, `head`, `view`, `idle` and `expressions`.

## Build and preview

Use the repository's Node 24.22.0 runtime rather than the older system Node:

```powershell
Set-Location apps/web
$syntelleRuntime = (Resolve-Path ../../.tools/node-v24.22.0-win-x64).Path
$env:PATH = "$syntelleRuntime;$env:PATH"
npm run build:demo:baseline
npm run build:demo:protected
npm run preview:demo:protected
```

Outputs are `apps/web/dist-demo-baseline/` and `apps/web/dist-demo-protected/`. The baseline is minified without obfuscation; normal `npm run dev` is the readable source workflow. Neither review profile emits source maps.

Preview listens only on loopback, defaults to port 5173 and reuses the existing API/WebSocket proxies and backend origin restrictions. An occupied port fails explicitly; the command does not stop the existing development server or choose another port. Build first: preview verifies every output against the private build report and refuses a modified or incomplete distribution. Vite preview is for local review, not public hosting.

## Distribution boundary

Build-only code replaces the demo catalog with an explicit registry of Syntelle, Café, Woodland and the three existing backgrounds. Per-character runtime definitions remain lazy-loaded. The registry emits only referenced runtime images and geometry, using hashed material filenames and logical resource names rather than repository paths. Brand assets and third-party notices are explicitly included.

Recursive manifest allowlists retain runtime geometry, transforms, motion settings, source/build bindings, checksums and validation flags. They omit full landmark arrays, calibration records, algorithm inventories, generation inputs, review annotations, diagnostics and historical comparison references. Missing required fields or nonfinite numeric values fail the build. A failed validation flag is retained, never converted into approval.

Original prepared JSON, images, geometry and frozen evidence are not rewritten. Emission checks material hashes against the source bytes. Existing browser checksum, build-binding and geometry checks still run.

The protected profile uses the local API of pinned `javascript-obfuscator@5.9.0` on 26 executable character engine modules. Engine chunks exclude Vue, interface components and the microphone AudioWorklet. The deterministic profile renames identifiers and uses lightweight string-array indirection, with no string encryption, control-flow flattening, injected dead code, property renaming, anti-debug timers or self-defending code. Transformation occurs before final chunk hashing.

Build audits reject laboratory/private modules, source maps, private paths, known credential patterns, unexpected resources, altered materials and engine modules that bypass protection. Public notices and font bytes are checked against their local dependency sources. Detailed module inventories, source paths and hashes remain outside both distributions in `.tools/client-protection-31/`. The text scanner is a targeted guard, not a general secret-detection guarantee.

## Measurements and verification

Both profiles contain the same 145 deduplicated runtime materials and 164 output files. Initial measured sizes:

| Entire distribution | Baseline | Protected |
| --- | ---: | ---: |
| Uncompressed bytes | 15,706,252 | 15,711,519 |
| Sum of individually gzip-compressed files | 13,872,634 | 13,876,759 |
| JavaScript gzip bytes | 237,250 | 241,379 |

Obfuscation adds about 4.0 KiB of compressed JavaScript across the whole distribution. These gzip totals describe compression potential, not a claim that local Vite preview sends gzip or that opening one character downloads every file. Raw preparation manifests total 6,127,798 bytes; runtime projections serialize to 1,046,018 bytes before bundling. That difference includes whitespace as well as removed evidence.

```powershell
npm run test:demo:build
npm run review:demo -- --smoke-only
npm run review:demo -- --performance-only
```

The build tests check all three projections, geometry equivalence, missing checksums, nonfinite coordinates, preservation of failed validation, the engine boundary and deterministic ESM obfuscation. Browser review checks character switching, live movement/gaze, expressions, customization, Chat/Read aloud, cache/cancellation, reduced motion, unavailable services, ignored laboratory switches and rejected checksum/build mismatches. It intercepts HTTP and WebSocket fixtures, disables preview API proxies and blocks unexpected requests, making zero provider calls.

Browser review uses separate loopback fixture ports 5181/5182, without changing owner-service origins. Performance runs are serial on Syntelle in Chromium and WebKit, with a five-second warm-up and twenty-second sample per build. They compare frame callbacks and actual body draw submissions, request counts/types and API paths. Acceptance requires no more than 5% frame-rate loss and no more than 2 ms additional p95 frame interval. If a result is noisy, retain it and repeat the affected pair once; do not silently weaken the gate. Windows WebKit lacks Web Audio, so its rendering comparison uses idle motion and typed Chat rather than claiming audio playback coverage.

Latest browser/performance results and screenshots are in the private evidence directory. Preliminary harness failures remain preserved: the first gaze assertion sampled before the existing quantization threshold, the error assertion initially expected an internal message instead of the workspace's generic failure, and the mismatch fixture initially assumed quoted strings rather than the minifier's template literals. These were review-harness corrections; character preparation and animation algorithms were unchanged.

The initial bounded review passes all six build tests, scoped lint/format checks, type checking, all three builds and the focused browser checks. All 590 original experiment files retain their hashes. Two consecutive protected builds produce the same 164 file hashes (distribution digest `f0a413e6d4a02f60f48624803b605cecbf46f6a76b39db9340ec4e38696b43eb`). Preview also rejects an occupied default port, non-loopback binding and modified distributions.

| Short headless sample | Baseline | Protected |
| --- | ---: | ---: |
| Chromium callback rate | 60.003/s | 60.002/s |
| Chromium callback p95 interval | 16.8 ms | 16.8 ms |
| Chromium body draw frames in 20 seconds | 801 | 801 |
| Chromium requests | 67 | 67 |
| WebKit callback rate | 63.963/s | 63.675/s |
| WebKit callback p95 interval | 16 ms | 16 ms |
| WebKit body draw frames in 20 seconds | 640 | 637 |
| WebKit requests | 70 | 70 |

Both browser pairs pass the relative gates, with identical request-type counts and API paths. The cold Syntelle sample records 10,868,196 asset-encoded bytes for baseline and 10,872,321 for protected, a 4,125-byte increase. Headless callback rates are scheduling measurements, not physical presentation rates: the separate body draw counts are lower, so these results do not establish a 60 fps renderer. The protected build adds no recurring animation request.

## Limits

Obfuscation increases analysis effort. Browser-delivered geometry, materials, shaders and behavior remain recoverable. It adds no authentication, encryption guarantee or protection against copying visible results. Provider credentials and private preparation remain outside browser artifacts.

This does not establish owner visual acceptance, public-upload readiness, real-device acceptance or sustained 60 fps. Existing backend contracts, PostgreSQL schemas, provider allowances and all historical evidence remain unchanged.
