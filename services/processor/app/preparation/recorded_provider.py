"""Commit a PostgreSQL request reservation and input snapshot before one POST."""
from hashlib import sha256
import json

from app.gemini_contour_trial import post_once
from .archive import encoded
from .faces import PreparationError


class RecordedProvider:
    def __init__(self,archive,budget,root,kind):
        self.archive=archive;self.budget=budget;self.root=root.resolve();self.kind=kind

    def __call__(self,payload,settings,key,directory):
        directory=directory.resolve()
        if not directory.is_relative_to(self.root):raise PreparationError('Request escaped its authorized batch directory.')
        name=directory.relative_to(self.root).as_posix();binding=sha256(encoded(payload)).hexdigest()
        experiment=f'{self.budget}/{name}'
        self.archive.archive_tree(experiment,directory,{'kind':self.kind,'model':payload['model']})
        id,created=self.archive.reserve(self.budget,name,self.kind,binding,{'model':payload['model'],'experiment':experiment})
        if not created:raise PreparationError('A received database request already exists. Restore/reuse its cached response; do not call again.')
        status='failed';usage={}
        try:
            raw,http_status,duration=post_once(payload,settings,key)
            (directory/'response.json').write_bytes(raw)
            body=json.loads(raw);usage=body.get('usage',{})
            if 200<=http_status<300 and body.get('status') in (None,'completed'):status='received'
            return raw,http_status,duration
        finally:
            # If PostgreSQL becomes unavailable after the POST, the committed
            # 'started' record remains conservative: no second request starts.
            self.archive.archive_tree(experiment,directory)
            self.archive.finish(id,status,usage)
