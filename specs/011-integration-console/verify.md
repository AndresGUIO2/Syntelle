# Verification

| Requirement | Evidence | Result |
| --- | --- | --- |
| REQ-1/2: console hierarchy and responsive design | `console-review.md`, desktop/mobile screenshots, browser navigation, resizer, role and language fixtures | Passed local review; owner visual acceptance remains open |
| REQ-3: immutable workspace publication | accounts isolated PostgreSQL tests, custom worker-packet assertion, processor package retention regression | Passed |
| REQ-4: scoped access and privacy | real API/fixture worker tests, command/response isolation, membership/revision/revocation cases | Passed |
| REQ-5: inline/floating website player | `widget-review.md`, separate-origin Chromium desktop/mobile and WebKit controls, capability/close/start checks | Passed fixtures; configured public network qualification pending |
| REQ-6: sandbox lifecycle and usage | billing15 top-level tests +22 subcases, canonical provider fixtures, exact fixture DB restoration | Passed offline; actual Paddle account pending |
| REQ-7: race and regression checks | Linux Go race (accounts/streaming/billing), Go vet, production player build, scoped lint, existing media lifecycle regressions | Passed |

Evidence root is `.tools/integration-console-126/`. Initial compilation, fixture assertion and provider-runner setup failures are retained alongside passing corrections. Runtime installation and the final PostgreSQL archive receipt are separate outputs; neither fixtures nor an installed UI establish production payment or external-network media qualification.

The local candidate is installed and its web/API routes, guest sign-in boundary, speech/recognition configuration and preserved native worker are verified. PostgreSQL source/evidence restoration passed byte-for-byte; the external `.tools/integration-console-126-archive.json` receipt identifies the final snapshot. Rollback remains explicit and preserves user data.

The renderer image, prepared character package, Emma voice, oral articulation and approved laughter are preserved. No new paid provider requests or cloud deployment occur. The follow-up character-creation UX redesign is deferred at the owner's request until this delivery is complete.
