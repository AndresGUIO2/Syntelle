# Local account-owned character creation

The local character creation candidate replaces the coming-soon page with a private upload/library workflow. Open **Menu → Characters**, sign in, choose a portrait and name, and submit. A queued or processing item cannot open. A ready item opens its own named conversation, with its source-bound native package; it never silently becomes Syntelle.

## Current capabilities and limits

Creation prepares mouth articulation, eye closure, foreground/depth, head projection, gaze and basic expression fields. New basic packages do not inherit Syntelle's emotional companions, laughter or identity. Emma remains the speech voice. Unsupported controls are unavailable, while ordinary speech, blink, head movement and basic supported gestures remain usable. An original portrait with different framing is hidden atomically on the first composed native frame instead of crossfading into a double face.

The full new-source preparation is connected. On November4 the owner authorized and enabled a separate initial campaign for one new portrait, capped at9 image/9 analysis attempts, with no retries, until November12 at00:00UTC. It is consumed only after a new portrait is submitted; no live request was made while enabling it. Existing exact cached portraits consume none of this allowance. Without an active campaign an unknown source fails explicitly. A development replay does not qualify arbitrary future uploads. See [private preparation](../characters/creation-workflow.md) for the policy format, local models, request bounds and source constraints.

One account can have one active preparation and finish one new character per rolling24hours. Failures release that success allowance; deleting a finished character does not reset it. The queue has a global bound. Original uploads expire after24hours; native runtime resources remain private until the last owning creation is deleted. After the original expires, the library displays initials and the stage waits for its private video; the UI never loads Syntelle as a substitute thumbnail.

## Access and runtime

The existing account service owns `/api/v1/characters` and its authenticated detail/portrait/delete operations. The browser submits a name and image, not a package hash. The API resolves an owned ready identifier to its immutable native package. Streams, continuations and pause/resume retain that identity. Another account receives not-found for private detail, portrait and playback. Deletion immediately closes only that owner's matching streams and prevents new admission. Private characters cannot be copied into shared workspace settings.

Local startup reads these private `.env` settings:

```dotenv
SYNTELLE_CHARACTER_PACKAGES=.tools/private-characters/packages
SYNTELLE_CHARACTER_SCRATCH=.tools/private-characters/work
SYNTELLE_CHARACTER_CHECKER=.tools/character-creation-review/render-frame.exe
# Optional only after finite generation authorization:
# SYNTELLE_CHARACTER_APPROVAL=<private campaign JSON path>
```

`dev start`, `dev stop` and `dev status` manage the hidden preparation consumer as well as the existing stack. Process identity is checked before reuse or shutdown. Native startup reads the same package root and mounts it read-only; no geometry/material package is served to browsers. Keep isolated test package roots separate from the live account database's package root.

## Review evidence

The candidate is frozen phase122 native source plus scoped package/session changes; its renderer, shaders and animation source remain byte-identical. Current API/account and frontend work is preserved. The source manifest, image IDs, build/race results, captured browser failures, successful real-account route, cached speech and short comparison recording are under `.tools/character-creation-123/`.

Offline verification includes complete source preparation replay from18 historical provider responses, eight focused preparation/database cases, Go account/stream integration and race checks, frontend build boundaries and desktop/mobile browser checks. Actual normal-account verification uses the image validator, PostgreSQL queue, native checker, GPU/NVENC video, original cached Emma PCM and the ordinary conversation/preflight path. The second account cannot access the first face. Browser authentication uses the actual email-code handlers with captured fixture delivery, not mocked account responses. It does not send email or record microphone input.

Initial browser attempts exposed an isolated-port origin mismatch; production origin rules were not weakened. Visual review also caught the custom photo/native crossfade overlap, fixed separately from Syntelle's matched-poster arrival. Passing playback and ownership checks is technical acceptance, not unseen-portrait quality, new vocal capability or owner naturalness approval.

Install with `.tools/character-creation-123/install.ps1` after the recorded checks. Roll back with `.tools/character-creation-123/rollback.ps1`; this restores phase122 images and stops the creation consumer while retaining created records/files. It does not erase customer data or automatically downgrade the database. `app.audits.finalize_character_creation` archives known development evidence and verifies restored bytes. Customer photos/request bodies are excluded from permanent experiment snapshots.
