# Local development commands

Run from the repository root in PowerShell. `dev.cmd` uses the bundled Node 24.22 runtime when present, otherwise Node on PATH. Other shells can call `node scripts/development/dev.mjs <command>`.

```powershell
.\dev start
```

This starts Docker Desktop's Linux engine if necessary, then the existing PostgreSQL preparation profile, processor, API, web and native render worker. It checks service readiness and returns without keeping a terminal or host background Node process alive. Open http://127.0.0.1:5173/. Docker Desktop remains responsible for running the containers.

Existing private root `.env` configuration, Docker Desktop with its CLI, the prepared `.tools/native-render-56/package/manifest.json` and its cached resources are prerequisites. The worker never generates resources or changes provider credentials. Missing images are built automatically; prepared assets and account/provider configuration are never invented. Docker engine startup is bounded; unsupported Desktop startup reports an actionable failure. Status, cleanup and stop do not start Docker.

| Command | Behavior |
| --- | --- |
| `.\dev start` | Reuse current images and a healthy worker; build only missing images. |
| `.\dev start --build` | Explicitly rebuild stable tags, then start/reconcile services. |
| `.\dev stop` | Stop the owned worker and Compose services; retain all volumes and images. |
| `.\dev status` | Show Compose services and renderer readiness/active count. |
| `.\dev build` | Build current source without restarting services. |
| `.\dev clean --dry-run` | Show recognized disposable Syntelle containers/images. |
| `.\dev clean` | Remove those stopped test/build containers and unreferenced project images. |
| `.\dev clean --cache` | Also trim unused shared Docker build cache to 2 GB. |

Ordinary startup deliberately does not rebuild on every run. After source/dependency changes use `start --build`; rebuilding or downloading dependencies can take time. The current installed native image remains selected until an explicit build replaces its stable tag. The command does not promote a historical candidate by tag or alter rendering settings.

Local Compose explicitly sets `SYNTELLE_STREAM_SESSION_LIMIT_ENABLED=false`, allowing several tabs from the same visitor up to the renderer's global capacity. Outside this local configuration the per-visitor cap remains enabled by default. Request/connection controls and the [30-second/two-minute message inactivity policy](stream-inactivity.md) still apply locally.

It also sets `SYNTELLE_STREAM_TIME_LIMITS_ENABLED=false`, disabling the five-minute session and 15-minute accumulated demo caps during local development. Local sessions do not reserve or consume production demo allowance. Production defaults to enforcing the caps when this explicit exception is absent; no prior usage is reset. The initial static preview uses the selected existing background without opening a render session. See [stream demo limits](stream-demo-limits.md).

## Ownership, failure and retention

The root command serializes start/stop/build/cleanup through a PID lock in ignored `.tools/dev/`. A stale lock from an exited process is recovered; concurrent invocations fail clearly instead of racing. A healthy renderer in the current API container namespace is reused. API replacement causes only the recognized local worker to be replaced. An unrelated container occupying its name or a separately connected worker is left unchanged with an explicit error.

The native launcher adds `--detach` for background operation. Startup validates that the new worker stays alive and connects; failure stops only that new worker. The container uses `--rm`, readonly assets/filesystem, existing resource bounds and bounded Docker logs. Compose services retain their stable identities; each runtime container keeps at most two 5 MB JSON log files. `dev stop` stops rendering before the API's existing five-second deadline and does not remove databases.

Builds reuse four stable Syntelle runtime image tags. Image labels and a local ledger of known runtime image IDs identify superseded untagged images, including older images created before labels existed. Cleanup excludes any image referenced by a retained container and uses neither force nor volume deletion. Recognized historical Syntelle compiler/test images and their stopped containers are disposable; other project images/containers and all volumes are retained. An explicit old reference tag is not treated as an untagged image.

After actual builds, including failures, cleanup trims unused **shared** Docker build cache to 2 GB and removes eligible Syntelle leftovers. This may slow later builds of any project, but does not delete its images, containers or data. Startup with existing images performs only selective Syntelle cleanup. Direct arbitrary Docker commands are outside this wrapper; they can still create resources. Resource cleanup does not replace immutable experiment records: PostgreSQL snapshots, source evidence, generated assets and `.tools/` history remain intact.

Cleanup decisions/timestamps and the image ownership ledger are in `.tools/dev/`; no private environment, OAuth token, provider key or connection ticket is written there. Errors expose operation/status without dumping Docker environment diagnostics. The initial phase-59 cleanup evidence is `.tools/dev-workflow-59/`, with generic PostgreSQL archival and byte-verified restoration.

## Bounded verification

```powershell
.tools/node-v24.22.0-win-x64/node.exe --test scripts/development/dev.test.mjs services/render-worker/tests/start-local.test.mjs
```

Focused fixtures cover foreign-resource protection, active image references, repeated startup, API namespace replacement, detached failure cleanup and bounded runtime flags. Live checks exercise stop/start, repeated start with unchanged container IDs, HTTP/renderer readiness and received idle video without paid requests. This is development lifecycle verification, not a new rendering-performance or physical-device qualification.
