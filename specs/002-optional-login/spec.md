# Optional Syntelle accounts

Status: local implementation candidate, October 21, 2026. The original discovery requirements are retained below; the bounded implementation and setup gates are recorded in [Local accounts and rendered streaming](../../docs/development/accounts-and-streaming.md).

## Current local implementation

- Select `/?view=stream` for Syntelle-only accounts, saved configurations and integration keys. The previous default at `/`, Café/Woodland and technical trials remain preserved.
- Optional Google/GitHub Authorization Code Flow with PKCE and passwordless Resend email codes share server-owned sessions. Google adds OIDC nonce/token validation. New social connections require account/email proof and explicit consent; matching emails do not silently merge accounts.
- Use explicit Save to retain only the current Syntelle configuration. Saved setups remain until deleted; temporary guest configurations expire after 24 hours. The later account-history extension saves signed-in conversations and offers explicit guest-chat claiming.
- The local implementation uses separate PostgreSQL ownership/configuration/key records. Ordinary accounts do not inherit owner privileges, historical evidence or provider allowances.
- Real Google/GitHub sign-in requires OAuth credentials; email requires `RESEND_API_KEY`, explicit verified `RESEND_FROM` and the private auth secret. Missing configuration leaves those methods unavailable. Fixture tests establish implementation behavior, not actual provider/email delivery or public launch acceptance.
- Current source, failed checks and verification records belong in `.tools/accounts-streaming-49/`. The linked development record is the current endpoint/setup reference.

## Confirmed product decision

The owner confirmed that signing in is optional. Visitors can talk to Syntelle to understand the product and start creating without an account. A login screen must not replace the public entry experience or interrupt those journeys.

This specification extends the public-access model in [the original MVP](../001-syntelle-demo/spec.md). It does not silently replace RF01 or claim that the current local owner demo is a secured public service.

The owner selected saved characters and settings across devices, plus Syntelle API keys for external integrations. The first account scope excluded conversation history; the later account-history extension adds it with 90-day retention after the last user turn.

The owner also confirmed rendered live video and synchronized audio as the API output. See the separate [character streaming API specification](../003-character-streaming-api/spec.md); this expands the MVP beyond a login screen.

## Design reference

Preserve the [owner's original HTML](../../docs/design/login/reference-original.html) without edits. SHA-256: a6f777cb9854b16db236717d34bc76c6b6d537aaad45bdac54b04e1e53fcb23e.

Use its centered, approximately 440 px card, short introduction and prominent social-provider actions as composition references. Google and GitHub were confirmed alongside email codes; actual provider configuration and delivery remain unverified.

Adaptation requirements:

- Retain Syntelle's existing DM Sans, graphite/violet tokens and light/dark preferences. The reference's cyan palette and alternate fonts do not constitute a rebrand.
- Reuse the actual Colors Studio assets and a typographic Syntelle wordmark. The waveform symbol is a mockup proposal, not an approved product logo.
- Provide a clear, persistent way to continue without signing in.
- Prefer one "Continue with Google/GitHub" flow if sign-in and registration have identical behavior. Separate Sign In / Sign Up tabs need a meaningful behavioral difference, not duplicated copy switches.
- Remove unsupported "End-to-end encrypted neural session" and invented "v2.4 Online" claims. Do not imply that social sign-in verifies the operational status of speech/model services.
- Terms and privacy links must lead to actual approved documents before release. Placeholder links are not accepted production behavior.
- Keep the card usable on short/mobile screens through normal scrolling. Do not copy global scrollbar hiding or viewport overflow suppression.
- Preserve visible keyboard focus, clear loading/error labels, minimum 44 px action targets and reduced-motion support.

## Proposed user journeys

### Continue as a visitor

Open Syntelle, talk to the public character, explore and start creation. Authentication is optional throughout. Public usage and cost policies remain separate product decisions; "optional login" does not grant the local owner's uncapped provider access.

### Sign in from the workspace

1. Choose Sign in from an available account action.
2. Pause/stop active microphone capture, pending replies and audio before leaving for the identity provider.
3. Select a configured provider and complete its normal authorization flow.
4. Return to the intended Syntelle workspace and restore the relevant local draft/context.
5. Require an explicit action to re-enable the microphone. Do not restart a conversation or incur a model request merely because authentication completed.

Authentication must not discard guest creation work. The local account flow saves the current Syntelle configuration only through explicit Save after sign-in. It does not silently import unrelated local owner experiments, conversations or diagnostics.

### Use account resources

An authenticated user can restore their saved characters/settings on another device and manage integration keys under Developer access. Library restoration and API access must use server-side ownership, not a browser's character/session identifiers. Signing out of the website does not implicitly revoke separately issued integration keys; explicit revocation and account disablement control those keys.

### Cancel or fail sign-in

Cancellation returns to a usable visitor experience. Provider refusal, unavailable configuration, network failure and expired authorization each have a clear recoverable state. Retrying sign-in must not duplicate a character job or conversation turn.

### Sign out

End the authenticated session, capture, active replies and protected connections. Return to the visitor experience. Signing out does not delete an account or its stored resources; account deletion and retention must be specified separately if account storage is selected.

## Functional requirements

| ID | Requirement |
| --- | --- |
| AUTH-01 | Public conversation and creation entry remain available without an account. |
| AUTH-02 | Sign in can be reached without replacing the main public experience. |
| AUTH-03 | Successful sign-in returns to the intended workspace without silently starting audio/capture. |
| AUTH-04 | Cancelling or failing sign-in preserves usable guest access and local work. |
| AUTH-05 | Show an authenticated account state and an explicit sign-out action. |
| AUTH-06 | Ordinary accounts receive no owner/admin privileges or uncapped owner provider access. |
| AUTH-07 | If resources are account-owned, enforce ownership in Go for reads, writes, audio/history and WebSockets; browser identifiers alone do not authorize access. |
| AUTH-08 | Never expose provider credentials, authentication secrets or private owner evidence to the browser. |
| AUTH-09 | An expired session has an explicit recovery path and cannot expose another account's resources. |
| AUTH-10 | Support both themes, keyboard operation, mobile scrolling and Safari-compatible provider return behavior. |
| AUTH-11 | Save and restore account-owned prepared characters/settings across devices. |
| AUTH-12 | Provide API-key creation, identification and revocation for the streaming API, without exposing provider credentials or owner-only privileges. |

## Boundaries

- Vue remains on Cloudflare; Go remains the API/session authority on Azure. FastAPI stays an internal image processor.
- Keep the local owner workflow, existing assets, paid-provider ledgers and historical allowances intact.
- Local provider/library choices, seven-day browser sessions, proof-based identity linking and canonical callback configuration are documented in the linked implementation record. Public domain/deployment configuration remains separate.
- Authentication does not complete public-upload validation, public provider access controls, account retention policy or deployment.
- No passwords, billing plans, organizations, role editor or social-profile import are assumed. Conversation history belongs to the later bounded extension.

## Historical discovery decisions

1. Define retention/deletion for saved rendering resources, temporary uploads and account metadata.
2. Define how the current guest character/settings are claimed by an account.
3. Confirm Google/GitHub for launch and define provider-account linking behavior.

These discovery questions are settled for the local Syntelle-only candidate by the current implementation section. Public retention/account deletion, deployment and launch-provider acceptance remain separate work.
