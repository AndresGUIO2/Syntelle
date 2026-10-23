"""Sequential, PostgreSQL-recorded corpus preparation within an authorized budget."""
import argparse
import base64
from hashlib import sha256
import json
import os
from pathlib import Path
import shutil

import cv2
from dotenv import dotenv_values
import numpy as np
from PIL import Image, ImageOps, ImageDraw

from app.workflows.contours.gemini_contour_trial import stamp
from app.workflows.oral.oral_semantics import analyze, load_inputs
from app.workflows.oral.prepare_semantic_oral import prepare, selected_masks, algorithm_hash, finalize
from app.preparation.archive import Archive, encoded
from app.preparation.dental_coverage import continue_attachment,neutralize_shared_lighting,extract_fragment
from app.preparation.faces import FaceDetector, PreparationError
from app.preparation.detection import PreparationDetector
from app.preparation.gemini import build_prompt, VISEMES
from app.preparation.mesh import INNER, OUTER
from app.preparation.mouth import align_variant, export_lossless_pair
from app.preparation.recorded_provider import RecordedProvider
from app.preparation.semantic_rig import row_profile, pose_quality
from app.workflows.visemes.viseme_trial import ROOT, read_generated, save_provider_image, write_report

VERSION='recorded-corpus-preparation/1'
NETWORK={'endpoint':'https://generativelanguage.googleapis.com/v1beta/interactions',
         'timeout_seconds':240,'retries':0,'follow_redirects':False}


def file_hash(path):return sha256(path.read_bytes()).hexdigest()


def generation_prompt(pose):
    prompt=build_prompt(ROOT/'docs/prompts/nano-banana-source.txt',pose)
    if pose=='FV':
        prompt=(ROOT/'docs/prompts/viseme-fv-contact-v2.txt').read_text().replace('{{ORIGINAL_FV_PROMPT}}',prompt)
    return prompt+'\nThe FIRST supplied image is the exact original portrait. Preserve its full aspect ratio, framing and character identity. No changes outside the mouth articulation.\n'


def prepare_source(item,corpus,directory,detector):
    original=(corpus.parent/item['original']).resolve()
    if file_hash(original)!=item['original_sha256']:raise PreparationError('Corpus source changed.')
    image=ImageOps.exif_transpose(Image.open(original)).convert('RGB')
    directory.mkdir(parents=True,exist_ok=True);base=directory/'base';base.mkdir(exist_ok=True)
    shutil_copy=base/'original.png'
    if shutil_copy.exists() and file_hash(shutil_copy)!=item['original_sha256']:
        raise PreparationError('Working original changed.')
    shutil_copy.write_bytes(original.read_bytes())
    try:
        face=detector.detect(image)
        write_report(base/'base-landmarks.json',face.as_dict())
        write_report(base/'preflight.json',{'source_sha256':file_hash(original),'eligible':True,'landmarks':478,'version':VERSION})
        return original,image,face
    except PreparationError as error:
        write_report(base/'preflight.json',{'source_sha256':file_hash(original),'eligible':False,'error':str(error),'version':VERSION})
        raise


def generate_pose(original,base,base_face,detector,directory,pose,provider,model,request,*,prompt=None,supporting=()):
    directory.mkdir(parents=True,exist_ok=True);ledger=directory/'run.json';prompt=prompt or generation_prompt(pose)
    ratio=base.width/base.height
    aspect=min(['1:1','3:4','4:3','9:16','16:9'],key=lambda s:abs(ratio/(int(s.split(':')[0])/int(s.split(':')[1]))-1))
    if abs(ratio/(int(aspect.split(':')[0])/int(aspect.split(':')[1]))-1)>.02:
        raise PreparationError('No supported generation aspect ratio preserves this original frame.')
    mime=Image.open(original).get_format_mimetype()
    payload={'model':model,'input':[{'type':'text','text':prompt},{'type':'image','mime_type':mime,
        'data':base64.b64encode(original.read_bytes()).decode()}],
        'response_format':{'type':'image','mime_type':'image/jpeg','aspect_ratio':aspect,'image_size':'1K'},'store':False}
    for data,image_mime in supporting:
        payload['input'].append({'type':'image','mime_type':image_mime,'data':base64.b64encode(data).decode()})
    binding=sha256(encoded(payload)).hexdigest()
    record=json.loads(ledger.read_text()) if ledger.exists() else None
    if record:
        if record['request_binding']!=binding:raise PreparationError('Cached generation inputs changed.')
        if record['generation']['status']!='received':raise PreparationError('A previous image request failed or may have incurred cost; no retry.')
        raw=(directory/record['generation']['file']).read_bytes()
        if sha256(raw).hexdigest()!=record['generation']['raw_sha256']:raise PreparationError('Cached original generation changed.')
        if record.get('metrics'):return
    else:
        if not request:raise PreparationError('Generation is not cached; explicit --request is required.')
        record={'schema':VERSION,'source_sha256':file_hash(original),'request_binding':binding,'viseme':pose,
            'canvas':{'width':base.width,'height':base.height},
            'generation':{'status':'request_started','attempts':1,'started_at':stamp(),'model':model,
                'request_settings':{'input_sha256':file_hash(original),'response_format':payload['response_format'],'retries':0}}}
        (directory/'prompt.txt').write_text(prompt);write_report(directory/'request.json',payload);write_report(ledger,record)
        key=os.environ.get('GEMINI_API_KEY') or dotenv_values(ROOT/'.env').get('GEMINI_API_KEY','')
        if not key:raise PreparationError('Configure the local Gemini key before starting.')
        try:
            raw,status,_=provider(payload,NETWORK,key,directory)
            if not 200<=status<300:raise PreparationError(f'Provider HTTP {status}; batch stopped without retry.')
            body=json.loads(raw)
            parts=[body['output_image']] if body.get('output_image') else [p for s in body.get('steps',[]) for p in s.get('content',[]) if p.get('type')=='image' and p.get('data')]
            if len(parts)!=1:raise PreparationError('Expected one generated image; raw response retained.')
            raw=base64.b64decode(parts[0]['data'],validate=True);path=save_provider_image(raw,directory,pose)
            record['generation'].update(status='received',file=path.name,raw_sha256=sha256(raw).hexdigest(),usage=body.get('usage',{}))
        except (ValueError,PreparationError) as error:
            record['generation'].update(status='failed',error=str(error).replace(key,'[redacted]')[:500]);raise PreparationError(record['generation']['error']) from None
        finally:
            record['generation']['finished_at']=stamp();write_report(ledger,record)
    try:
        generated=read_generated(raw,base.size);face=detector.detect(generated)
        _,_,metrics=align_variant(base,generated,base_face,face)
        write_report(directory/'variant-landmarks.json',face.as_dict());write_report(directory/'base-landmarks.json',base_face.as_dict())
        record.update(metrics=metrics,status='aligned')
    except PreparationError as error:
        record.update(status='alignment_failed',error=str(error));raise
    finally:write_report(ledger,record)


def input_manifest(original,base,directory,order):
    points=[np.asarray(json.loads((directory/'base/base-landmarks.json').read_text())['landmarks_px'])[:,:2]]
    for pose in order:
        folder=directory/'poses'/pose.lower();record=json.loads((folder/'run.json').read_text())
        if not record.get('metrics'):raise PreparationError(f'{pose}: alignment is incomplete.')
        raw=np.asarray(json.loads((folder/'variant-landmarks.json').read_text())['landmarks_px'])[:,:2]
        matrix=np.asarray(record['metrics']['transform']);points.append(raw@matrix[:,:2].T+matrix[:,2])
    lips=np.concatenate([p[OUTER+INNER] for p in points]);width=np.ptp(points[0][OUTER,0]);padding=width*.4
    left,top=np.maximum(0,np.floor(lips.min(0)-padding)).astype(int)
    right,bottom=np.minimum(base.size,np.ceil(lips.max(0)+padding)).astype(int)
    manifest={'schema':'syntelle-oral-input/1','original':os.path.relpath(original,directory),'original_sha256':file_hash(original),
        'crop':{'x':int(left),'y':int(top),'width':int(right-left),'height':int(bottom-top)},
        'poses':{'Rest':{'source':'base'},**{pose:{'source':f'poses/{pose.lower()}'} for pose in order}}}
    path=directory/'oral-input.json';write_report(path,manifest);return path


def select_material(manifest,semantics,output,reference_dentition=None):
    if reference_dentition is not None:
        selected=json.loads(reference_dentition.read_text(encoding='utf-8'))
        spec=json.loads(manifest.read_text(encoding='utf-8'))
        if selected.get('source_sha256')!=spec['original_sha256']:
            raise PreparationError('Expressive dentition belongs to a different immutable portrait.')
        if not selected.get('ready') or not selected.get('speech_validation',{}).get('ready'):
            raise PreparationError('Expressive dentition must pass observed coverage and articulation checks before corpus composition.')
        output.mkdir(exist_ok=True)
        material={'schema':'syntelle-oral-material/1','original_sha256':spec['original_sha256'],
                  'reference_pose':'Laugh','external_reference':True,
                  'reference_sha256':selected['reference_sha256'],
                  'reference_dentition_sha256':file_hash(reference_dentition)}
        for row in ('upper','lower'):
            source=(reference_dentition.parent/selected['rows'][row]['material']).resolve()
            observed=(reference_dentition.parent/selected['rows'][row]['observed']).resolve()
            material[row]=os.path.relpath(source,output.parent)
            material['protected_'+row]=os.path.relpath(observed,output.parent)
        path=output.parent/f'oral-{output.name}.json';write_report(path,material);return path
    spec,_,images,_,evidence=load_inputs(manifest);candidates=[]
    for pose in spec['poses']:
        masks,_=selected_masks(semantics/pose.lower(),pose,images[pose],evidence[pose])
        upper=np.asarray(masks['upper_teeth'])>=128;lower=np.asarray(masks['lower_teeth'])>=128
        tongue=np.asarray(masks['tongue'])>=128
        lower &= ~upper;upper &= ~tongue;lower &= ~tongue
        score=min(upper.any(0).sum(),lower.any(0).sum())
        if score>=8 and upper.sum()>=20 and lower.sum()>=12:candidates.append((int(score),pose,upper,lower))
    if not candidates:raise PreparationError('No single pose provides a usable upper/lower dental reference.')
    _,pose,upper,lower=max(candidates,key=lambda c:(c[0],c[1]=='EI'))
    output.mkdir(exist_ok=True);record={'method':'widest-common-observed-dental-rows/1','selected_pose':pose,
        'scores':{p:s for s,p,_,_ in candidates},'manual_masks':False,'rows':{}}
    masks,_=selected_masks(semantics/pose.lower(),pose,images[pose],evidence[pose])
    fragments={}
    for row,mask in [('upper',upper),('lower',lower)]:
        excluded=np.asarray(masks['tongue']).copy()
        if row=='lower':excluded=np.maximum(excluded,np.asarray(masks['upper_teeth']))
        source=extract_fragment(images[pose],masks[f'{row}_teeth'],excluded)
        export_lossless_pair(source,output,f'{row}-fragment');fragments[row]=source
        Image.fromarray(mask.astype(np.uint8)*255).save(output/f'{row}-protected.png')
    illuminated,record['illumination']=neutralize_shared_lighting(fragments['upper'],fragments['lower'])
    for row,source in zip(('upper','lower'),illuminated):
        image,details=continue_attachment(source,row);export_lossless_pair(image,output,f'{row}-teeth');record['rows'][row]=details
    write_report(output/'selection.json',record)
    material={'schema':'syntelle-oral-material/1','original_sha256':spec['original_sha256'],'reference_pose':pose,
        **{key:f'{output.name}/{key.replace("_","-")}.png' for key in ('protected_upper','protected_lower')}}
    for row in ('upper','lower'):material[row]=f'{output.name}/{row}-teeth.png'
    # Protected mask filenames use row-first naming in the asset directory.
    for row in ('upper','lower'):material[f'protected_{row}']=f'{output.name}/{row}-protected.png'
    path=output.parent/f'oral-{output.name}.json';write_report(path,material);return path


def correct_fv(directory,provider,model,request):
    manifest=directory/'oral-input.json'
    spec,base,crops,_,evidence=load_inputs(manifest)
    regions,_=selected_masks(directory/'semantics/fv','FV',crops['FV'],evidence['FV'])
    quality=pose_quality('FV',regions)
    if quality['ready']:raise PreparationError('FV already passes source articulation checks; no correction is needed.')
    target=directory/'corrections/fv';target.mkdir(parents=True,exist_ok=True)
    guide=Image.open(directory/'semantics/fv/proposal/overlay.png').convert('RGB').resize((crops['FV'].width*4,crops['FV'].height*4),Image.Resampling.NEAREST)
    edge=row_profile(regions['upper_teeth'],'bottom');columns=np.flatnonzero(np.isfinite(edge))
    if len(columns)<3:raise PreparationError('No reliable upper incisal contact guide.')
    ImageDraw.Draw(guide).line([(int(x*4),int((edge[x]+1)*4)) for x in columns],fill='#00ff50',width=3)
    guide.save(target/'contact-guide.png');crops['FV'].save(target/'rejected-mouth.png')
    source_dir=(manifest.parent/spec['poses']['FV']['source']).resolve()
    record=json.loads((source_dir/'run.json').read_text());rejected=source_dir/record['generation']['file']
    prompt=(ROOT/'docs/prompts/viseme-fv-correction-v3.txt').read_text().replace('{{ORIGINAL_FV_PROMPT}}',generation_prompt('FV')).replace('{{VALIDATION_ISSUES}}','\n'.join(quality['issues']))
    write_report(target/'guide.json',{'method':'automatic-observed-upper-incisal-contact/1','source':evidence['FV'],
        'contours_sha256':file_hash(directory/'semantics/fv/proposal/contours.json'),'manual_coordinates':False,
        'scale':4,'contact_offset_native_px':1,'issues':quality['issues']})
    with PreparationDetector(ROOT/'.tools/models/face_landmarker.task') as detector:
        original=(manifest.parent/spec['original']).resolve();face=detector.detect(base)
        generate_pose(original,base,face,detector,target,'FV',provider,model,request,prompt=prompt,
            supporting=((rejected.read_bytes(),'image/jpeg'),((target/'rejected-mouth.png').read_bytes(),'image/png'),((target/'contact-guide.png').read_bytes(),'image/png')))
    candidate={**spec,'poses':{**spec['poses'],'FV':{'source':'corrections/fv'}}}
    write_report(directory/'oral-input-corrected.json',candidate)


def run(args):
    config=json.loads(args.config.read_text());corpus_path=(args.config.parent/config['corpus']).resolve()
    corpus=json.loads(corpus_path.read_text());db=Archive();args.output.mkdir(parents=True,exist_ok=True)
    db.budget(config['budget_id'],file_hash(args.config),config['authorized_limits'])
    images=RecordedProvider(db,config['budget_id'],args.output,'image');analysis=RecordedProvider(db,config['budget_id'],args.output,'analysis')
    cases={item['id']:item for item in corpus['cases']}
    for name in args.cases:
        if name not in config['development']+config['held_out']:raise PreparationError('Portrait is outside this authorized batch.')
        if name in config['held_out']:
            freeze=args.output/'freeze.json'
            if not freeze.exists() or json.loads(freeze.read_text())['algorithm_sha256']!=algorithm_hash()[0]:
                raise PreparationError('Freeze the validated development pipeline before held-out preparation.')
        directory=args.output/name;experiment=f'{config["budget_id"]}/{name}'
        print(f'{name}: {args.phase}',flush=True)
        try:
            if args.phase=='generate':
                with PreparationDetector(ROOT/'.tools/models/face_landmarker.task') as detector:
                    original,base,face=prepare_source(cases[name],corpus_path,directory,detector)
                    db.archive_tree(experiment,directory,{'split':cases[name]['split']})
                    for pose in config['generation_order']:
                        print(f'{name}/{pose}: one attempt maximum, immutable original',flush=True)
                        generate_pose(original,base,face,detector,directory/'poses'/pose.lower(),pose,images,config['model'],args.request)
                        db.archive_tree(experiment,directory)
                    input_manifest(original,base,directory,config['generation_order'])
            elif args.phase=='analyze':
                analyze(directory/'oral-input.json',directory/'semantics',['Rest']+config['generation_order'],request=args.request,provider=analysis)
            elif args.phase=='compose':
                manifest=directory/'oral-input.json';material=select_material(manifest,directory/'semantics',directory/'material',args.reference_dentition)
                prepare(manifest,material,directory/'semantics',directory/'package')
            elif args.phase=='correct-fv':
                correct_fv(directory,images,config['model'],args.request)
            elif args.phase=='analyze-corrected':
                for pose in ['Rest']+config['generation_order']:
                    if pose!='FV':shutil.copytree(directory/'semantics'/pose.lower(),directory/'semantics-corrected'/pose.lower(),dirs_exist_ok=True)
                analyze(directory/'oral-input-corrected.json',directory/'semantics-corrected',['Rest']+config['generation_order'],request=args.request,provider=analysis)
            elif args.phase=='compose-corrected':
                manifest=directory/'oral-input-corrected.json';material=select_material(manifest,directory/'semantics-corrected',directory/'material-corrected',args.reference_dentition)
                prepare(manifest,material,directory/'semantics-corrected',directory/'package-corrected')
            elif args.phase=='finalize':
                finalize(directory/'package',directory/'browser-review')
            db.event(experiment,'phase-complete',{'phase':args.phase,'version':VERSION})
        except PreparationError as error:
            db.event(experiment,'phase-failed',{'phase':args.phase,'error':str(error)});raise
        finally:
            if directory.exists():db.archive_tree(experiment,directory)


if __name__=='__main__':
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--config',type=Path,required=True);parser.add_argument('--output',type=Path,required=True)
    parser.add_argument('--cases',nargs='+',required=True);parser.add_argument('--phase',choices=['generate','analyze','compose','correct-fv','analyze-corrected','compose-corrected','finalize'],required=True)
    parser.add_argument('--reference-dentition',type=Path,help='Cached source-bound expressive dental material; no generation is performed.')
    parser.add_argument('--request',action='store_true');args=parser.parse_args()
    args.config=args.config.resolve();args.output=args.output.resolve()
    try:run(args)
    except PreparationError as error:raise SystemExit(str(error)) from None
