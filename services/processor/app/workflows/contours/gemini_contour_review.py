"""One recorded Argon review of the existing EI Flash proposal; no image generation."""
import argparse
import json
from pathlib import Path

from PIL import Image, ImageDraw

from app.workflows.contours.gemini_contour_loop import capture_round, checked_files, digest, evaluate_round, round_request
from app.workflows.contours.gemini_contour_trial import stamp
from app.preparation.faces import PreparationError
from app.workflows.visemes.viseme_trial import ROOT, write_report

SETTINGS=ROOT/'docs/characters/syntelle/gemini-contour-argon-review.json'
PROMPT=ROOT/'docs/prompts/dental-contours-argon-review-v1.txt'


def comparison(source, output):
    names=[('Original EI',source/'input-native.png'),('Flash proposal',source/'proposal/mask-overlay.png'),
           ('Argon review: unapproved',output/'review/mask-overlay.png')]
    width,height=680,464
    sheet=Image.new('RGB',(width*3,height+90),'#17161B');draw=ImageDraw.Draw(sheet)
    for index,(title,path) in enumerate(names):
        draw.text((index*width+12,12),title,fill='white',font_size=23)
        sheet.paste(Image.open(path).convert('RGB').resize((width,height),Image.Resampling.LANCZOS),(index*width,48))
    draw.text((12,height+60),'Blue: upper teeth. Pink: lower teeth. No manual contour correction or active installation.',fill='white',font_size=20)
    sheet.save(output/'comparison.png')


def prepare(source,output,request=False,settings_path=SETTINGS,prompt_path=PROMPT):
    settings=json.loads(settings_path.read_text(encoding='utf-8'))
    if (settings['model']!='gemini-4-argon' or settings['maximum_requests']!=1 or settings['retries']!=0
            or settings['follow_redirects'] or settings['store']
            or settings['endpoint']!='https://generativelanguage.googleapis.com/v1beta/interactions'
            or settings['generation_config']['thinking_level']!='medium'
            or settings['generation_config']['max_output_tokens']!=24576):
        raise PreparationError('This review permits one bounded Argon analysis without retries.')
    source_record=json.loads((source/'run.json').read_text(encoding='utf-8'))
    checked_files(source,source_record['input_hashes'])
    proposal_record=json.loads((source/'proposal/run.json').read_text(encoding='utf-8'))
    checked_files(source/'proposal',proposal_record['cached_hashes'])
    names=['input-native.png','clean.png','grid.png','mediapipe-lips.png','mediapipe.json',
           'correction-feedback.png','proposal/output.txt','proposal/validation.json']
    binding={'source':str(source.resolve()),'source_sha256':{name:digest((source/name).read_bytes()) for name in names},
             'settings_sha256':digest(settings_path.read_bytes()),'prompt_sha256':digest(prompt_path.read_bytes())}
    ledger=output/'run.json'
    if ledger.exists():
        record=json.loads(ledger.read_text(encoding='utf-8'))
        if record['binding']!=binding:raise PreparationError('Review inputs changed; no request.')
        if record['status']!='completed':raise PreparationError('Previous review stopped or is incomplete; no retry.')
        checked_files(output,record['artifact_hashes'])
        return record
    output.mkdir(parents=True,exist_ok=True)
    record={'schema':'syntelle-gemini-contour-review/1','binding':binding,'status':'prepared','started_at':stamp(),
            'maximum_requests':1,'new_viseme_generations':0,'manual_mask_inputs':False,'active_mask_installation':False}
    with ledger.open('x',encoding='utf-8') as handle:json.dump(record,handle,indent=2)
    for name in names:
        target=output/'inputs'/name;target.parent.mkdir(parents=True,exist_ok=True);target.write_bytes((source/name).read_bytes())
    (output/'settings.json').write_bytes(settings_path.read_bytes());(output/'prompt-template.txt').write_bytes(prompt_path.read_bytes())
    for name in ('gemini_contour_review.py','gemini_contour_loop.py','gemini_contour_trial.py'):
        (output/name).write_bytes(Path(__file__).with_name(name).read_bytes())
    prompt=prompt_path.read_text(encoding='utf-8').replace('{{PREVIOUS_CONTOURS}}',(source/'proposal/output.txt').read_text(encoding='utf-8')).replace('{{VALIDATION_REPORT}}',(source/'proposal/validation.json').read_text(encoding='utf-8'))
    views={name:(source/name).read_bytes() for name in ('clean.png','grid.png','mediapipe-lips.png')}
    payload=round_request(prompt,views,settings,(source/'correction-feedback.png').read_bytes())
    try:
        text,provider=capture_round(output/'review',payload,settings,request)
        record['provider']=provider
        _,quality=evaluate_round((source/'input-native.png').read_bytes(),text,source_record['binding']['input'],
                                 json.loads((source/'mediapipe.json').read_text(encoding='utf-8')),settings,output/'review')
        record['validation']=quality
        comparison(source,output)
        record['status']='completed'
    except (ValueError,PreparationError) as exc:
        record.update({'status':'stopped','error':str(exc)[:800]})
        raise PreparationError(str(exc)) from None
    finally:
        record['finished_at']=stamp()
        record['artifact_hashes']={str(p.relative_to(output)):digest(p.read_bytes()) for p in output.rglob('*') if p.is_file() and p!=ledger}
        write_report(ledger,record)
    return record


if __name__=='__main__':
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--source',type=Path,default=ROOT/'.tools/gemini-contour-loop-ei-01')
    parser.add_argument('--output',type=Path,default=ROOT/'.tools/gemini-contour-argon-ei-01')
    parser.add_argument('--request',action='store_true')
    args=parser.parse_args()
    try:
        record=prepare(args.source,args.output,args.request)
        print(json.dumps({'status':record['status'],'validation':record['validation'],'usage':record['provider'].get('usage')},indent=2))
    except PreparationError as exc:raise SystemExit(str(exc)) from None
