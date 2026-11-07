"""Private PostgreSQL consumer for authenticated character-creation jobs."""
import argparse
import json
from contextlib import contextmanager
from datetime import datetime, timezone
from pathlib import Path
import re
import shutil
import time

import psycopg
from psycopg.rows import dict_row

from app.preparation.archive import Archive, encoded, local_connection
from app.preparation.faces import PreparationError
from app.core.paths import ROOT
from .preparation import CreationError, REVISION, build, digest, read, published_source, source_bundle, validate_native, write
from . import evidence

JOB_ID = re.compile(r"char_[a-f0-9]{32}")
TABLE = "syntelle_accounts.character_creations"
ACTIVE = "deleted_at IS NULL AND expires_at > now() AND status='processing'"


def approval_for(path, job):
    if not path:
        return None
    auth = read(path)
    if auth.get("version") == "character-creation-campaign/1":
        n = auth.get("maximum_characters")
        if (type(n) is not int or not 1 <= n <= 100 or not re.fullmatch(r"[a-z0-9-]{4,64}",auth.get("campaign_id","")) or
            auth.get("limits") != {"image":9*n,"analysis":9*n} or not unexpired(auth)):
            raise CreationError("invalid_authorization")
        return {"creation_id":job["id"],"source_sha256":job["source_sha256"],
                "limits":{"image":9,"analysis":9},"campaign":auth}
    if auth.get("creation_id") != job["id"]:
        return None
    if (auth.get("version") != "character-creation-approval/1" or
        auth.get("source_sha256") != job["source_sha256"] or auth.get("limits") != {"image":9,"analysis":9}):
        raise CreationError("invalid_authorization")
    return auth


def unexpired(policy):
    try:
        end = datetime.fromisoformat(policy["expires_at"])
        return end.tzinfo is not None and datetime.now(timezone.utc) < end
    except (ValueError,KeyError,TypeError):
        return False


@contextmanager
def funded_providers(auth, dsn, active):
    """Reserve a unique request before each paid call. Never retry a spent slot.

    Complete input is committed before the call, with private 24-hour retention.
    The permanent ledger keeps only hashes and usage after bodies expire.
    """
    if not auth:
        yield None
        return
    ledger = Archive(dsn=dsn)
    evidence.ensure(ledger)
    policy = auth.get("campaign",auth)
    budget = ("account-character-campaign/"+policy["campaign_id"] if "campaign" in auth else
              "account-character/"+auth["creation_id"])
    ledger.budget(budget, digest(encoded(policy)), policy["limits"])
    # A whole job must fit before spending even one request. The session lock
    # serializes the optional shared campaign across independent consumers.
    with ledger.connect() as guard:
        guard.commit()  # Archive.connect sets the dedicated schema.
        guard.autocommit = True
        if not guard.execute("SELECT pg_try_advisory_lock(hashtextextended(%s,0)) AS ok",(budget,)).fetchone()["ok"]:
            raise CreationError("generation_busy")
        scope = guard.execute("SELECT halted FROM budgets WHERE id=%s",(budget,)).fetchone()
        counts = guard.execute("SELECT kind,count(*) AS n,bool_or(status='started') AS uncertain FROM requests WHERE budget_id=%s GROUP BY kind",(budget,)).fetchall()
        if scope["halted"] or any(r["uncertain"] or policy["limits"][r["kind"]]-r["n"] < auth["limits"][r["kind"]] for r in counts):
            raise CreationError("generation_budget_unavailable")
        yield {kind:recorded_provider(kind, ledger, budget, auth, active) for kind in ("image","analysis")}

def recorded_provider(kind, ledger, budget, auth, active):
    from app.workflows.contours.gemini_contour_trial import post_once
    def submit(payload, settings, key, directory):
        if not active():
            raise CreationError("cancelled")
        if "campaign" in auth and not unexpired(auth["campaign"]):
            raise CreationError("invalid_authorization")
        if "correction" in directory.parts or payload["model"] != {
            "image":"gemini-nano-banana-2.1", "analysis":"gemini-4-argon"}[kind]:
            raise CreationError("invalid_authorization")
        # All path components after the per-job root are application-generated.
        parent = next((p for p in directory.parents if p.name == auth["creation_id"]), None)
        if parent is None:
            raise CreationError("invalid_authorization")
        binding = digest(encoded(payload))
        rid, fresh = ledger.reserve(budget, auth["creation_id"]+"/"+directory.relative_to(parent).as_posix(), kind, binding,
                                    {"source_sha256":auth["source_sha256"],"model":payload["model"]})
        if not fresh:
            raise CreationError("request_already_reserved")
        status = "failed"
        usage = {}
        try:
            evidence.before_call(ledger, rid, auth["creation_id"], payload)
            raw, code, duration = post_once(payload, settings, key)
            evidence.after_call(ledger, rid, raw, code, directory)
            data = json.loads(raw)
            if isinstance(data,dict) and 200 <= code < 300 and "error" not in data and data.get("status") in (None,"completed"):
                status = "received"
            if isinstance(data,dict) and isinstance(data.get("usage"),dict):
                usage = data["usage"]
            if status != "received":
                raise CreationError("provider_failed")
            return raw, code, duration
        finally:
            ledger.finish(rid, status, usage)
    return submit


def remove_private_work(root, job_id):
    if not JOB_ID.fullmatch(job_id):
        raise CreationError("invalid_job")
    p = root / job_id
    if p.is_symlink() or p.resolve().parent != root.resolve():
        raise CreationError("invalid_job")
    if p.exists():
        shutil.rmtree(p)


@contextmanager
def private_stage(root, job_id):
    remove_private_work(root,job_id)
    path = root / job_id
    path.mkdir(mode=0o700)
    try:
        yield path
    finally:
        remove_private_work(root,job_id)


def cleanup_packages(conn, packages):
    """Only delete folders created by this workflow, after the last owner reference."""
    for path in packages.iterdir():
        if not re.fullmatch("[0-9a-f]{64}",path.name) or path.is_symlink() or path.resolve().parent != packages:
            continue
        marker = path / ".creation-managed.json"
        if not marker.is_file() or marker.is_symlink():
            continue
        record = read(marker)
        if record.get("package_hash") != path.name or not re.fullmatch("[0-9a-f]{64}",record.get("source_sha256","")):
            continue
        with conn.transaction():
            conn.execute("SELECT pg_advisory_xact_lock(hashtextextended(%s,0))",("package:"+path.name,))
            referenced = conn.execute(f"""SELECT 1 FROM {TABLE} WHERE deleted_at IS NULL
                AND ((status='ready' AND package_hash=%s) OR (status IN ('queued','processing')
                AND source_sha256=%s)) LIMIT 1""",(path.name,record["source_sha256"])).fetchone()
            # Published workspace copies own their immutable package independently
            # of the personal creation and its source-retention lifecycle.
            publications = conn.execute("SELECT to_regclass('syntelle_accounts.publications') AS name").fetchone()
            if not referenced and publications["name"] is not None:
                referenced = conn.execute("SELECT 1 FROM syntelle_accounts.publications WHERE package_hash=%s LIMIT 1", (path.name,)).fetchone()
            if not referenced and path.is_dir():
                shutil.rmtree(path)


def claim(conn, selected=None):
    args = (selected,) if selected else ()
    restrict = " AND id=%s" if selected else ""
    with conn.transaction():
        rows = conn.execute(f"""SELECT id,status FROM {TABLE}
            WHERE status IN ('queued','processing') AND deleted_at IS NULL AND expires_at>now(){restrict}
            ORDER BY CASE WHEN status='queued' THEN 0 ELSE 1 END, created_at LIMIT 32
            FOR UPDATE SKIP LOCKED""", args).fetchall()
        for row in rows:
            if not JOB_ID.fullmatch(row["id"]):
                continue
            if not conn.execute("SELECT pg_try_advisory_lock(hashtextextended(%s, 0)) AS locked",
                                ("creation:" + row["id"],)).fetchone()["locked"]:
                continue
            if row["status"] == "processing":
                # A previous process lost its DB connection. A paid outcome
                # could be uncertain: preserve the closed ledger, never replay.
                conn.execute(f"""UPDATE {TABLE} SET status='failed',failure_code='worker_interrupted',
                             updated_at=now() WHERE id=%s""", (row["id"],))
                return {**row,"abandoned":True}
            return conn.execute(f"""UPDATE {TABLE} SET status='processing',updated_at=now()
                    WHERE id=%s RETURNING id,source_bytes,source_sha256""", (row["id"],)).fetchone()
    return None


def run_once(conn, dsn, packages, scratch, checker, *, approval=None, selected=None):
    evidence.expire(conn)
    conn.execute(f"""UPDATE {TABLE} SET source_bytes=NULL,updated_at=now(),
                      failure_code=CASE WHEN status IN ('processing','queued') THEN 'source_expired' ELSE failure_code END,
                      status=CASE WHEN status IN ('processing','queued') THEN 'failed' ELSE status END
                      WHERE source_bytes IS NOT NULL AND expires_at<=now()""")
    cleanup_packages(conn, packages)
    # Recover private working files left by crashes even after source expiry or deletion.
    staging = packages/".staging"
    if staging.is_symlink() or staging.resolve().parent != packages:
        raise CreationError("invalid_job")
    staging.mkdir(exist_ok=True,mode=0o700)
    for path in (*scratch.iterdir(), *staging.iterdir()):
        if JOB_ID.fullmatch(path.name) and conn.execute(
                "SELECT pg_try_advisory_lock(hashtextextended(%s,0)) AS locked",("creation:"+path.name,)).fetchone()["locked"]:
            try:
                if path.parent == scratch:
                    evidence.recover(conn, path)
                remove_private_work(path.parent, path.name)
            finally:
                conn.execute("SELECT pg_advisory_unlock(hashtextextended(%s,0))",("creation:"+path.name,))
    job = claim(conn, selected)
    if not job:
        return None
    job_id = job["id"]
    def active():
        return bool(conn.execute(f"SELECT 1 FROM {TABLE} WHERE id=%s AND {ACTIVE}", (job_id,)).fetchone())
    try:
        if job.get("abandoned"):
            return {"id":job_id,"status":"failed","failure_code":"worker_interrupted"}
        work = scratch / job_id
        work.mkdir(mode=0o700)
        catalog = read(ROOT / "docs/characters/creation-source-cache.json")
        reusable = source_bundle(job["source_sha256"], catalog) or published_source(job["source_sha256"], packages)
        auth = None if reusable else approval_for(approval, job)
        # Place temporary output on the destination filesystem for atomic rename.
        # It holds only exported runtime data, not upload/request working files.
        with private_stage(staging,job_id) as temp:
            output = temp / "package"
            with funded_providers(auth, dsn, active) as provider:
                package_hash = build(bytes(job["source_bytes"]), job["source_sha256"], work, output, checker,
                                     approval=auth, providers=provider, saved=packages,catalog=catalog)
            dest = packages / package_hash
            with conn.transaction():
                if not conn.execute(f"SELECT 1 FROM {TABLE} WHERE id=%s AND {ACTIVE} FOR UPDATE",(job_id,)).fetchone():
                    raise CreationError("cancelled")
                conn.execute("SELECT pg_advisory_xact_lock(hashtextextended(%s,0))", ("package:"+package_hash,))
                if dest.exists():
                    validate_native(dest, job["source_sha256"], package_hash)
                else:
                    write(output/".creation-managed.json",{"package_hash":package_hash,"source_sha256":job["source_sha256"],"revision":REVISION})
                    output.rename(dest)
                conn.execute(f"""UPDATE {TABLE} SET status='ready',package_hash=%s,ready_at=now(),
                             updated_at=now(),failure_code=NULL WHERE id=%s""",(package_hash,job_id))
        return {"id":job_id,"status":"ready","package_hash":package_hash}
    except (CreationError, PreparationError, ValueError, OSError, KeyError, TypeError, RuntimeError) as exc:
        code = exc.code if isinstance(exc, CreationError) else "preparation_failed"
        conn.execute(f"""UPDATE {TABLE} SET status='failed',failure_code=%s,updated_at=now()
                     WHERE id=%s AND status='processing' AND deleted_at IS NULL""",(code,job_id))
        return {"id":job_id,"status":"failed","failure_code":code}
    finally:
        try:
            # A failed recovery leaves private files for the next connected
            # consumer. It cannot erase the only copy of a received response.
            evidence.recover(conn, scratch / job_id)
            remove_private_work(scratch, job_id)
        finally:
            conn.execute("SELECT pg_advisory_unlock(hashtextextended(%s,0))",("creation:"+job_id,))


def main():
    cli = argparse.ArgumentParser(description=__doc__)
    cli.add_argument("--packages", type=Path, required=True)
    cli.add_argument("--scratch", type=Path, required=True)
    cli.add_argument("--checker", type=Path, required=True, help="Trusted native render-frame executable")
    cli.add_argument("--approval", type=Path, help="Private exact-job paid-generation authorization")
    cli.add_argument("--once", action="store_true")
    cli.add_argument("--id", help="Handle only this job")
    args = cli.parse_args()
    if args.id and (not JOB_ID.fullmatch(args.id) or not args.once):
        cli.error("--id requires --once and a valid creation id")
    packages, scratch = args.packages.resolve(), args.scratch.resolve()
    if packages.is_relative_to(scratch) or scratch.is_relative_to(packages):
        cli.error("Keep published packages and private expiring work in separate directories")
    for path in (packages,scratch):
        path.mkdir(parents=True,exist_ok=True,mode=0o700)
    dsn, _ = local_connection()
    while True:
        try:
            with psycopg.connect(dsn,autocommit=True,row_factory=dict_row,connect_timeout=5) as conn:
                while True:
                    result = run_once(conn, dsn, packages, scratch, args.checker,
                                      approval=args.approval,selected=args.id)
                    if result:
                        print(json.dumps(result),flush=True)
                    if args.once:
                        return
                    if not result:
                        time.sleep(2)
        except psycopg.Error:
            if args.once:
                raise SystemExit("Creation database unavailable") from None
            print("Creation database unavailable; waiting for local service.", flush=True)
            time.sleep(5)


if __name__ == "__main__":
    main()
