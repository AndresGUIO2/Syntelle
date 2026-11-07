import os
import uuid
from pathlib import Path

import psycopg
import pytest
from psycopg import sql
from psycopg.conninfo import make_conninfo
from psycopg.rows import dict_row

from app.core.paths import ROOT
from app.workflows.characters.preparation import CreationError, build, digest, read
from app.workflows.characters.worker import approval_for, funded_providers, run_once, cleanup_packages


def test_unrecognized_portrait_is_never_substituted(tmp_path):
    data = b"an unrecognized input"
    with pytest.raises(CreationError, match="generation_not_enabled"):
        build(data, digest(data), tmp_path, tmp_path/"out", tmp_path/"checker")
    with pytest.raises(CreationError, match="source_mismatch"):
        build(data, "a"*64, tmp_path, tmp_path/"out", tmp_path/"checker")
    assert not (tmp_path/"out").exists()


def test_authorization_cannot_move_to_another_job_or_source(tmp_path):
    import json
    f = tmp_path/"approval.json"
    job = {"id":"char_"+"a"*32, "source_sha256":"c"*64}
    auth = {"version":"character-creation-approval/1","creation_id":job["id"],"source_sha256":job["source_sha256"],
            "limits":{"image":9,"analysis":9}}
    f.write_text(json.dumps(auth))
    assert approval_for(f,job) == auth
    assert approval_for(f,{**job,"id":"char_"+"b"*32}) is None
    with pytest.raises(CreationError,match="invalid_authorization"):
        approval_for(f, {**job,"source_sha256":"a"*64})


@pytest.fixture
def isolated_db():
    base = os.environ.get("SYNTELLE_CHARACTER_CREATION_TEST_DSN")
    if not base:
        pytest.skip("set a local PostgreSQL test admin DSN to create an isolated database")
    name = "syntelle_creation_test_" + uuid.uuid4().hex
    with psycopg.connect(base,autocommit=True) as admin:
        admin.execute(sql.SQL("CREATE DATABASE {}").format(sql.Identifier(name)))
        try:
            dsn = make_conninfo(base,dbname=name)
            with psycopg.connect(dsn,autocommit=True,row_factory=dict_row) as conn:
                assert conn.info.dbname == name
                conn.execute("CREATE SCHEMA syntelle_accounts")
                conn.execute("CREATE TABLE syntelle_accounts.accounts (id text PRIMARY KEY)")
                conn.execute((ROOT/"services/api/internal/accounts/migrations/004_character_creations.sql").read_text())
                yield conn,dsn
        finally:
            admin.execute(sql.SQL("DROP DATABASE {} WITH (FORCE)").format(sql.Identifier(name)))


def test_published_package_survives_personal_deletion(isolated_db, tmp_path):
    import json
    conn, _ = isolated_db
    packages = (tmp_path / "packages").resolve()
    packages.mkdir()
    package = packages / ("a" * 64)
    package.mkdir()
    (package / ".creation-managed.json").write_text(json.dumps({
        "package_hash": package.name, "source_sha256": "b" * 64,
    }))
    # No personal source remains; a workspace publication owns the immutable
    # prepared output, including after its public access has been revoked.
    conn.execute("CREATE TABLE syntelle_accounts.publications (package_hash text)")
    conn.execute("INSERT INTO syntelle_accounts.publications VALUES (%s)", (package.name,))
    cleanup_packages(conn, packages)
    assert package.is_dir()
    conn.execute("DELETE FROM syntelle_accounts.publications")
    cleanup_packages(conn, packages)
    assert not package.exists()


def test_claim_publish_expire_and_recover_without_provider_calls(isolated_db,tmp_path):
    checker = os.environ.get("SYNTELLE_NATIVE_FRAME_CHECKER")
    if not checker:
        pytest.skip("set a trusted native CPU frame checker for the complete worker check")
    conn,dsn = isolated_db
    packages=tmp_path/"packages";packages.mkdir()
    scratch=tmp_path/"private";scratch.mkdir()
    rows="syntelle_accounts.character_creations"
    conn.execute("INSERT INTO syntelle_accounts.accounts VALUES ('owner')")

    def insert(data,status="queued"):
        id="char_"+uuid.uuid4().hex
        conn.execute(f"""INSERT INTO {rows}(id,account_id,request_key,name,source_sha256,source_bytes,
            mime_type,width,height,status,expires_at) VALUES(%s,'owner',%s,'Test',%s,%s,
            'image/png',1024,572,%s,now()+interval '24 hours')""",(id,id,digest(data),data,status))
        return id
    def handle(id):
        return run_once(conn,dsn,packages,scratch,Path(checker),selected=id)

    portrait=(ROOT/".tools/semantic-oral-06/corpus/cafe/original.png").read_bytes()
    id=insert(portrait)
    result=handle(id)
    assert result["status"]=="ready"
    m=read(packages/result["package_hash"]/"manifest.json")
    assert m["packages"]["motion"]["original_sha256"]==digest(portrait)
    # An exact already-prepared portrait is reusable even when it is not in
    # the development catalog. This path cannot repeat paid preparation.
    second_work=tmp_path/"reuse";second_work.mkdir()
    assert build(portrait,digest(portrait),second_work,tmp_path/"unused",Path(checker),
                 saved=packages,catalog={"version":"creation-source-cache/1","items":[]})==result["package_hash"]
    assert not (tmp_path/"unused").exists()
    assert handle(id) is None and not list(scratch.iterdir())
    conn.execute(f"UPDATE {rows} SET expires_at=now() WHERE id=%s",(id,))
    assert handle(id) is None
    row=conn.execute(f"SELECT source_bytes,status,package_hash FROM {rows} WHERE id=%s",(id,)).fetchone()
    assert row=={"source_bytes":None,"status":"ready","package_hash":result["package_hash"]}
    other=insert(b"new source")
    assert handle(other)["failure_code"]=="generation_not_enabled"
    abandoned=insert(b"uncertain previous provider run","processing")
    assert handle(abandoned)["failure_code"]=="worker_interrupted"
    assert len([p for p in packages.iterdir() if not p.name.startswith(".")])==1
    conn.execute(f"UPDATE {rows} SET deleted_at=now() WHERE id=%s",(id,))
    assert handle(id) is None
    assert not (packages/result["package_hash"]).exists()


def test_campaign_requires_full_remaining_allowance_before_first_call(isolated_db,tmp_path,monkeypatch):
    import json
    from datetime import datetime, timedelta, timezone
    from app.workflows.contours import gemini_contour_trial
    conn,dsn=isolated_db
    policy={"version":"character-creation-campaign/1", "campaign_id":"isolated-verification",
            "maximum_characters":1, "limits":{"image":9,"analysis":9},
            "expires_at":(datetime.now(timezone.utc)+timedelta(hours=1)).isoformat()}
    f=tmp_path/"approval.json"
    f.write_text(json.dumps(policy))
    job={"id":"char_"+"1"*32,"source_sha256":"2"*64}
    auth=approval_for(f,job)
    attempts=[]
    def counted(payload,*args):
        # A different transaction can read complete input before the provider
        # starts; a hash-only ledger is insufficient for restoration.
        from app.preparation.archive import encoded
        record=conn.execute("SELECT request_body FROM syntelle_preparation.creation_request_evidence").fetchone()
        assert bytes(record["request_body"])==encoded(payload)
        attempts.append(payload["model"])
        return b'{"status":"completed"}',200,.001
    monkeypatch.setattr(gemini_contour_trial,"post_once",counted)
    dest=tmp_path/job["id"]/"poses"/"a"
    dest.mkdir(parents=True)
    with funded_providers(auth,dsn,lambda:True) as providers:
        providers["image"]({"model":"gemini-nano-banana-2.1"},{},'offline',dest)
    other=approval_for(f,{**job,"id":"char_"+"3"*32})
    with pytest.raises(CreationError,match="generation_budget_unavailable"):
        with funded_providers(other,dsn,lambda:True):
            pytest.fail("A second portrait exceeded the operator's global cap")
    assert attempts==["gemini-nano-banana-2.1"]
    record=conn.execute("SELECT response_body,response_status FROM syntelle_preparation.creation_request_evidence").fetchone()
    assert bytes(record["response_body"])==b'{"status":"completed"}' and record["response_status"]==200
    from app.workflows.characters import evidence
    conn.execute("UPDATE syntelle_preparation.creation_request_evidence SET expires_at=now()")
    evidence.expire(conn)
    record=conn.execute("SELECT request_body,response_body,erased_at FROM syntelle_preparation.creation_request_evidence").fetchone()
    assert record["request_body"] is None and record["response_body"] is None and record["erased_at"] is not None
    assert conn.execute("SELECT count(*) AS n FROM syntelle_preparation.requests").fetchone()["n"]==1
    policy["expires_at"]="2000-01-01T00:00:00Z"
    f.write_text(json.dumps(policy))
    with pytest.raises(CreationError,match="invalid_authorization"):
        approval_for(f,job)


@pytest.mark.parametrize("raw,code", [
    (b'{"error":{"message":"fixture rejection"}}',400),
    (b'{"error":{"message":"fixture rejection"}}',200),
    (b'[]',200),
])
def test_provider_rejection_keeps_body_and_stops_campaign(isolated_db,tmp_path,monkeypatch,raw,code):
    from app.workflows.contours import gemini_contour_trial
    conn,dsn=isolated_db
    auth={"creation_id":"char_"+"4"*32,"source_sha256":"5"*64,"limits":{"image":9,"analysis":9}}
    dest=tmp_path/auth["creation_id"]/"poses"/"a"
    dest.mkdir(parents=True)
    calls=[]
    def rejected(*args):
        calls.append(True)
        return raw,code,.001
    monkeypatch.setattr(gemini_contour_trial,"post_once",rejected)
    with pytest.raises(CreationError,match="provider_failed"):
        with funded_providers(auth,dsn,lambda:True) as providers:
            providers["image"]({"model":"gemini-nano-banana-2.1"},{},"offline",dest)
    record=conn.execute("SELECT response_body,response_status FROM syntelle_preparation.creation_request_evidence").fetchone()
    assert bytes(record["response_body"])==raw and record["response_status"]==code
    assert conn.execute("SELECT halted FROM syntelle_preparation.budgets").fetchone()["halted"]
    with pytest.raises(CreationError,match="generation_budget_unavailable"):
        with funded_providers(auth,dsn,lambda:True):
            pytest.fail("Provider failure must stop the campaign")
    assert len(calls)==1


def test_received_body_survives_database_failure_without_provider_replay(isolated_db,tmp_path,monkeypatch):
    from app.workflows.characters import evidence
    from app.workflows.contours import gemini_contour_trial
    conn,dsn=isolated_db
    auth={"creation_id":"char_"+"6"*32,"source_sha256":"7"*64,"limits":{"image":9,"analysis":9}}
    work=tmp_path/auth["creation_id"]
    dest=work/"poses"/"a";dest.mkdir(parents=True)
    raw=b'{"status":"completed","output_image":{"data":"fixture"}}'
    calls=[]
    def received(*args):
        calls.append(True)
        return raw,200,.001
    monkeypatch.setattr(gemini_contour_trial,"post_once",received)
    save=evidence.after_call
    def failed_save(ledger,*args):
        def disconnected():
            raise psycopg.OperationalError("fixture connection lost")
        with monkeypatch.context() as patch:
            patch.setattr(ledger,"connect",disconnected)
            save(ledger,*args)
    monkeypatch.setattr(evidence,"after_call",failed_save)
    with pytest.raises(psycopg.OperationalError,match="fixture connection lost"):
        with funded_providers(auth,dsn,lambda:True) as providers:
            providers["image"]({"model":"gemini-nano-banana-2.1"},{},"offline",dest)
    assert (dest/"response.json").read_bytes()==raw
    assert conn.execute("SELECT response_body FROM syntelle_preparation.creation_request_evidence").fetchone()["response_body"] is None
    evidence.recover(conn,work)
    assert bytes(conn.execute("SELECT response_body FROM syntelle_preparation.creation_request_evidence").fetchone()["response_body"])==raw
    assert conn.execute("SELECT halted FROM syntelle_preparation.budgets").fetchone()["halted"]
    assert len(calls)==1
