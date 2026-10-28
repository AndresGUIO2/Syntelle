import base64
import io
import wave

import pytest

from app.preparation.faces import PreparationError
from app.workflows.speech.qualify_laughter import VOICE, validate_laughter


def original_laugh():
    output=io.BytesIO()
    with wave.open(output,'wb') as wav:
        wav.setparams((1,2,24000,0,'NONE','not compressed'))
        wav.writeframes((b"\xd0\x07\x30\xf8")*12000)
    return {'voice':VOICE,'language':'en-US','provider_status':'SynthesizingAudioCompleted','audio_base64':base64.b64encode(output.getvalue()).decode(),'duration_ms':1000,'visemes':[{'id':0,'offset_ms':0},{'id':0,'offset_ms':1000}]}


def test_nonverbal_rest_timeline_keeps_original_pcm_and_identity():
    value=original_laugh();original=value['audio_base64']
    digest,duration=validate_laughter(value,'en-US')
    assert len(digest)==64 and duration==1000
    assert value['audio_base64']==original


@pytest.mark.parametrize('field,value',[('voice','en-US-Serena:DragonHDLatestNeural'),('language','es-CO'),('provider_status','Canceled'),('duration_ms',3000)])
def test_invalid_laugh_never_becomes_qualified(field,value):
    record=original_laugh();record[field]=value
    with pytest.raises(PreparationError):validate_laughter(record,'en-US')


def test_invalid_native_events_are_rejected():
    record=original_laugh();record['visemes'][1]['offset_ms']=-1
    with pytest.raises(PreparationError):validate_laughter(record,'en-US')


def test_audible_review_is_bound_to_original_audio_before_enabling(tmp_path,monkeypatch):
    import json
    from app.workflows.speech import qualify_laughter as q
    base=tmp_path/'.tools/av-sync-emma-106/profile';base.mkdir(parents=True)
    (base/'profile.json').write_text(json.dumps({'voice':VOICE,'laughter':{}}))
    (base/'voice-capabilities.json').write_text(json.dumps({'voice':VOICE}))
    output=tmp_path/'candidate';output.mkdir()
    class Database:
        def archive_tree(self,*args):return {'snapshot_id':0}
    monkeypatch.setattr(q,'ROOT',tmp_path);monkeypatch.setattr(q,'OUTPUT',output);monkeypatch.setattr(q,'Archive',Database)
    record=original_laugh();monkeypatch.setattr(q,'export',lambda db:[{'request_key':'en-US','status':'received','result':record}])
    q.prepare()
    assert json.loads((output/'profile/profile.json').read_text())['laughter']=={}
    reviews=output/'audible-review';reviews.mkdir()
    digest,_=q.validate_laughter(record,'en-US')
    (reviews/'en-US.json').write_text(json.dumps({'voice':VOICE,'accepted':True,'audio_sha256':digest}))
    q.prepare('profile-reviewed')
    assert list(json.loads((output/'profile-reviewed/profile.json').read_text())['laughter'])==[VOICE+'|en-US']
