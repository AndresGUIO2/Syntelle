"""PostgreSQL content archive and transactional limits for offline preparation.

Files remain a convenient working cache. Immutable database snapshots contain
their actual bytes and can restore a complete run without its original folder.
No request may start when its budget or database record cannot be committed.
"""
from hashlib import sha256
import json
import os
from pathlib import Path, PurePosixPath
from urllib.parse import unquote, urlparse

from dotenv import dotenv_values
import psycopg
from psycopg import sql
from psycopg.rows import dict_row
from psycopg.types.json import Jsonb

from .faces import PreparationError

ROOT=Path(__file__).resolve().parents[4]


def encoded(value): return json.dumps(value,sort_keys=True,separators=(',',':')).encode()


def local_connection():
    values=dotenv_values(ROOT/'.env')
    dsn=os.environ.get('SYNTELLE_DATABASE_URL') or values.get('SYNTELLE_DATABASE_URL')
    if not dsn: raise PreparationError('Configure SYNTELLE_DATABASE_URL in local secret configuration.')
    secrets=[str(v).encode() for k,v in {**values,**os.environ}.items()
             if v and any(token in k.upper() for token in ('API_KEY','PASSWORD','SECRET','ACCESS_TOKEN'))]
    password=urlparse(dsn).password
    if password: secrets.append(unquote(password).encode())
    return dsn,secrets


class Archive:
    def __init__(self,dsn=None,*,schema='syntelle_preparation',secrets=None):
        if dsn is None: dsn,configured=local_connection()
        else: configured=[]
        self.dsn=dsn;self.schema=schema;self.secrets=[*configured,*(secrets or [])]
        password=urlparse(dsn).password
        if password:self.secrets.append(unquote(password).encode())
        try:
            with psycopg.connect(dsn,connect_timeout=5) as conn:
                conn.execute('SELECT pg_advisory_xact_lock(836528914)')
                conn.execute(sql.SQL('CREATE SCHEMA IF NOT EXISTS {}').format(sql.Identifier(schema)))
                conn.execute(sql.SQL('SET search_path TO {}').format(sql.Identifier(schema)))
                conn.execute(Path(__file__).with_name('archive.sql').read_text())
        except psycopg.Error:
            raise PreparationError('PostgreSQL archive is unavailable; no provider request is allowed.') from None

    def connect(self):
        try:
            conn=psycopg.connect(self.dsn,connect_timeout=5,row_factory=dict_row)
            conn.execute(sql.SQL('SET search_path TO {}').format(sql.Identifier(self.schema)))
            return conn
        except psycopg.Error:
            raise PreparationError('PostgreSQL archive connection failed; credentials were not recorded.') from None

    def checked(self,data):
        if any(secret and secret in data for secret in self.secrets):
            raise PreparationError('Secret material cannot enter the preparation archive.')
        return data

    def put(self,conn,data):
        self.checked(data);digest=sha256(data).hexdigest()
        conn.execute('INSERT INTO blobs(sha256,data,byte_length) VALUES (%s,%s,%s) ON CONFLICT DO NOTHING',
                     (digest,data,len(data)))
        return digest

    def experiment(self,conn,name,metadata=None):
        value=metadata or {};self.checked(encoded({'id':name,'metadata':value}))
        conn.execute('INSERT INTO experiments(id,metadata) VALUES (%s,%s) ON CONFLICT DO NOTHING',(name,Jsonb(value)))

    def archive_tree(self,name,directory,metadata=None):
        directory=directory.resolve();files=[]
        with self.connect() as conn:
            self.experiment(conn,name,metadata)
            for path in sorted(directory.rglob('*')):
                if path.is_symlink():raise PreparationError('Do not archive symlinked preparation inputs.')
                if not path.is_file():continue
                if path.name=='.env' or path.name.startswith('.env.'):
                    raise PreparationError('Secret configuration files cannot be archived.')
                if '__pycache__' in path.parts or path.suffix in ('.pyc','.tmp'):continue
                data=path.read_bytes();digest=self.put(conn,data)
                files.append({'path':path.relative_to(directory).as_posix(),'sha256':digest,'bytes':len(data)})
            manifest=self.put(conn,encoded(files))
            row=conn.execute('INSERT INTO snapshots(experiment_id,manifest_sha256) VALUES (%s,%s) ON CONFLICT (experiment_id,manifest_sha256) DO UPDATE SET experiment_id=excluded.experiment_id RETURNING id',
                             (name,manifest)).fetchone()
            for file in files:
                conn.execute('INSERT INTO snapshot_files(snapshot_id,path,blob_sha256) VALUES (%s,%s,%s) ON CONFLICT DO NOTHING',
                             (row['id'],file['path'],file['sha256']))
            return {'snapshot_id':row['id'],'files':len(files),'manifest_sha256':manifest}

    def event(self,name,kind,record):
        with self.connect() as conn:
            self.experiment(conn,name);digest=self.put(conn,encoded(record))
            conn.execute('INSERT INTO events(experiment_id,kind,record_sha256) VALUES (%s,%s,%s) ON CONFLICT DO NOTHING',(name,kind,digest))

    def restore(self,snapshot,destination):
        destination=destination.resolve()
        with self.connect() as conn:
            # Complete preparation snapshots include models and many rejected
            # renderings. Bound restore memory without changing stored evidence.
            rows=conn.cursor(name='preparation_restore')
            rows.itersize=4
            rows.execute('SELECT f.path,b.sha256,b.data FROM snapshot_files f JOIN blobs b ON f.blob_sha256=b.sha256 WHERE f.snapshot_id=%s ORDER BY f.path',(snapshot,))
            restored=0
            for row in rows:
                name=PurePosixPath(row['path'])
                if name.is_absolute() or '..' in name.parts or '\\' in row['path']:
                    raise PreparationError('Unsafe archived path.')
                path=(destination/row['path']).resolve()
                if not path.is_relative_to(destination):raise PreparationError('Restored path escapes its destination.')
                data=bytes(row['data'])
                if sha256(data).hexdigest()!=row['sha256']:raise PreparationError('Database blob checksum failed.')
                if path.exists() and path.read_bytes()!=data:raise PreparationError('Restore refuses to overwrite a different working file.')
                path.parent.mkdir(parents=True,exist_ok=True);path.write_bytes(data)
                restored+=1
            if not restored:raise PreparationError('Snapshot is missing or empty.')
        return restored

    def budget(self,name,binding,limits):
        self.checked(encoded({'id':name,'binding':binding,'limits':limits}))
        if set(limits)!= {'image','analysis'} or any(type(v)!=int or v<0 for v in limits.values()):
            raise PreparationError('Invalid preparation request limits.')
        with self.connect() as conn:
            conn.execute('INSERT INTO budgets(id,binding,limits) VALUES (%s,%s,%s) ON CONFLICT DO NOTHING',(name,binding,Jsonb(limits)))
            row=conn.execute('SELECT binding,limits FROM budgets WHERE id=%s',(name,)).fetchone()
            if row['binding']!=binding or row['limits']!=limits:raise PreparationError('A persisted request budget cannot be reset or changed.')

    def reserve(self,budget,key,kind,binding,metadata=None):
        if kind not in ('image','analysis'):raise PreparationError('Unknown request kind.')
        self.checked(encoded({'budget':budget,'key':key,'binding':binding,'metadata':metadata or {}}))
        with self.connect() as conn:
            scope=conn.execute('SELECT * FROM budgets WHERE id=%s FOR UPDATE',(budget,)).fetchone()
            if not scope:raise PreparationError('No authorized database budget.')
            existing=conn.execute('SELECT * FROM requests WHERE budget_id=%s AND request_key=%s',(budget,key)).fetchone()
            if existing:
                if existing['binding']!=binding:raise PreparationError('Request identity changed; no paid retry.')
                if existing['status']!='received':raise PreparationError('Previous request failed or may have incurred cost; no retry.')
                return existing['id'],False
            if scope['halted']:raise PreparationError('A provider failure stopped this batch.')
            if conn.execute("SELECT 1 FROM requests WHERE budget_id=%s AND status='started'",(budget,)).fetchone():
                raise PreparationError('An earlier request may still be running; reconcile it before starting another.')
            count=conn.execute('SELECT count(*) AS n FROM requests WHERE budget_id=%s AND kind=%s',(budget,kind)).fetchone()['n']
            if count>=scope['limits'][kind]:raise PreparationError('Authorized request budget exhausted.')
            row=conn.execute("INSERT INTO requests(budget_id,request_key,kind,binding,status,metadata) VALUES (%s,%s,%s,%s,'started',%s) RETURNING id",(budget,key,kind,binding,Jsonb(metadata or {}))).fetchone()
            return row['id'],True

    def finish(self,request,status,usage=None):
        if status not in ('received','failed'):raise PreparationError('Invalid terminal provider status.')
        self.checked(encoded(usage or {}))
        with self.connect() as conn:
            row=conn.execute('SELECT * FROM requests WHERE id=%s FOR UPDATE',(request,)).fetchone()
            if not row:raise PreparationError('Unknown database request.')
            if row['status']!='started':
                if row['status']==status and row['usage']==(usage or {}):return
                raise PreparationError('Terminal provider records are immutable.')
            conn.execute('UPDATE requests SET status=%s,usage=%s,finished_at=now() WHERE id=%s',(status,Jsonb(usage or {}),request))
            if status=='failed':conn.execute('UPDATE budgets SET halted=true WHERE id=%s',(row['budget_id'],))

    def summary(self):
        with self.connect() as conn:
            result={table:conn.execute(sql.SQL('SELECT count(*) AS n FROM {}').format(sql.Identifier(table))).fetchone()['n']
                    for table in ('experiments','snapshots','snapshot_files','blobs','events','requests')}
            result['stored_bytes']=int(conn.execute('SELECT coalesce(sum(byte_length),0) AS n FROM blobs').fetchone()['n'])
            return result
