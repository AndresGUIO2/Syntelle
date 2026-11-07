"""Fixed owner-approved Emma emotion preparation, with immutable PG evidence."""
import argparse
import base64
import hashlib
import json
import os
from pathlib import Path
import subprocess
from urllib.parse import urlparse, urlunparse

from dotenv import dotenv_values
from app.core.paths import ROOT
from app.preparation.archive import Archive
from app.preparation.faces import PreparationError
from app.workflows.speech.qualify_serena import checked_record

IDENTITY = 'owner-approved-emma-emotion-2026-10-28'
VOICE = 'en-US-Emma:DragonHDLatestNeural'
FOLDER = ROOT/'.tools/emotional-presence-114'
TEXTS = ROOT/'docs/characters/emma-emotion-experiment.json'
STYLES = ('fearful', 'sad', 'angry', 'reassuring', 'amused', 'curious')

def settings():
    values = dotenv_values(ROOT/'.env')
    if values.get('AZURE_SPEECH_REGION') != 'canadacentral':
        raise PreparationError('Use the personal Canada Central resource.')
    return values

def catalog():
    values = settings()
    folder = FOLDER/'experiment'
    folder.mkdir(parents=True, exist_ok=True)
    receipt = folder/'regional-availability.json'
    if receipt.exists():
        raise PreparationError('Preserve the frozen catalog; do not overwrite it.')
    script = "import os,sys;from urllib.request import Request,urlopen;sys.stdout.buffer.write(urlopen(Request('https://canadacentral.tts.speech.microsoft.com/cognitiveservices/voices/list',headers={'Ocp-Apim-Subscription-Key':os.environ['AZURE_SPEECH_KEY']}),timeout=30).read())"
    result = subprocess.run(['docker','run','--rm','--pull','never','--network','syntelle_default','--dns','1.1.1.1','--dns','8.8.8.8','--env','AZURE_SPEECH_KEY','syntelle-processor','python','-c',script],env={**os.environ,'AZURE_SPEECH_KEY':values['AZURE_SPEECH_KEY']},capture_output=True,timeout=45)
    (folder/'catalog-response.json').write_bytes(result.stdout)
    if result.returncode:
        raise PreparationError('Catalog read failed; no synthesis requested.')
    selected = [v for v in json.loads(result.stdout) if v.get('ShortName') == VOICE]
    record = {'voice':VOICE,'region':'canadacentral','status':'available' if selected else 'unavailable',
              'catalog_sha256':hashlib.sha256(result.stdout).hexdigest(),'selected':selected,'synthesis_attempts':0}
    receipt.write_text(json.dumps(record,indent=2),encoding='utf-8')
    frozen = folder/'experiment.json'
    if frozen.exists():
        raise PreparationError('Preserve fixed experiment inputs.')
    frozen.write_bytes(TEXTS.read_bytes())
    db = Archive()
    db.event(IDENTITY,'authorization',json.loads(frozen.read_bytes()))
    archived = db.archive_tree(IDENTITY+'/inputs',folder,record)
    print(json.dumps({'status':record['status'],**archived}))
    if not selected:
        raise PreparationError('Exact Emma unavailable; no synthesis requested.')

def synthesize(style, mode):
    values = settings()
    frozen = FOLDER/'experiment/experiment.json'
    spec = json.loads(frozen.read_bytes())
    if spec['id'] != IDENTITY or spec['limits'] != {'attempts':12,'characters':3000,'image':0,'analysis':0} or tuple(spec['texts']) != STYLES:
        raise PreparationError('Preserve the fixed authorized experiment.')
    slot = style+'-'+mode
    output = FOLDER/'speech'/slot
    output.mkdir(parents=True,exist_ok=False)
    parsed = urlparse(values['SYNTELLE_DATABASE_URL'])
    internal = parsed._replace(netloc=parsed.netloc.rsplit('@',1)[0]+'@postgres:5432')
    env = {**os.environ,'AZURE_SPEECH_KEY':values['AZURE_SPEECH_KEY'],'AZURE_SPEECH_REGION':'canadacentral','SYNTELLE_DATABASE_URL':urlunparse(internal)}
    result = subprocess.run(['docker','run','--rm','--pull','never','--network','syntelle_default','--dns','1.1.1.1','--dns','8.8.8.8','--env','AZURE_SPEECH_KEY','--env','AZURE_SPEECH_REGION','--env','SYNTELLE_DATABASE_URL','--mount',f'type=bind,source={(FOLDER/"experiment").as_posix()},target=/experiment,readonly','syntelle-emotion-probe:114','-style',style,'-mode',mode,'-catalog','/experiment/regional-availability.json','-experiment','/experiment/experiment.json'],env=env,capture_output=True,timeout=100)
    db = Archive()
    for name,data in [('response.json',result.stdout),('provider.log',result.stderr)]:
        db.checked(data)
        (output/name).write_bytes(data)
    technical = {'slot':slot,'accepted':False,'owner_listening':'pending','provider_retries':0}
    if result.stdout.strip():
        record = json.loads(result.stdout)
        if record.get('audio_base64'):
            (output/'audio.wav').write_bytes(base64.b64decode(record['audio_base64'],validate=True))
        if result.returncode == 0:
            checked_record(record,'en-US',voice=VOICE)
            technical['accepted'] = True
            technical['audio_sha256'] = hashlib.sha256((output/'audio.wav').read_bytes()).hexdigest()
    (output/'technical.json').write_text(json.dumps(technical,indent=2),encoding='utf-8')
    archived = db.archive_tree(IDENTITY+'/'+slot,output,technical)
    print(json.dumps({**technical,**archived}))
    if result.returncode or not technical['accepted']:
        raise PreparationError('Qualification halted; preserve original output without retry.')

def review(style, decision, statement):
    records = {}
    for mode in ('plain','directed'):
        record = json.loads((FOLDER/'speech'/(style+'-'+mode)/'response.json').read_bytes())
        checked_record(record,'en-US',voice=VOICE)
        records[mode] = hashlib.sha256(base64.b64decode(record['audio_base64'])).hexdigest()
    folder = FOLDER/'audible-review'/style
    folder.mkdir(parents=True,exist_ok=False)
    receipt = {'version':'voice-style-owner-review/1','voice':VOICE,'language':'en-US','style':style,
               'audio_sha256':records,'decision':decision,'qualified':decision=='accepted',
               'owner_statement':statement,'synthesis_calls':0}
    (folder/'receipt.json').write_text(json.dumps(receipt,indent=2),encoding='utf-8')
    db = Archive()
    db.event(IDENTITY,'owner-listening-review',receipt)
    archived = db.archive_tree(IDENTITY+'/owner-review/'+style,folder,receipt)
    print(json.dumps({'qualified':receipt['qualified'],**archived}))

def export_profile(package=None, output=None):
    package = package or FOLDER/'range-v3/prepared/package'
    manifest=json.loads((package/'manifest.json').read_bytes())
    profile=json.loads((FOLDER/'baseline-profile/profile.json').read_bytes())
    old_cap=json.loads((FOLDER/'baseline-profile/voice-capabilities.json').read_bytes())
    receipts={}
    for style in STYLES:
        receipt=json.loads((FOLDER/'audible-review'/style/'receipt.json').read_bytes())
        if receipt['voice']!=VOICE or receipt['language']!='en-US' or receipt['style']!=style:
            raise PreparationError('Owner review identity mismatch.')
        for mode in ('plain','directed'):
            original=(FOLDER/'speech'/(style+'-'+mode)/'audio.wav').read_bytes()
            if hashlib.sha256(original).hexdigest()!=receipt['audio_sha256'][mode]:
                raise PreparationError('Owner review no longer matches original audio.')
        receipts[style]=receipt
    accepted=[style for style in STYLES if receipts[style]['qualified'] and receipts[style]['decision']=='accepted']
    cap={**old_cap,'locales':{'en-US':{'available':True,'styles':accepted},'es-CO':{'available':True,'styles':[]}},
         'qualification':'hash-bound-owner-audible-review','style_reviews':receipts}
    cap['revision']=hashlib.sha256(json.dumps(cap,sort_keys=True,separators=(',',':')).encode()).hexdigest()
    profile['package_hash']=manifest['package_hash']
    profile['emotional_presence']=manifest['packages']['motion']['emotional_presence']
    folder=output or FOLDER/'profile-qualified';folder.mkdir(exist_ok=False)
    for name,data in [('profile.json',profile),('voice-capabilities.json',cap)]:
        (folder/name).write_text(json.dumps(data,indent=2),encoding='utf-8')
    print(json.dumps({'package_hash':profile['package_hash'],'English_styles':accepted,'Spanish_styles':[],
                      'fearful':'weak; not activated','laughter_preserved':True,'provider_calls':0}))

if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('action',choices=['catalog','synthesize','review','export-profile'])
    parser.add_argument('--style',choices=STYLES)
    parser.add_argument('--mode',choices=['plain','directed'])
    parser.add_argument('--decision',choices=['accepted','weak','rejected'])
    parser.add_argument('--statement')
    parser.add_argument('--package',type=Path)
    parser.add_argument('--output',type=Path)
    args = parser.parse_args()
    if args.action == 'catalog': catalog()
    elif args.action == 'export-profile': export_profile(args.package,args.output)
    elif args.action == 'review' and args.style and args.decision and args.statement: review(args.style,args.decision,args.statement)
    elif args.style and args.mode: synthesize(args.style,args.mode)
    else: parser.error('Choose a fixed style and mode.')
