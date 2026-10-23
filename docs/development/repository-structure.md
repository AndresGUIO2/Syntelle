# Repository structure

Current Go ownership: [independent API and renderer modules](renderer-service-boundary.md).

The source is grouped by responsibility. The monorepo keeps the existing Vue, Go and independent FastAPI services; this reorganization adds no runtime library, infrastructure or product behavior.

The public frontend is a remote player. The only private render worker is native Go; its Node scripts launch/build that image and export/check offline fixtures. The former Node/Chromium renderer and private Vite entry are retired. Browser character modules retained below support source-bound preparation and deterministic comparisons, not a second runtime or public rendering route. Playwright/Chromium remains for player and media-receiver tests. Historical browser implementations remain in immutable evidence snapshots.

```text
apps/web/
  config/                         Development compatibility aliases
  src/
    components/                   Product workspace UI
    composables/                  Product upload behavior
    features/character/
      animation/                  Timing, head motion and speech-context behavior
      speech/                     Native-viseme timeline and audio decoding
      rendering/                  WebGL and oral/eye composition
      composables/                Asset loading, playback and renderer lifecycle
      components/                 Shared mouth and speech controls
      types/                      Package contracts and explicit contexts
    experiments/                  Local trials, catalogs and prepared browser assets
    stores/                       Product state
    types/                        Product-level contracts
  scripts/                        Offline browser review/export utilities
  tests/                          Browser and API verification

services/
  api/
    cmd/api/                      Go entry point, configuration and wiring
    internal/httpapi/             Validation HTTP, responses, local access and lifecycle
    internal/conversation/        Conversation sockets, model adapters and persistence
    internal/speech/              Native/stub voice, ledgers and conversation adapters
  render-worker/
    cmd/                          Worker, offline media probe and frame exporter
    internal/                     Animation, clock, frame, rendering and media engines
    go.mod, go.sum                Independent worker module
    native.Dockerfile             Minimal pinned native runtime
    scripts/                      Native launcher, build and fixture/receiver checks
    tests/                        Launcher and offline verification tests
  processor/
    app/
      main.py                     Image-file validation HTTP service
      core/                       Shared local paths
      workflows/
        visemes/                  Original generation and package preparation
        dentition/                Selection, completion and dental repair trials
        contours/                 Recorded Gemini/Argon contour experiments
        oral/                     Semantic composition and validation
        corpus/                   Multi-character preparation and visual review
        foreground/               Automatic foreground refinement
        motion/                   Eyes/depth, head rigs and contextual settings
        speech/                   Local Speech verification helpers
      audits/                     Archive, finalization and restoration commands
      preparation/                Source-hashed vision/composition algorithms
      compat/                     Historical command/import adapters
    tests/                        Processor checks

packages/render-contract/          Shared settings and private wire types; no service dependency

docs/
  README.md                       Documentation entry point
  product/                        Scope, brand and character requirements
  design/                         Design guidance
  development/                    Setup, organization and skill records
  preparation/                    Database/evidence operations
  experiments/oral/               Mouth/dental experiment records
  experiments/motion/             Eyes, foreground, movement and speech records
  characters/                     Stable character configurations
  prompts/                        Protected original prompts
  design-references/              Protected original HTML designs

specs/                            MVP specification and technical plan
scripts/development/              Background startup, selective cleanup and lifecycle checks
dev.cmd                           Thin Windows entry for the development commands
.tools/                           Ignored runtimes, models, caches and evidence
```

## Dependency rules

- Product views compose UI and receive explicit data; reusable character code does not import experiment catalogs or read URL selections.
- Rendering owns geometry and pixels. Speech owns audio/viseme timing. Animation planning proposes movement; composables connect these pieces and manage disposal.
- Processor workflows orchestrate local preparation; `preparation/` contains the existing source-hashed algorithms. Those algorithm bytes were preserved to keep their cache bindings intact.
- `app.main` stays lightweight and does not initialize models, providers or the development database on startup.
- Go uses the concrete command/HTTP/conversation/speech boundaries in [Go service organization](go-service.md), with tests beside their implementations. Do not add handler/service/repository layers solely to create folders.
- Configuration, cached/generated assets, original prompts and archived evidence are not moved into reusable runtime modules.

## Commands and compatibility

New documentation should prefer the grouped module names, for example:

```powershell
# From services/processor; these examples only display command help.
.venv/Scripts/python.exe -m app.workflows.motion.prepare_contextual_motion --help
.venv/Scripts/python.exe -m app.workflows.corpus.corpus_prepare --help
.venv/Scripts/python.exe -m app.audits.archive_preparation --help
```

Do not execute a preparation/finalization command merely to test its import: some historical commands generate paid resources or archive frozen experiments. Existing authorization, request ceilings and stopped-provider rules remain in force.

Old `python -m app.<command>` invocations and Python imports still resolve through `app/compat/`. They delegate to the same canonical module object so test patches and process-local state are shared. The package's search path includes that explicit compatibility directory; there is no global import hook or automatic workflow discovery.

Old development browser URLs for the seven moved character modules are retained through explicit Vite aliases in `apps/web/config/legacy-module-paths.ts`. Current imports use the new folders. The assets and experiment URLs keep their existing paths.

The relocation index is [relocations.json](relocations.json). Historical `.tools/` snapshots remain byte-exact and retain their original paths. Source-hash provenance on existing packages refers to those frozen preparation sources; relocating live source does not silently rewrite package IDs or request/cache bindings. A new explicit preparation records its current source paths/hashes normally.

## Working conventions

- Add new workflow commands inside their feature folder, not directly under `app/` or `compat/`.
- Add new documentation to its topic folder and link current entry points from `docs/README.md`.
- Keep shared character modules inside the appropriate feature subfolder. Avoid a broad barrel file that pulls experiment assets into production.
- Keep one canonical implementation; compatibility modules contain redirects only.
- Ignore generated caches in the editor instead of deleting evidence. Workspace editor settings hide dependency/build caches, while `.tools/` stays available for intentional inspection.

Verification records and source snapshots for this change live in `.tools/organization-16/`; complete evidence is archived/restored separately in PostgreSQL. No provider request, database migration, image alteration or animation behavior change belongs to this reorganization. Functional browser review must continue to use the existing geometry/texture invariants rather than approving newly captured images by baseline replacement alone.

The cleanup moves 64 existing files: 30 processor command/audit modules, seven shared character modules and 27 documentation files. The processor package root now contains only `__init__.py` and `main.py`; the documentation root contains its entry-point README. Verification passes 160 processor tests and 104 Chromium/WebKit desktop/mobile/API scenarios, including historical command/module compatibility. All 589 protected algorithm, prompt, configuration and experiment-resource files match their original hashes. All 192 deterministic exported frames match the preceding decoded pixels exactly. Frontend formatting, lint, type checking and build pass. No physical-device or new performance approval is implied by this structural cleanup.
