"""Prepare bounded, original voice/language-bound laughter for local review."""
import argparse
import math
from array import array
import base64
import hashlib
import io
import json
import os
import subprocess
import wave
from pathlib import Path
from urllib.parse import urlparse, urlunparse

from dotenv import dotenv_values
from app.core.paths import ROOT
from app.preparation.archive import Archive, encoded
from app.preparation.faces import PreparationError
from app.workflows.speech.qualify_serena import checked_record

VOICE='en-US-Emma:DragonHDLatestNeural'
IDENTITY='owner-requested-emma-laughter-2026-10-27'
OUTPUT=ROOT/'.tools/emma-presence-107'
LANGUAGES=('en-US','es-CO')


def validate_laughter(record, language, voice=VOICE):
    digest,duration=checked_record(record,language,voice=voice)
    if record.get('provider_status')!='SynthesizingAudioCompleted' or duration>2500:
        raise PreparationError('Original laughter must complete inside the existing 2.5-second gate.')
    audio=base64.b64decode(record['audio_base64'],validate=True)
    with wave.open(io.BytesIO(audio)) as wav: pcm=wav.readframes(wav.getnframes())
    samples=array('h');samples.frombytes(pcm)
    rms=math.sqrt(sum(value*value for value in samples)/max(1,len(samples)))/32768
    if rms<.003:raise PreparationError('Silent output is not a qualified laugh.')
    return digest,duration


def catalog():
    values=dotenv_values(ROOT/'.env')
    if values.get('AZURE_SPEECH_REGION')!='canadacentral':raise PreparationError('Use the personal Canada Central resource.')
    folder=OUTPUT/'catalog';receipt=folder/'availability.json'
    if receipt.exists():
        if json.loads(receipt.read_bytes())['voice']!=VOICE:raise PreparationError('Frozen catalog identity differs.')
        print('Existing frozen catalog retained.');return
    script="import os,sys;from urllib.request import Request,urlopen;sys.stdout.buffer.write(urlopen(Request('https://canadacentral.tts.speech.microsoft.com/cognitiveservices/voices/list',headers={'Ocp-Apim-Subscription-Key':os.environ['AZURE_SPEECH_KEY']}),timeout=30).read())"
    result=subprocess.run(['docker','run','--rm','--pull','never','--network','syntelle_default','--dns','1.1.1.1','--dns','8.8.8.8','--env','AZURE_SPEECH_KEY','syntelle-processor','python','-c',script],env={**os.environ,'AZURE_SPEECH_KEY':values['AZURE_SPEECH_KEY']},capture_output=True,timeout=45)
    if result.returncode:raise PreparationError('Catalog unavailable; no synthesis requested.')
    selected=[v for v in json.loads(result.stdout) if v['ShortName']==VOICE]
    if len(selected)!=1:raise PreparationError('Requested voice unavailable; no substitution.')
    folder.mkdir(parents=True);(folder/'regional-voices.json').write_bytes(result.stdout)
    receipt.write_bytes(encoded({'voice':VOICE,'region':'canadacentral','status':'available','selected':selected,'catalog_sha256':hashlib.sha256(result.stdout).hexdigest(),'languages':LANGUAGES,'spanish_nonverbal_qualification':'actual-output-required'}))
    print(json.dumps(Archive().archive_tree(IDENTITY+'/catalog',folder)))


def operations(db):
    with db.connect() as conn:
        return conn.execute('SELECT request_key,status,binding,request,result,diagnostics FROM syntelle_conversation.operations WHERE kind=%s ORDER BY id',(IDENTITY,)).fetchall()


def export(db):
    OUTPUT.mkdir(parents=True,exist_ok=True)
    rows=operations(db)
    (OUTPUT/'complete-operations.json').write_bytes(db.checked(encoded(rows)))
    for row in rows:
        language=row['request_key'];assert language in LANGUAGES
        folder=OUTPUT/'speech'/language;folder.mkdir(parents=True,exist_ok=True)
        (folder/'operation.json').write_bytes(db.checked(encoded(row)))
        record=row.get('result') or {}
        if record.get('audio_base64'):
            target=folder/'response.json';content=encoded(record)
            if target.exists() and json.loads(target.read_bytes())!=record:raise PreparationError('Never replace an original result.')
            target.write_bytes(db.checked(content));(folder/'audio.wav').write_bytes(base64.b64decode(record['audio_base64'],validate=True))
    return rows


def request(language):
    values=dotenv_values(ROOT/'.env');db=Archive()
    receipt=OUTPUT/'catalog/availability.json'
    if not receipt.exists():raise PreparationError('Regional catalog must be frozen first.')
    db.event(IDENTITY,'owner-authorization',{'voice':VOICE,'languages':LANGUAGES,'text':'[laughter]','limits':{'attempts':2,'characters':20},'retries':0,'images':0,'analyses':0})
    db.archive_tree(IDENTITY+'/inputs',OUTPUT/'catalog')
    parsed=urlparse(values['SYNTELLE_DATABASE_URL']);internal=parsed._replace(netloc=parsed.netloc.rsplit('@',1)[0]+'@postgres:5432')
    try:
        result=subprocess.run(['docker','run','--rm','--pull','never','--network','syntelle_default','--dns','1.1.1.1','--dns','8.8.8.8','--env','AZURE_SPEECH_KEY','--env','AZURE_SPEECH_REGION','--env','SYNTELLE_DATABASE_URL','--mount',f'type=bind,source={OUTPUT.as_posix()},target=/experiment,readonly','syntelle-laughter-probe:107','--language',language,'--catalog','/experiment/catalog/availability.json'],env={**os.environ,'AZURE_SPEECH_KEY':values['AZURE_SPEECH_KEY'],'AZURE_SPEECH_REGION':values['AZURE_SPEECH_REGION'],'SYNTELLE_DATABASE_URL':urlunparse(internal)},capture_output=True,timeout=90)
    except subprocess.TimeoutExpired:
        export(db)
        db.archive_tree(IDENTITY+'/timeout',OUTPUT/'speech')
        raise PreparationError('Probe timed out; original database state retained and no retry is allowed.') from None
    folder=OUTPUT/'speech'/language;folder.mkdir(parents=True,exist_ok=True)
    log=folder/'probe.log'
    if not log.exists():log.write_bytes(db.checked(result.stderr))
    rows=export(db)
    print(json.dumps({'language':language,'accepted':result.returncode==0,**db.archive_tree(IDENTITY+'/'+language,folder)}))
    if any(r['status']!='received' and (r.get('diagnostics',{}).get('provider_error') or (r.get('result') or {}).get('provider_status')!='SynthesizingAudioCompleted') for r in rows):
        raise PreparationError('Provider/uncertain failure halts remaining calls; no retry.')


def record_review(language, accepted):
    db=Archive();export(db)
    record=json.loads((OUTPUT/'speech'/language/'response.json').read_bytes())
    digest,_=validate_laughter(record,language)
    receipt={'voice':VOICE,'language':language,'audio_sha256':digest,'accepted':accepted,'reviewer':'owner','criterion':'audible carcajada, compatible voice and complete ending'}
    folder=OUTPUT/'audible-review';folder.mkdir(exist_ok=True)
    target=folder/(language+'.json');content=encoded(receipt)
    if target.exists() and target.read_bytes()!=content:raise PreparationError('Preserve the earlier listening decision; create an explicit review revision.')
    db.event(IDENTITY,'owner-audible-review',receipt);target.write_bytes(content)
    print(json.dumps(db.archive_tree(IDENTITY+'/audible-review',folder)))


def prepare(profile_name='profile'):
    if not profile_name or any(c not in 'abcdefghijklmnopqrstuvwxyz0123456789-' for c in profile_name):raise PreparationError('Use a simple local profile revision name.')
    db=Archive();rows=export(db)
    profile=json.loads((ROOT/'.tools/av-sync-emma-106/profile/profile.json').read_bytes())
    capability=json.loads((ROOT/'.tools/av-sync-emma-106/profile/voice-capabilities.json').read_bytes())
    assert profile['voice']==capability['voice']==VOICE
    profile['laughter']={};evidence={}
    for row in rows:
        language=row['request_key'];record=row.get('result') or {}
        try:
            digest,duration=validate_laughter(record,language)
        except (PreparationError,KeyError,ValueError) as error:
            evidence[language]={'accepted':False,'reason':str(error)};continue
        if row['status']!='received':
            evidence[language]={'accepted':False,'reason':'Original application rejection retained.'};continue
        review=OUTPUT/'audible-review'/(language+'.json')
        audible=json.loads(review.read_bytes()) if review.exists() else {}
        accepted=audible.get('accepted') is True and audible.get('audio_sha256')==digest and audible.get('voice')==VOICE
        if accepted:profile['laughter'][VOICE+'|'+language]=record
        evidence[language]={'technical_accepted':True,'accepted':accepted,'sha256':digest,'duration_ms':duration,'audible_owner_review':'accepted' if accepted else ('rejected' if review.exists() else 'pending')}
    folder=OUTPUT/profile_name;folder.mkdir(exist_ok=True)
    for name,value in [('profile.json',profile),('voice-capabilities.json',capability),('laughter-qualification.json',evidence)]:
        target=folder/name;content=encoded(value)
        if target.exists() and target.read_bytes()!=content:raise PreparationError('Preserve published profile; export a new candidate revision.')
        target.write_bytes(content)
    print(json.dumps({'languages':evidence,**db.archive_tree(IDENTITY+'/profile',folder)}))


def main():
    parser=argparse.ArgumentParser(description=__doc__);group=parser.add_mutually_exclusive_group(required=True)
    group.add_argument('--catalog',action='store_true');group.add_argument('--language',choices=LANGUAGES);group.add_argument('--prepare',action='store_true');group.add_argument('--export',action='store_true');group.add_argument('--review',choices=LANGUAGES)
    parser.add_argument('--decision',choices=['accept','reject']);parser.add_argument('--profile-name',default='profile')
    args=parser.parse_args()
    if args.catalog:catalog()
    elif args.language:request(args.language)
    elif args.prepare:prepare(args.profile_name)
    elif args.review:
        if not args.decision:parser.error('--review requires an explicit --decision')
        record_review(args.review,args.decision=='accept')
    else:export(Archive())

if __name__=='__main__':main()
