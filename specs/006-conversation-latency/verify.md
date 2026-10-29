# Verification

**Verdict: PASS** for the bounded local implementation · **Date:** 2026-10-29

| Requirement | Tasks | Evidence from this change | Status |
| --- | --- | --- | --- |
| REQ-1 | T-1, T-3 | Baseline 1,802.13 ms vs candidate 402.46 ms in the same confirmed quiet-turn fixture; `endpoint-baseline.log`, `integration-final.log` | Covered |
| REQ-2 | T-1, T-3 | Sustained PCM without SDK callbacks, delayed final, queued input, first-frame guard, existing segmentation/interim tests | Covered |
| REQ-3 | T-2, T-3 | `browser-v2/report.json`: decoded native Opus before completion gate release; full drain. Native exact-PCM/underflow/duplicate tests | Covered |
| REQ-4 | T-2, T-3 | Durable request/prefix/activation and terminal failure tests; cache reuse; private public-event allowlist; frozen runtime source manifest and unchanged assets/frontend | Covered |
| REQ-5 | T-1, T-3 | Linux API/native race suites, isolated PostgreSQL, cached-only browser proof, installation/rollback receipts; no paid calls | Covered |

API: `go test -race -tags speech ./internal/conversation ./internal/speech ./internal/streaming` and production native Speech SDK build passed (exit 0). Native: `go test -p 2 -race -tags nativegpu ./internal/nativeanimation ./internal/nativeadapter ./internal/nativemedia` and production native codec/GPU build passed (exit 0). Focused database checks finished with exit 0; temporary database inventory was independently zero before fixture removal. Logs retain actual output, failed intermediate builds and the first browser-direction failure. New behavior was not validated by paid services.

No orphan requirements/tasks. Publication isolates this change against the tracked baseline; the shared delivery-coherence helper and its focused regression are the only required earlier prerequisite included. Other existing edits remain unstaged. Separate approvals of arbitrary future speech timing, physical devices, noisy rooms, sustained frame cadence or emotional naturalness are not implied by this verdict. Actual total live-provider latency is unmeasured.

Publication verification: the isolated commit source passes the same focused API and native race suites plus both production builds. Logs are `.tools/conversation-latency-117-git/api-verify-v2.log` and `worker-verify.log`; the initial prerequisite-test selection failure remains preserved. These checks do not replace or redeploy the already verified local runtime. PostgreSQL snapshot 800 restores all 577 original evidence files byte-for-byte, recorded in `.tools/conversation-latency-117-archive.json`.
