# Private animated character creation

**Status:** approved scope — owner explicitly selected complete animated character creation, then requested implementation.

## Requirements

- **REQ-1 — Navigation.** Signed-in users can find Chat, Characters, Workspaces and their account from one accessible, responsive menu. Guests signing in from Characters return there without starting the microphone or playback.
- **REQ-2 — Owned creations.** A regular account can upload one named JPEG, PNG or WebP portrait, track a durable preparation job and open a ready character. Validate actual bytes, enforce the existing 5 MB input limit, scope every read/write to the owner and retain idempotency across retries. Pending and failed jobs must never appear playable.
- **REQ-3 — Complete preparation.** Use immutable original bytes, the shared oral/eye/foreground/depth/head/gaze/expression pipeline and native package export. Reuse exact matching cached inputs. Novel inputs require a finite, separate operator generation campaign; reserve complete inputs before provider calls, retain failures and make no transport/provider retries. Historical budgets are not reset or reused.
- **REQ-4 — Private playback.** Resolve an owned ready character on the server to its immutable package hash. Bind native sessions, continuations and resumed streams to that package. Load materials only through the private worker filesystem; never expose prepared packages to the browser or substitute Syntelle for a failed custom load. Capabilities follow the package; unsupported emotional resources and laughter remain unavailable.
- **REQ-5 — Privacy and limits.** One active job per account, one successfully created character per rolling24hours, bounded global queue. Delete source bytes after24hours; remove a deleted creation from access immediately. Preserve consumption/idempotency tombstones, erase unneeded private package storage when no active owner references it. Do not copy private characters into workspace shared settings.
- **REQ-6 — Verification and delivery.** Exercise normal-account upload, actual preparation, native playback and cross-account denial with an isolated PostgreSQL fixture and cached provider/audio inputs. Verify desktop/mobile menus, build boundaries and source/package identity. Preserve phase122 runtime and rollback. Distinguish offline technical verification from unseen-photo/live-provider and owner naturalness qualification.

## Scope

Local review delivery; no cloud deployment, new model or dependency. Existing Syntelle mouth, laughter, motion and audio paths remain preserved. The requested feature authorizes implementation and local tests, not an unbounded provider budget. Creation names are character identity labels; do not infer age, gender or biography from a photo.
