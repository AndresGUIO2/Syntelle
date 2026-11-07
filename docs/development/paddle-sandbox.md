# Workspace billing and usage: sandbox candidate

This implementation is **sandbox-only**. The owner has no Paddle sandbox account yet. Isolated PostgreSQL and HTTP fixtures exercise the application boundary; they do not qualify a real Paddle checkout, tax calculation, payment method or subscription lifecycle. There is no production payment mode or development credit bypass.

## Configuration and authority

Configure `PADDLE_API_KEY`, `PADDLE_WEBHOOK_SECRET`, `PADDLE_CLIENT_TOKEN`, `PADDLE_STARTER_PRICE_ID`, `PADDLE_TEAM_PRICE_ID` and `PADDLE_TOPUP_PRICE_ID` only after creating a sandbox account. The API key must have the sandbox prefix, the public client token must start with `test_`, and all three price IDs must be distinct. The API destination is fixed to `https://sandbox-api.paddle.com`; redirects are rejected. Credentials are never returned to the browser or written into request evidence. Only the public client token is exposed for Paddle.js checkout.

The test catalog is Starter USD 10/month with 60 minutes and one concurrent session; Team USD 20/month with 180 minutes and two concurrent sessions; and a USD 5 non-recurring 30-minute top-up. Every external session is limited to 300 seconds. These values are test fixtures, not approved commercial prices. Price amount, currency and recurrence are checked with Paddle before requesting checkout or a plan change.

The existing account service authorizes cookie, same-origin and workspace membership. Only the workspace owner or billing manager can read financial data and initiate checkout, plan changes, cancellation, top-up or payment-method replacement. An administrator does not inherit payment authority. Operational usage is readable by workspace members. Missing configuration disables purchases and external session admission.

## Routes and payment lifecycle

`GET /api/v1/workspaces/{id}/billing` reports configuration, the public sandbox token, subscription, catalog and review status. `GET /api/v1/workspaces/{id}/usage` reports current-period consumed/reserved seconds, available monthly/top-up credit, live session count and per-integration totals.

POST actions under `/billing/checkout`, `/billing/subscription`, `/billing/topup`, `/billing/cancel`, `/billing/payment-method` and `/billing/reconcile` require an `Idempotency-Key`. The server selects the configured price and supplies workspace/action metadata. Clients never provide a customer, subscription or transaction identifier. Checkout returns a transaction ID for the Paddle.js sandbox overlay. Payment-method replacement requests a transaction for that workspace's subscription; it does not expose a payer-wide portal.

An action and complete normalized input are committed before the first provider request. Each provider request records method, relative path, exact body and exact bounded response bytes, excluding authentication headers. The same action key cannot issue a second request. A known catalog/rejection failure is retained; an ambiguous mutation blocks further purchases until an authenticated result resolves it. There are no automatic mutation retries. Explicit reconciliation reads only server-bound subscriptions and unpaid known transaction IDs; a lost response with no provider identifier or matching webhook remains unresolved rather than inventing a successful result.

Only a completed, recognized, paid transaction grants credit. Subscription notifications alone cannot grant minutes. An immediate upgrade retains existing rights until its completed prorated payment grants the additional portion of the current period. A downgrade changes the next billed price while current application rights remain through the paid period. Cancellation is scheduled for period end. New monthly grants expire with their paid period; top-ups are retained and consumed after monthly credit. Future-period transactions never grant early access.

## Webhooks and reconciliation

`POST /api/v1/billing/paddle/webhook` verifies the exact raw body using HMAC-SHA256 over `timestamp:body`, accepts multiple `h1` values for secret rotation, and enforces five-second timestamp freshness. The endpoint acknowledges only after retaining the immutable event bytes. Reusing an event ID with different bytes fails. Processing is separately transactional and idempotent by both event ID and paid transaction ID.

The durable inbox preserves pending dependencies, rejection reasons and attempt timestamps. Subscription event timestamps prevent an older activation from overriding a newer cancellation or past-due state. Unmatched adjustments can wait for their transaction binding. Approved refunds or disputes freeze the affected grant and place the workspace in review, stopping new/renewed usage until an explicit future reconciliation policy resolves it. No automatic refund, credit purchase or negative balance is generated.

## Server-metered usage

Admission locks one workspace row, reaps expired leases, verifies its active paid period, review/suspension state and published integration, enforces workspace concurrency and the integration's configured daily cap, then reserves up to 300 seconds from valid credits. Concurrent requests cannot overspend or exceed the plan's slots. The daily limit uses UTC and counts outstanding reservations; a session crossing midnight is counted conservatively until settlement.

`ReserveIntegrationUsage` runs before allocating a worker. `StartIntegrationUsage` starts charging when the worker becomes ready. `RenewIntegrationUsage` runs every ten seconds and rechecks entitlement and integration revocation. `ReleaseIntegrationUsage` settles consumption and returns unused reservation on every termination path, including startup failure. Leases expire after thirty seconds without renewal and cannot outlive the reserved session or paid period. Crash cleanup charges only through that lease deadline. Billing uses PostgreSQL's clock; browser clocks, message counts, microphone recordings and Paddle.js success callbacks cannot grant or measure paid usage.

The ledger records grant, reservation, consumption, release and freeze entries. Session/grant allocation rows prevent double consumption and double return. Releasing an unstarted session costs zero. Session identifiers cannot be reused after release. The current usage screen can trail a running session by one renewal interval.

## Verification and actual-account prerequisite

Run the focused package tests with the repository's pinned Go toolchain. Integration tests require `SYNTELLE_BILLING_TEST_DATABASE_URL` whose database name starts with `syntelle_billing_test_`; tests refuse any other database before mutation. They apply the account migrations, including additive billing migration 006, to that isolated database. No tests contact Paddle: an injected transport verifies the fixed sandbox destination and returns complete provider fixtures.

Before claiming live sandbox qualification, configure the new account, create the exact test prices, configure the signed webhook URL, then verify checkout, duplicate notifications, upgrade payment, scheduled downgrade/cancellation, renewal, top-up and subscription-specific payment-method checkout with Paddle test payments. Browser checkout success is not proof that entitlements were committed. Preserve the sandbox request/event records and compare application grants and usage with provider transactions. Production enablement, commercial terms and real charges require a separate approved scope.

Official documentation consulted for this implementation:

- [Create a transaction](https://developer.paddle.com/api-reference/transactions/create-transaction/): automatic collection, items and custom data.
- [Update a subscription](https://developer.paddle.com/api-reference/subscriptions/update-subscription/): complete item replacement, proration and payment-failure behavior.
- [Create a subscription payment-method transaction](https://developer.paddle.com/api-reference/subscriptions/get-subscription-update-payment-method-transaction/): subscription-specific checkout without a shared payer portal.
- [Verify webhook signatures](https://developer.paddle.com/webhooks/about/signature-verification/): exact-body HMAC, timestamp and multiple signature values.

Actual sandbox responses remain necessary to qualify the configured catalog and provider lifecycle; fixtures are explicitly separate evidence.
