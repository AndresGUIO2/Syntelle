# PostgreSQL preparation archive

The owner selected PostgreSQL for preparation records. Local development uses the official PostgreSQL 18 container, a named `preparation-db` volume and a loopback-only port at `127.0.0.1:5433`. The existing host service on port 5432 is untouched. No hosted database or cloud resource was provisioned. This local setup is separate from future production persistence and retention decisions.

## Stored evidence

`app.preparation.archive` uses Psycopg 3 and explicit SQL without an ORM. Its versioned schema is `syntelle_preparation`; definitions live in `archive.sql` alongside the implementation. Every snapshot contains the actual bytes of images, prompts, configuration, provider requests/responses, masks, landmarks, transformations, exports, review evidence and code/version records. SHA-256-addressed blobs are shared across snapshots to avoid storing the same file repeatedly. Snapshots preserve preceding versions instead of overwriting them.

The archive also contains experiments, immutable events, request budgets and per-attempt status/usage. The recorded provider commits its input snapshot and request reservation before sending a POST. Concurrent reservations lock the same budget row. Received requests reuse cached results; failed or unresolved attempts cannot be retried automatically. A provider failure halts both image and analysis work in that batch. Database unavailability prevents a new provider request. A database failure after sending leaves a conservative started reservation for reconciliation rather than allowing another potentially billable call.

The current four-portrait authorization is recorded in `docs/characters/oral-validation-batch.json`: at most 32 image requests and 36 analyses, no transport retries. Earlier cached experiments do not consume this new allowance. Their original records, including rejected results and usage, are archived as historical snapshots.

Credentials stay in ignored `.env` configuration. Configure `SYNTELLE_DATABASE_URL` and `SYNTELLE_DB_PASSWORD`; do not put either in prompts, browser variables, snapshots or documentation. Archive ingestion rejects `.env` files and known credential values. The local container requires its password; no trust-authentication fallback is used.

## Local operation

From the repository root:

```powershell
docker compose --profile preparation up -d postgres
```

From `services/processor`, inspect the database or archive a run without paid calls:

```powershell
uv run --frozen --extra preparation python -m app.archive_preparation
uv run --frozen --extra preparation python -m app.archive_preparation --directory ../../.tools/oral-repair-07 --experiment oral-repair-07
```

Restore an archived snapshot into a new folder:

```powershell
uv run --frozen --extra preparation python -m app.archive_preparation --restore SNAPSHOT_ID --destination ../../.tools/restored-run
```

Restoration verifies each blob hash, rejects escaping paths and refuses to overwrite different working files. Local files are a working cache, not the sole copy of archived evidence. The named Docker volume preserves PostgreSQL data when its container stops; deleting that volume deletes the database and is not a cleanup step.

## Verified migration

The initial historical import archived 6,935 file references from 25 experiment/documentation snapshots, deduplicated into 3,028 blobs. This includes the original visemes, Argon analyses, rejected dental experiments, semantic preparation, current correction and documentation. Later preparation runs and snapshots increase these counts. The import report is `.tools/postgres-migration.json`.

A real eight-file FV experiment was restored from PostgreSQL into a separate folder and verified byte-for-byte against its working originals. Evidence is `.tools/postgres-restore-verification.json`. Integration tests use isolated temporary PostgreSQL schemas and verify restoration, immutable snapshots, content deduplication, secret rejection, durable pre-request reservations, concurrent limits, exhausted budgets and stopping after provider errors. They do not send paid requests. Without PostgreSQL configured, these integration tests explicitly skip; a skipped test is not evidence that persistence works.

The corpus runner is `app.corpus_prepare`. It processes development cases before held-out cases and records each phase and all artifacts. The held-out phase requires a frozen preparation algorithm; generating poses is not by itself successful character preparation.

References: [official PostgreSQL container and persistent-volume layout](https://docs.docker.com/guides/postgresql/immediate-setup-and-data-persistence/), [Psycopg installation](https://www.psycopg.org/psycopg3/docs/basic/install.html).
