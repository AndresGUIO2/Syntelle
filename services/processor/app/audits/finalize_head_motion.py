"""Freeze, audit and restore the local head/speech candidate without paid calls."""
import base64
from hashlib import sha256
import json
from pathlib import Path
import shutil
from uuid import uuid4

from dotenv import dotenv_values
import psycopg
from psycopg import sql
from psycopg.rows import dict_row
from psycopg.types.json import Jsonb

from app.audits.finalize_refinement import digest, require, source_snapshot
from app.preparation.archive import Archive
from app.workflows.visemes.viseme_trial import ROOT, write_report

BUDGET = 'authorized-dragon-motion-2026-10-19'


def ledger_export(conn):
    budgets = conn.execute('SELECT * FROM syntelle_speech.budgets ORDER BY id').fetchall()
    requests = conn.execute('SELECT * FROM syntelle_speech.requests ORDER BY id').fetchall()
    for row in requests:
        row['audio'] = base64.b64encode(bytes(row['audio'] or b'')).decode()
    return json.loads(json.dumps({'budgets': budgets, 'requests': requests}, default=str))


def restore_ledger(dsn, record, schema):
    name = 'syntelle_speech_restore_' + uuid4().hex[:12]
    require(name.startswith('syntelle_speech_restore_'), 'Unsafe test database name.')
    with psycopg.connect(dsn, autocommit=True) as admin:
        admin.execute(sql.SQL('CREATE DATABASE {}').format(sql.Identifier(name)))
        try:
            with psycopg.connect(dsn, dbname=name, row_factory=dict_row) as conn:
                require(conn.execute('SELECT current_database() AS name').fetchone()['name'] == name,
                        'Refusing restoration outside its isolated database.')
                conn.execute(schema)
                for b in record['budgets']:
                    conn.execute('INSERT INTO syntelle_speech.budgets(id,max_requests,max_characters,halted,created_at) VALUES(%s,%s,%s,%s,%s) ON CONFLICT(id) DO UPDATE SET max_requests=excluded.max_requests,max_characters=excluded.max_characters,halted=excluded.halted,created_at=excluded.created_at',
                                 tuple(b[k] for k in ['id','max_requests','max_characters','halted','created_at']))
                for r in record['requests']:
                    conn.execute('INSERT INTO syntelle_speech.requests(id,budget_id,request_key,binding,characters,request,status,response,audio,error,started_at,finished_at) OVERRIDING SYSTEM VALUE VALUES(%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s)',
                        (r['id'],r['budget_id'],r['request_key'],r['binding'],r['characters'],Jsonb(r['request']),r['status'],Jsonb(r['response']),base64.b64decode(r['audio']),r['error'],r['started_at'],r['finished_at']))
                require(ledger_export(conn) == record, 'Restored speech rows/audio differ.')
            return {'isolated_database': True, 'actual_audio_and_rows_equal': True,
                    'requests': len(record['requests']), 'fixture_records_preserved': 12}
        finally:
            # Exact name was created above for this verification; never the owner database.
            admin.execute(sql.SQL('DROP DATABASE {} WITH (FORCE)').format(sql.Identifier(name)))


def main():
    root = ROOT / '.tools/head-motion-12'
    dsn = dotenv_values(ROOT / '.env')['SYNTELLE_DATABASE_URL']
    db = Archive(dsn=dsn)
    cases = {}
    for name in ['syntelle','cafe','woodland']:
        package = root / name / 'package.json'
        definition = json.loads(package.read_text())
        previous = ROOT / '.tools/foreground-11/prepared' / name
        old = json.loads((previous/'package.json').read_text())
        require(definition['original_sha256'] == old['original_sha256'] and definition['oral_build_id'] == old['oral_build_id'], 'Source or oral dependency changed.')
        require(digest(package) == digest(ROOT/'apps/web/src/experiments/motion/natural'/name/'package.json'), 'Candidate manifest is not the prepared manifest.')
        require(digest(package) == digest(root/'cache-replay'/name/'package.json'), 'Rig replay differs.')
        require(definition['rig']['geometry_validation']['valid'], 'Invalid rig geometry.')
        unchanged = {}
        for file in ['foreground.webp','depth.webp','eyes/open.webp','eyes/closed.webp','eyes/mask.png']:
            runtime = ROOT/'apps/web/src/experiments/motion/assets'/name/file
            require(digest(runtime) == digest(previous/file), 'Cached character resource changed.')
            unchanged[file] = digest(runtime)
        review = json.loads((root/name/'browser-review/report.json').read_text())
        cases[name] = {'package_sha256':digest(package),'geometry':definition['rig']['geometry_validation'],
                       'unchanged':unchanged,'render_checks':len(review['checks']),
                       'dependency_snapshot':db.archive_tree('head-motion-12/motion-dependency/'+name,previous)}
    with psycopg.connect(dsn,row_factory=dict_row) as conn:
        record = ledger_export(conn)
    real = [r for r in record['requests'] if r['budget_id'] == BUDGET]
    require(len(real) <= 12 and sum(r['characters'] for r in real) <= 6000, 'Speech allowance exceeded.')
    require(len([r for r in real if r['status']=='received']) >= 4, 'Live voice/language verification is incomplete.')
    require(all(base64.b64decode(r['audio'])[:4] == b'RIFF' for r in real if r['status']=='received'), 'Real ledger includes fixture audio.')
    write_report(root/'speech/ledger-export.json',record)
    schema = (ROOT/'services/api/speech.sql').read_text()
    (root/'speech/schema.sql').write_text(schema)
    restored_ledger = restore_ledger(dsn,record,schema)
    write_report(root/'speech/ledger-restore.json',restored_ledger)
    with db.connect() as conn:
        counts = conn.execute('SELECT budget_id,kind,count(*) AS n FROM requests GROUP BY budget_id,kind ORDER BY budget_id,kind').fetchall()
    motion = json.loads((ROOT/'docs/characters/motion-development.json').read_text())
    require(sum(r['n'] for r in counts if r['budget_id']==motion['budget_id']) == 3, 'Eye request count changed.')
    source_snapshot(root/'source-snapshot')
    for name in ['README.md','compose.yaml','services/api/Dockerfile','services/api/go.sum','services/api/speech.sql','docs/experiments/motion/natural-head-speech.md']:
        destination = root/'source-snapshot'/name
        destination.parent.mkdir(parents=True,exist_ok=True)
        shutil.copyfile(ROOT/name,destination)
    for path in (ROOT/'apps/web/src/experiments/motion/natural').rglob('*.json'):
        target = root/'source-snapshot'/path.relative_to(ROOT)
        target.parent.mkdir(parents=True,exist_ok=True);shutil.copyfile(path,target)
    # Preserve the public template without a secret-config filename in the archive.
    template = root/'source-snapshot/configuration/env-example.txt'
    template.parent.mkdir(parents=True,exist_ok=True);shutil.copyfile(ROOT/'.env.example',template)
    write_report(root/'source-snapshot/configuration/path-map.json',{'env-example.txt':'.env.example'})
    native = ROOT/'.tools/SpeechSDK-Linux-1.52.0.tar.gz'
    require(digest(native) == '2573d5336cdba31759967b77b3596dcd450b802fa8c1a410245c541a5df0fb47', 'Native SDK checksum differs.')
    vendor = root/'native-sdk';vendor.mkdir(exist_ok=True)
    shutil.copyfile(native,vendor/native.name)
    write_report(root/'audit.json',{'cases':cases,'speech_requests':len(real),
        'speech_characters':sum(r['characters'] for r in real),'ledger_restoration':restored_ledger,
        'image_analysis_counts':counts,'new_image_calls':0,'manual_character_overrides':False,
        'default_promoted':False,'owner_acceptance':False,'real_iPhone_validated':False,
        'strict_syntelle_webkit_60fps':False,'public_upload_validation_complete':False})
    archived = db.archive_tree('head-motion-12',root)
    destination = ROOT/'.tools/head-motion-12-restored'/str(archived['snapshot_id'])
    restored = db.restore(archived['snapshot_id'],destination)
    for path in root.rglob('*'):
        if not path.is_file() or '__pycache__' in path.parts or path.suffix in ('.pyc','.tmp'): continue
        require(digest(path)==digest(destination/path.relative_to(root)), 'Restored evidence bytes differ.')
    result = {'snapshot':archived,'restored_files':restored,'all_bytes_verified':True,
              'speech_requests':len(real),'speech_characters':sum(r['characters'] for r in real)}
    db.event('head-motion-12','verified-restoration',result)
    write_report(ROOT/'.tools/head-motion-12-postgres-restore.json',result)
    print(json.dumps(result,indent=2))


if __name__ == '__main__':
    main()
