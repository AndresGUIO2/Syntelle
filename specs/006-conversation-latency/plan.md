# Implementation plan

**Status:** authorized implementation scope · **Date:** 2026-10-29

1. Add a socket-owned PCM activity/quiet tracker and conservative incomplete-clause delay. Keep final-only recognition, queued-event priority, echo rules and the existing idle watchdog (REQ-1, REQ-2).
2. Add an optional bounded first-phrase PCM delivery path through the existing Speech SDK, durable operation records and private worker channel. Maintain immutable prefix snapshots, an audio lookahead reserve, ordered completion and cancellation. Preserve complete phrase preparation for ineligible/special delivery (REQ-3, REQ-4).
3. Build from frozen phase116 API and phase114 worker sources plus an explicit file manifest; retain current profiles, frontend and resources. Validate endpointing, partial audio failure, identity/order, playback onset and cancellation with fixtures; document limitations and rollback (REQ-4, REQ-5).

Risks: noise mistaken for speech delays rather than accelerates submission; slow/late recognition cannot authorize interim text; provider stalls require bounded buffering; incremental animation must retain the same audio clock. Tests must exercise these paths rather than only checking constants.

No new dependencies or database schema. All new evidence goes in `.tools/conversation-latency-117/`; historical experiment sources remain immutable. Direct implementation authorization supersedes optional skill approval pauses; unrelated existing working-tree edits are not included in runtime builds or staged commits.
