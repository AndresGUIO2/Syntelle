"""Private, expiring provider bodies; budget receipts remain in the archive ledger."""
from hashlib import sha256
import json

from app.preparation.archive import encoded


def ensure(ledger):
    # Separate from immutable experiment blobs: customer photo retention must
    # not be extended by an immutable shared blob or historical snapshot.
    with ledger.connect() as conn:
        conn.execute("""CREATE TABLE IF NOT EXISTS creation_request_evidence (
            request_id bigint PRIMARY KEY REFERENCES requests(id),
            creation_id text NOT NULL CHECK(creation_id ~ '^char_[a-f0-9]{32}$'),
            request_body bytea, response_body bytea, response_status integer,
            created_at timestamptz NOT NULL DEFAULT now(),
            expires_at timestamptz NOT NULL DEFAULT now()+interval '24 hours',
            erased_at timestamptz
        )""")


def before_call(ledger, request_id, creation_id, payload):
    body = ledger.checked(encoded(payload))
    with ledger.connect() as conn:
        conn.execute("""INSERT INTO creation_request_evidence
            (request_id,creation_id,request_body,expires_at)
            VALUES (%s,%s,%s,LEAST(now()+interval '24 hours',COALESCE(
                (SELECT expires_at FROM syntelle_accounts.character_creations WHERE id=%s),
                now()+interval '24 hours')))""", (request_id,creation_id,body,creation_id))


def after_call(ledger, request_id, raw, status, directory):
    body = ledger.checked(raw)
    # Keep a private recovery copy before a separate database connection can
    # fail. It is never used to retry the provider or resume generation.
    (directory / "response.json").write_bytes(body)
    (directory / ".provider-receipt.json").write_bytes(encoded({
        "request_id":request_id,"status":status,"sha256":sha256(body).hexdigest(),
    }))
    with ledger.connect() as conn:
        conn.execute("""UPDATE creation_request_evidence SET response_body=%s,response_status=%s
            WHERE request_id=%s AND erased_at IS NULL AND expires_at>now()""", (body,status,request_id))


def recover(conn, work):
    """Restore received bodies after a lost DB connection, never replay calls."""
    if not work.exists():
        return
    for marker in work.rglob(".provider-receipt.json"):
        body_path = marker.parent / "response.json"
        if (marker.is_symlink() or body_path.is_symlink() or
            not marker.resolve().is_relative_to(work.resolve()) or
            not body_path.resolve().is_relative_to(work.resolve())):
            raise ValueError("Unsafe private response evidence")
        receipt = json.loads(marker.read_bytes())
        body = body_path.read_bytes()
        if sha256(body).hexdigest() != receipt["sha256"]:
            raise ValueError("Private response evidence changed")
        key = work.name + "/" + marker.parent.relative_to(work).as_posix()
        conn.execute("""UPDATE syntelle_preparation.creation_request_evidence AS evidence
            SET response_body=%s,response_status=%s
            FROM syntelle_preparation.requests AS request
            WHERE evidence.request_id=%s AND request.id=evidence.request_id
                AND evidence.creation_id=%s AND request.request_key=%s
                AND evidence.response_body IS NULL AND evidence.erased_at IS NULL
                AND evidence.expires_at>now()
                AND NOT EXISTS (SELECT 1 FROM syntelle_accounts.character_creations AS job
                    WHERE job.id=evidence.creation_id AND job.deleted_at IS NOT NULL)""",
            (body,receipt["status"],receipt["request_id"],work.name,key))


def expire(conn):
    # The standard worker connection uses the account schema explicitly.
    table = "syntelle_preparation.creation_request_evidence"
    if conn.execute("SELECT to_regclass(%s) AS name", (table,)).fetchone()["name"] is None:
        return
    conn.execute(f"""UPDATE {table} AS evidence SET request_body=NULL,response_body=NULL,erased_at=now()
        WHERE erased_at IS NULL AND (expires_at<=now() OR EXISTS (
            SELECT 1 FROM syntelle_accounts.character_creations AS job
            WHERE job.id=evidence.creation_id AND job.deleted_at IS NOT NULL))""")
