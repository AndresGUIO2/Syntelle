# Character code organization

The later repository-wide cleanup is documented in [repository structure](repository-structure.md). Shared animation, speech and package contracts now have dedicated subfolders, Python commands are grouped into workflows/audits, and the documentation has topic folders. The implementation notes below describe the preceding phase-10 refactor.

This behavior-preserving refactor keeps experiment assets and compatibility URLs in place. Reusable rendering lives in `apps/web/src/features/character`; it does not select characters from the browser URL or import experiment catalogs.

## Component and data-flow map

- `App` selects the development trial and passes its query selection as typed props.
- `MotionTrial` composes the stage, controls and detail preview. Its package context is explicit.
- `MotionStage` displays the character canvas above the independently blurred background; props only.
- `MotionControls` edits background, blur and animation toggles through model events; blink and closure use explicit events.
- `MouthControls` edits source/target selections and emits play/scrub/endpoint actions.
- `VisemeTrial` retains the historical mouth inspection layout. Its renderer lifecycle belongs to a composable.

Composables own asynchronous loading, saved preferences, clocks and WebGL lifecycle. Plain TypeScript rendering modules own geometry and pixels. Three persistent contexts and partial texture uploads remain required. Compatibility modules re-export shared implementations; old assets and source prompts are not moved.

## Verification

Before foreground replacement, the refactored compositors produced **147 byte-identical decoded images** (49 per character) against the preceding recorded exports. These include mouth endpoints, reported intermediate frames, eyelid closure and deterministic movement samples. All 76 refactor checks passed across Chromium/WebKit desktop/mobile; independent contexts, late-load disposal, three-context reuse and frame suspension were included. Reports are in `.tools/organization-10/`.

The Go lifecycle now waits for active requests, force-closes connections after the existing five-second grace period and returns startup/shutdown errors. Synchronization-based tests cover draining, deadline expiry and startup failure. Go tests, vet and formatting checks pass. The small flat package remains appropriate; no new service layers or runtime libraries were added.

ESLint and Prettier are pinned development tools. Run `npm run lint`, `npm run format:check` and `npm run build` from `apps/web`. Generated manifests, binary assets, source prompts and archived evidence are excluded. Node 24.22.0 is used for verification; the system Node is older than the project engine requirement. The production build remains limited to the upload workspace: character trials, models and experiment assets are excluded.

Foreground matting is a separate versioned preparation change, not a rendering baseline update; see `docs/experiments/motion/foreground-refinement.md`. Browser scripts accept `SYNTELLE_REVIEW_OUTPUT` to record new review/performance runs without overwriting previous experiments.

The installed foregrounds passed the motion checks on all four browser/viewport projects. Expanded coverage exercised 140 scenarios: eight initial upload failures occurred while the local backends were stopped, and one exposed a changed image-load error message. The backends were started, the friendly error restored, and all nine affected checks passed on targeted rerun. Together with the 76-check refactor suite (40 overlapping motion checks), **176 distinct scenarios have successful results**; the initial failures remain recorded rather than erased. Processor verification finished at **149 passing tests**, including PostgreSQL integration. The pre-existing Starlette/HTTPX deprecation warning remains outside this change.

Before/after measurements use the same local desktop hardware, active mouth/blink/movement and maximum background blur. Chromium remains approximately 60 fps; WebKit desktop Syntelle measured 53.8 fps before and 57.4 afterward, with cafe/woodland around 61.6/62.6 afterward. These short runs show no observed regression, not a causal performance improvement claim. Syntelle still misses the strict 60 fps WebKit target. Mobile viewport measurements remain desktop emulation, not physical iPhone validation.

The new complete evidence and source snapshots are archived/restored separately with `app.finalize_refinement`; restore records are in `.tools/refinement-10-postgres-restore.json`. No provider request was added: PostgreSQL counts remain three motion images and eighteen images/twenty-one analyses in the separate oral-validation budget.
