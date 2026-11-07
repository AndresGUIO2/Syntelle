# Application navigation

## Current integration console (phase126)

The owner superseded the conversation-led phase125 design. Characters and external integrations are now the primary product. Overview, Characters, Integrations, Usage and Billing share the resizable workspace console; Team and Settings are secondary. Test conversations and their history live under the selected character, without a global sidebar chat list. See [integration console](integration-console.md) for current routes, publication ownership, embedding, payment boundaries and rollback. The phase124/125 sections below are preserved historical evidence.

## Current phase125 correction

The owner rejected the modal history and bare workspace creation. Recent conversations now live in the sidebar as flat, date-grouped rows, with paginated older conversations, full-title tooltips and explicit guest-save/delete actions. The list has its own bounded scroll area, leaving account and preferences reachable. Collapsed navigation expands to the same list; mobile uses the existing drawer.

Selecting a conversation pauses the currently playing session, loads only the selected transcript/settings, and presents it without allocating audio, a microphone or a renderer. **Continue conversation** deliberately sends the original conversation ID. If detail loading fails, the prior transcript and persisted paused stream remain resumable. Older messages load inside the transcript. Account changes, rapid selection and navigation away invalidate late results; private route panels are keyed by account identity. Missing saved settings produce an explicit unavailable state rather than silently substituting a character.

Pricing is public guest navigation. Signed-in accounts do not see that destination; authenticated `/pricing` entry returns to the retained conversation. Existing workspace Billing and account management remain available without inventing an active subscription product.

Workspace entry uses a focused name form with help, privacy context, busy state and recoverable errors. Existing workspaces use a selector and a New workspace dialog. Permissions, invitations and ownership transfer are unchanged. Shared navigation spacing is denser, redundant section branding/back links are removed, and character replies use an open transcript surface instead of stacked bordered bubbles.

Phase125 changes web only. API/native image hashes remain phase124-exact. Review evidence, build, installation and rollback live in `.tools/navigation-125/`. Restore the preceding web interface with:

```powershell
& .tools/navigation-125/rollback.ps1
```

Archive the current evidence with `python -m app.audits.finalize_navigation --experiment navigation-125 --parent-snapshot 844`; do not run the historical default against current sources. The external archive receipt verifies all restored bytes. No provider requests or cloud deployment are involved. The existing cross-tab server Continue/Delete atomicity is outside this frontend correction; this UI serializes its own pending history actions but does not claim a new server concurrency guarantee.

## Preserved phase124 baseline

The phase124 local candidate replaces the generic header menu with a shared application shell. Conversation, Characters, History and Workspaces are primary destinations; Pricing and eligible Administration are secondary. Account, language and theme remain available from every section. The sidebar is 240 px expanded or 72 px collapsed, defaulting to expanded from 1440 px and compact from 1024 px. Narrower layouts use a modal drawer with a 320 px maximum. Desktop preference is local and optional.

`/characters` is the private collection; `/characters/new` opens preparation. Guest entry opens sign-in at the requested route and returns to that same destination. Existing auth callbacks, workspace/admin query routes, invitations and ownership transfers remain supported. The existing Vite/static SPA fallback serves the new route; deployment must continue using its application fallback rather than a 404 document.

One shell-owned account authority supplies the account overlay, history and chat. Account access does not redirect or allocate media. Private account resources are invalidated when ownership changes and late requests cannot repopulate another account's data. Loading a saved setup deliberately opens its character; media requires an explicit Start.

Cross-section history/setup actions wait for the mounted chat handle, not only its downloaded module. This prevents WebKit from dropping an action during the first asynchronous chat mount. A route, character, account or chat-generation change cancels pending delivery of that action.

## Conversation continuity

Only the current chat is cached. Navigating away immediately stops microphone tracks and audible output, cancels pending starts/previews/reconnects, closes customization and picture-in-picture, and suspends the authenticated stream. The existing command channel accepts `suspend`; the API stops work, releases the renderer and commits state `paused` with reason `navigation` before its terminal `paused` notice. Existing `resume_from` validates principal, character, package and language and restores the conversation. No endpoint or table is added.

Returning restores the transcript and scroll position with **Continue conversation**. It does not start media automatically. Missing acknowledgement, persistence failure, expired access or deleted characters remain explicit errors; a cached transcript is never evidence of server continuation. New conversation, character replacement and account identity changes clear the corresponding cached state. Opening navigation itself does not interrupt playback.

## Local review and rollback

The candidate uses `syntelle-api:navigation-124` and `syntelle-web:navigation-124`. The phase123 native image and existing prepared package are unchanged. Evidence, original image IDs, build logs, fixture screenshots and installation receipt live in `.tools/navigation-124/`. Restore the previous API/web with:

```powershell
& .tools/navigation-124/rollback.ps1
```

The rollback preserves databases, created characters, volumes and the creation consumer. It reconnects the same native renderer to the restored API namespace. No cloud deployment or paid provider requests belong to this change. Browser protocol fixtures do not establish physical-device audio qualification. Owner visual acceptance remains separate.
