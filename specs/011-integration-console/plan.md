# Implementation plan

## Boundaries

- Console/navigation components own destinations, responsive layout and account/workspace selection. Feature components own character/publication, integration, usage and billing forms; shared composables guard stale account/workspace requests.
- Existing StreamWorkspace retains private Test/history behavior. RemotePlayer and useRemoteStream remain the single media implementation.
- WidgetHost owns public metadata, parent-origin handshake, challenge and lifecycle UI. A typed optional transport adapts useRemoteStream to scoped session capabilities without changing its default account behavior. The loader owns only inline/floating placement and verified parent messages.
- Accounts owns publication/integration resources and workspace key authorization. Streaming owns visitor/session capability admission and command restrictions. Billing owns Paddle communication, durable events and usage reservations. Existing native rendering is unchanged.

## Public additions

- Workspace publications/integrations and billing/usage routes under `/api/v1/workspaces/{id}`.
- Public widget metadata/bootstrap/session routes under `/api/v1/widgets/{id}`; hosted `/embed/{id}` and versioned `/widget/v1.js` loader.
- Existing stream endpoints accept integration-scoped authority and session-only widget capabilities. Private rendering instructions never appear in widget responses.
- Additive PostgreSQL migrations005(publications/integrations/grants) and006(billing/events/usage). Existing account keys and data retain ownership.

## Verification and rollout

Freeze phase125 before edits. Use isolated PostgreSQL, cached media/provider fixtures and separate browser origins. Validate console and widget presentation before local installation. Paddle real sandbox test requires a new owner account and secure environment configuration. Record fixture vs external checks separately. Preserve rollback images and immutable package; do not broadly rebuild native worker sources.
