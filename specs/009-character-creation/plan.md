# Implementation plan

**Status:** implementation authorized by the owner's complete-creation choice.

1. **REQ-1, REQ-2:** Reuse Vue navigation/account authentication. Replace the coming-soon page with an owned library, upload form and durable status polling; use a ready-only explicit Open action.
2. **REQ-2, REQ-5:** Add the account-scoped PostgreSQL creation ledger and authenticated portrait endpoints to the existing Go account service. Keep database ownership, idempotency and admission authoritative.
3. **REQ-3, REQ-5:** Add `app.workflows.characters` as a private queue consumer. Reuse the existing preparation workflows and cached exact-source catalog. Publish atomically by validated content hash and keep provider authorization explicit and finite.
4. **REQ-4:** Extend private native package resolution, session package binding and bounded character identifiers. Standard packages receive supported speech/animation only; preserve the default Syntelle profile.
5. **REQ-6:** Build frozen phase122 native source plus explicit loader/session patches. Run focused tests and real local regular-account fixture playback. Archive source/input/result records and byte-verified restoration. Document limitations and rollback.

## Risks addressed

- UI success cannot stand in for native source identity or visual acceptance.
- A profile with Syntelle-only resources must not accompany a custom face.
- Provider mistakes are retained and terminal; a budget is not permission to retry.
- A client-supplied package path/hash must never authorize access.
- The working native tree contains rejected rendering experiments: only scoped changes enter the preserved phase122 build.
