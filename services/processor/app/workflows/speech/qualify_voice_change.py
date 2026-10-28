"""Verify the owner-requested Emma voice without changing historical budgets."""
import argparse
import base64
import hashlib
import json
import os
import subprocess
from urllib.parse import urlparse, urlunparse

from dotenv import dotenv_values

from app.core.paths import ROOT
from app.preparation.archive import Archive, encoded
from app.preparation.faces import PreparationError
from app.workflows.speech.qualify_serena import checked_record as check_original_record

VOICE = 'en-US-Emma:DragonHDLatestNeural'
IDENTITY = 'owner-requested-emma-2026-10-27'
OUTPUT = ROOT/'.tools/av-sync-emma-106'


def catalog():
    values=dotenv_values(ROOT/'.env')
    if values.get('AZURE_SPEECH_REGION')!='canadacentral':raise PreparationError('Use the personal Canada Central resource.')
    script="import os,sys;from urllib.request import Request,urlopen;sys.stdout.buffer.write(urlopen(Request('https://canadacentral.tts.speech.microsoft.com/cognitiveservices/voices/list',headers={'Ocp-Apim-Subscription-Key':os.environ['AZURE_SPEECH_KEY']}),timeout=30).read())"
    result=subprocess.run(['docker','run','--rm','--pull','never','--network','syntelle_default','--dns','1.1.1.1','--dns','8.8.8.8','--env','AZURE_SPEECH_KEY','syntelle-processor','python','-c',script],env={**os.environ,'AZURE_SPEECH_KEY':values['AZURE_SPEECH_KEY']},capture_output=True,timeout=45)
    if result.returncode:raise PreparationError('Regional voice catalog unavailable; no synthesis requested.')
    selected=[v for v in json.loads(result.stdout) if v['ShortName']==VOICE]
    if len(selected)!=1:raise PreparationError('Requested Emma voice is unavailable; no substitution.')
    locales={selected[0]['Locale'],*selected[0].get('SecondaryLocaleList',[])}
    if not {'en','es'}.issubset({locale.split('-')[0] for locale in locales}):raise PreparationError('Requested voice languages are unavailable.')
    folder=OUTPUT/'catalog';folder.mkdir(exist_ok=True)
    (folder/'regional-voices.json').write_bytes(result.stdout)
    receipt={'voice':VOICE,'region':'canadacentral','status':'available','selected':selected,'requested_locales':['en-US','es-CO'],'qualification_note':'Catalog confirms English/Spanish; es-CO is separately verified by the bounded native output.','catalog_sha256':hashlib.sha256(result.stdout).hexdigest()}
    (folder/'availability.json').write_bytes(encoded(receipt))
    print(json.dumps(Archive().archive_tree(IDENTITY+'/catalog',folder)))


def run(language):
    values=dotenv_values(ROOT/'.env');folder=OUTPUT/'speech'/language;folder.mkdir(parents=True,exist_ok=True)
    receipt=OUTPUT/'catalog/availability.json'
    if not receipt.exists():raise PreparationError('Verify regional availability before synthesis.')
    db=Archive();db.event(IDENTITY,'owner-requested-voice-change',{'voice':VOICE,'languages':['en-US','es-CO'],'limits':{'attempts':2,'characters':1000},'retries':0})
    db.archive_tree(IDENTITY+'/inputs',OUTPUT/'catalog')
    parsed=urlparse(values['SYNTELLE_DATABASE_URL']);internal=parsed._replace(netloc=parsed.netloc.rsplit('@',1)[0]+'@postgres:5432')
    result=subprocess.run(['docker','run','--rm','--pull','never','--network','syntelle_default','--dns','1.1.1.1','--dns','8.8.8.8','--env','AZURE_SPEECH_KEY','--env','AZURE_SPEECH_REGION','--env','SYNTELLE_DATABASE_URL','--mount',f'type=bind,source={OUTPUT.as_posix()},target=/experiment,readonly','syntelle-voice-check:106','--language',language,'--catalog','/experiment/catalog/availability.json'],env={**os.environ,'AZURE_SPEECH_KEY':values['AZURE_SPEECH_KEY'],'AZURE_SPEECH_REGION':values['AZURE_SPEECH_REGION'],'SYNTELLE_DATABASE_URL':urlunparse(internal)},capture_output=True,timeout=90)
    for name,content in [('response.json',result.stdout),('provider.log',result.stderr)]:
        db.checked(content);target=folder/name
        if target.exists() and target.read_bytes()!=content:raise PreparationError('Preserve prior output; do not retry or overwrite.')
        target.write_bytes(content)
    if result.returncode==0:
        record=json.loads(result.stdout)
        (folder/'audio.wav').write_bytes(base64.b64decode(record['audio_base64'],validate=True))
    print(json.dumps({'success':result.returncode==0,**db.archive_tree(IDENTITY+'/'+language,folder)}))
    if result.returncode:raise PreparationError('Voice verification stopped without retries.')


def prepare():
    # Reuse the PCM/event validator while retaining the exact requested voice.
    evidence={}
    for language in ('en-US','es-CO'):
        record=json.loads((OUTPUT/'speech'/language/'response.json').read_bytes())
        evidence[language]=check_original_record(record,language,voice=VOICE)[0]
    profile=json.loads((ROOT/'.tools/coordinated-performance-103/active-profile-v2/profile.json').read_bytes())
    profile.update(voice=VOICE,laughter={},voice_review='owner-pending')
    capability={'version':'voice-capabilities/1','revision':hashlib.sha256(encoded(evidence)).hexdigest(),'voice':VOICE,'region':'canadacentral','locales':{lang:{'available':True,'styles':[]} for lang in evidence},'audio_sha256':evidence,'qualification':'technical-local-review'}
    folder=OUTPUT/'profile';folder.mkdir(exist_ok=True)
    for name,value in [('profile.json',profile),('voice-capabilities.json',capability)]:
        target=folder/name;content=encoded(value)
        if target.exists() and target.read_bytes()!=content:raise PreparationError('Export a new version instead of replacing evidence.')
        target.write_bytes(content)
    print(json.dumps(Archive().archive_tree(IDENTITY+'/profile',folder)))


if __name__=='__main__':
    parser=argparse.ArgumentParser(description=__doc__);choice=parser.add_mutually_exclusive_group(required=True)
    choice.add_argument('--catalog',action='store_true');choice.add_argument('--language',choices=['en-US','es-CO']);choice.add_argument('--prepare',action='store_true')
    args=parser.parse_args();catalog() if args.catalog else prepare() if args.prepare else run(args.language)
