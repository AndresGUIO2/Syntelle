from concurrent.futures import ThreadPoolExecutor
from hashlib import sha256
from uuid import uuid4

import psycopg
from psycopg import sql
import pytest

from app.preparation.archive import Archive, local_connection
from app.preparation.faces import PreparationError


@pytest.fixture
def archive():
    try: dsn,_=local_connection()
    except PreparationError:pytest.skip('Configure local PostgreSQL to run integration archive checks.')
    schema='test_archive_'+uuid4().hex
    db=Archive(dsn,schema=schema,secrets=[b'test-secret-never-store'])
    yield db
    with psycopg.connect(dsn) as conn:
        conn.execute(sql.SQL('DROP SCHEMA {} CASCADE').format(sql.Identifier(schema)))


def test_actual_bytes_restore_and_snapshots_are_immutable_deduplicated(archive,tmp_path):
    source=tmp_path/'source';source.mkdir();(source/'image.png').write_bytes(b'actual-image-bytes')
    (source/'prompt.txt').write_text('original prompt')
    first=archive.archive_tree('test',source)
    same=archive.archive_tree('test',source)
    assert first==same and archive.summary()['snapshots']==1
    (source/'prompt.txt').write_text('new recorded prompt')
    second=archive.archive_tree('test',source)
    assert second['snapshot_id']!=first['snapshot_id']
    destination=tmp_path/'recovered'
    assert archive.restore(first['snapshot_id'],destination)==2
    assert (destination/'image.png').read_bytes()==b'actual-image-bytes'
    assert (destination/'prompt.txt').read_text()=='original prompt'
    with pytest.raises(PreparationError,match='overwrite'):archive.restore(second['snapshot_id'],destination)
    assert archive.summary()['blobs']==5 # shared image, two prompts, two manifests


def test_secret_rejection_rolls_back_the_complete_snapshot(archive,tmp_path):
    (tmp_path/'a.txt').write_text('safe')
    (tmp_path/'z.txt').write_text('test-secret-never-store')
    with pytest.raises(PreparationError,match='Secret'):archive.archive_tree('unsafe',tmp_path)
    assert archive.summary()['blobs']==0 and archive.summary()['snapshots']==0
    (tmp_path/'z.txt').unlink();(tmp_path/'.env').write_text('a harmless-looking but prohibited file')
    with pytest.raises(PreparationError,match='configuration'):archive.archive_tree('unsafe',tmp_path)


def test_atomic_reservation_survives_reconnection_and_never_retries(archive):
    archive.budget('batch','source-bound',{'image':1,'analysis':1})
    id,created=archive.reserve('batch','portrait/pose','image','payload-hash')
    assert created
    reconnected=Archive(archive.dsn,schema=archive.schema)
    with pytest.raises(PreparationError,match='no retry'):reconnected.reserve('batch','portrait/pose','image','payload-hash')
    with pytest.raises(PreparationError,match='still be running'):reconnected.reserve('batch','other','analysis','different')
    archive.finish(id,'received',{'output_tokens':1120})
    assert reconnected.reserve('batch','portrait/pose','image','payload-hash')==(id,False)
    with pytest.raises(PreparationError,match='exhausted'):archive.reserve('batch','new','image','new')
    with pytest.raises(PreparationError,match='cannot be reset'):archive.budget('batch','source-bound',{'image':2,'analysis':1})
    with pytest.raises(PreparationError,match='immutable'):archive.finish(id,'failed')


def test_failed_request_consumes_budget_and_stops_both_kinds(archive):
    archive.budget('batch','fixed',{'image':32,'analysis':36})
    id,_=archive.reserve('batch','first','image','one');archive.finish(id,'failed',{'tokens':7})
    for kind in ('image','analysis'):
        with pytest.raises(PreparationError,match='stopped'):archive.reserve('batch','next',kind,'two')
    assert archive.summary()['requests']==1


def test_six_image_motion_allowance_is_separate_from_existing_viseme_budget(archive):
    archive.budget('visemes','original-viseme-binding',{'image':32,'analysis':36})
    archive.budget('motion','separate-motion-binding',{'image':6,'analysis':0})
    for i in range(6):
        id,created=archive.reserve('motion',f'eye-{i}','image',f'payload-{i}')
        assert created;archive.finish(id,'received',{'images':1})
    with pytest.raises(PreparationError,match='exhausted'):archive.reserve('motion','seventh','image','seven')
    with pytest.raises(PreparationError,match='exhausted'):archive.reserve('motion','contours','analysis','none')
    id,created=archive.reserve('visemes','existing-work','image','unchanged-budget')
    assert created;archive.finish(id,'received')


def test_concurrent_reservations_cannot_duplicate_a_paid_attempt(archive):
    archive.budget('batch','fixed',{'image':1,'analysis':0})
    def reserve():
        try:return archive.reserve('batch','one','image','payload')
        except PreparationError:return None
    with ThreadPoolExecutor(max_workers=2) as workers:results=list(workers.map(lambda _:reserve(),range(2)))
    assert sum(r is not None and r[1] for r in results)==1
    assert archive.summary()['requests']==1


def test_provider_cannot_send_until_reservation_and_inputs_are_committed(archive,tmp_path,monkeypatch):
    from app.preparation import recorded_provider
    directory=tmp_path/'portrait'/'ei';directory.mkdir(parents=True)
    (directory/'prompt.txt').write_text('source-bound prompt')
    (directory/'request.json').write_text('{"model":"test-model"}')
    archive.budget('batch','fixed',{'image':1,'analysis':0});calls=[]
    def post(payload,settings,key):
        with archive.connect() as conn:
            row=conn.execute('SELECT status FROM requests').fetchone()
            assert row['status']=='started'
            assert conn.execute('SELECT count(*) AS n FROM snapshot_files').fetchone()['n']==2
        calls.append(1)
        return b'{"status":"completed","usage":{"input_tokens":7},"output_text":"safe"}',200,.1
    monkeypatch.setattr(recorded_provider,'post_once',post)
    provider=recorded_provider.RecordedProvider(archive,'batch',tmp_path,'image')
    provider({'model':'test-model'},{},'test-secret-never-store',directory)
    assert len(calls)==1
    with archive.connect() as conn:
        row=conn.execute('SELECT status,usage FROM requests').fetchone()
        assert row['status']=='received' and row['usage']=={'input_tokens':7}
        assert conn.execute('SELECT count(*) AS n FROM blobs WHERE position(%s::bytea in data)>0',(b'test-secret-never-store',)).fetchone()['n']==0
    with pytest.raises(PreparationError,match='already exists'):provider({'model':'test-model'},{},'test-secret-never-store',directory)
    assert len(calls)==1
