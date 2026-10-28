"""One source-bound reference request; a separate finite PostgreSQL allowance."""
import argparse
import base64
from hashlib import sha256
import json
from pathlib import Path
import re

from dotenv import dotenv_values
from PIL import Image

from app.core.paths import ROOT
from app.preparation.archive import Archive, encoded
from app.preparation.faces import PreparationError
from app.preparation.recorded_provider import RecordedProvider
from app.workflows.motion.prepare_motion import NETWORK
from app.workflows.visemes.viseme_trial import save_provider_image
from app.preparation.reference_policy import DEFAULT_POLICY, load_policy, prompt_for


def preview_prompt(config_path,slot,output,policy_path=DEFAULT_POLICY):
    config=json.loads(config_path.read_bytes());policy,policy_sha=load_policy(policy_path)
    source=(ROOT/config['original']).resolve()
    if sha256(source.read_bytes()).hexdigest()!=config['source_sha256']:raise PreparationError('Immutable original checksum mismatch.')
    if output.exists():raise PreparationError('Preserve prior prompt drafts; choose a fresh output directory.')
    output.mkdir(parents=True)
    (output/'prompt.txt').write_text(prompt_for(policy,slot),encoding='utf-8')
    record=dict(version='reference-prompt-preview/1',slot=slot,source_sha256=config['source_sha256'],
        policy_sha256=policy_sha,expected_pose=policy['views'][slot],provider_calls=0,
        authorization=False,existing_budget_changed=False)
    (output/'preview.json').write_bytes(encoded(record))
    return record


def request_reference(config_path, slot, output, need, attempt=None):
    config=json.loads(config_path.read_bytes())
    limits=config.get('limits',{})
    if config.get('status')!='owner-authorized' or set(limits)!={'image','analysis'} or type(limits.get('image')) is not int or not 1<=limits['image']<=6 or limits.get('analysis')!=0 or config.get('transport_retries')!=0 or not config.get('stop_on_provider_error'):
        raise PreparationError('A separate explicitly authorized bounded image scope (at most six) with zero analyses and retries is required.')
    if slot not in config['slots']:
        raise PreparationError('Unknown reference slot.')
    source=(ROOT/config['original']).resolve();identity=sha256(source.read_bytes()).hexdigest()
    if identity!=config['source_sha256']:
        raise PreparationError('Immutable original checksum mismatch.')
    decision=json.loads(need.read_bytes())
    if decision.get('source_sha256')!=identity or slot not in decision.get('needed_slots',[]) or not decision.get('reason'):
        raise PreparationError('Reference need must bind this source and requested slot.')
    if attempt is not None and not re.fullmatch(r'[a-z0-9][a-z0-9-]{0,63}',attempt):raise PreparationError('Use a simple immutable attempt identifier.')
    stage=output/attempt/slot if attempt else output/slot
    if (stage/'run.json').exists():
        record=json.loads((stage/'run.json').read_bytes())
        if record['status']!='received' or (stage/'settings.json').read_bytes()!=config_path.read_bytes() or sha256((stage/record['file']).read_bytes()).hexdigest()!=record['sha256']:
            raise PreparationError('Stopped or mismatched reference; do not retry.')
        return {'cached':True,'file':str(stage/record['file'])}
    if any(json.loads(p.read_bytes())['status']!='received' for p in output.glob('*/run.json')):
        raise PreparationError('An earlier request stopped the batch; no retry allowed.')
    # A revised prompt cannot silently reuse a historical budget binding. The
    # owner-authorized configuration must pin its policy before a new call.
    if not config.get('reference_policy') or not config.get('reference_policy_sha256'):
        raise PreparationError('New acquisition requires an explicitly authorized configuration bound to the revised reference policy. Historical cached requests remain inspectable; no call was made.')
    policy,policy_sha=load_policy(ROOT/config['reference_policy'])
    if policy_sha!=config['reference_policy_sha256']:raise PreparationError('Reference prompt policy checksum changed; no call was made.')
    if policy['version']=='parallax-reference-policy/3' and not config.get('orientation_guide'):raise PreparationError('The guided reference policy requires its source-bound orientation guide; no call was made.')
    prompt=prompt_for(policy,slot)
    key=dotenv_values(ROOT/'.env').get('GEMINI_API_KEY','')
    if not key: raise PreparationError('Configure the existing local image provider key.')
    budget_binding=sha256(config_path.read_bytes()).hexdigest()
    if config.get('authorization_manifest'):
        authorization=(ROOT/config['authorization_manifest']).read_bytes()
        authority=json.loads(authorization)
        if sha256(authorization).hexdigest()!=config['authorization_sha256'] or authority.get('status')!='owner-authorized' or any(authority.get(k)!=config.get(k) for k in ('budget_id','limits','source_sha256','model')) or not authority.get('allow_prompt_refinement'):
            raise PreparationError('Iterative reference authorization does not match this source/model/allowance.')
        budget_binding=sha256(authorization).hexdigest()
    db=Archive();db.budget(config['budget_id'],budget_binding,config['limits'])
    stage.mkdir(parents=True,exist_ok=True)
    (stage/'settings.json').write_bytes(config_path.read_bytes());(stage/'need.json').write_bytes(need.read_bytes())
    if config.get('authorization_manifest'):(stage/'authorization.json').write_bytes(authorization)
    (stage/('original'+source.suffix)).write_bytes(source.read_bytes())
    (stage/'prompt.txt').write_text(prompt,encoding='utf-8')
    (stage/'reference-policy.json').write_bytes(encoded(policy))
    payload=dict(model=config['model'],input=[dict(type='text',text=prompt),dict(type='image',mime_type=Image.open(source).get_format_mimetype(),data=base64.b64encode(source.read_bytes()).decode())],response_format=config['response_format'],store=False)
    if config.get('orientation_guide'):
        guide=(ROOT/config['orientation_guide']).resolve();data=guide.read_bytes()
        if sha256(data).hexdigest()!=config['orientation_guide_sha256']:raise PreparationError('Orientation guide checksum mismatch.')
        guide_record=json.loads((guide.with_suffix('.json')).read_bytes())
        if guide_record['source_sha256']!=identity or guide_record['slot']!=slot:raise PreparationError('Orientation guide belongs to another source or direction.')
        (stage/'orientation-guide.png').write_bytes(data)
        (stage/'orientation-guide.json').write_bytes(encoded(guide_record))
        payload['input'].append(dict(type='image',mime_type='image/png',data=base64.b64encode(data).decode()))
    (stage/'request.json').write_bytes(encoded(payload))
    record=dict(status='started',slot=slot,original_sha256=identity)
    try:
        (stage/'run.json').write_bytes(encoded(record))
        raw,status,seconds=RecordedProvider(db,config['budget_id'],output,'image')(payload,NETWORK,key,stage)
        if not 200<=status<300: raise PreparationError(f'Provider HTTP {status}; stopped without retry.')
        body=json.loads(raw)
        parts=[body['output_image']] if body.get('output_image') else [p for s in body.get('steps',[]) for p in s.get('content',[]) if p.get('type')=='image' and p.get('data')]
        if len(parts)!=1: raise PreparationError('Expected exactly one image; preserve the provider result.')
        file=save_provider_image(base64.b64decode(parts[0]['data'],validate=True),stage,'reference')
        record.update(status='received',file=file.name,sha256=sha256(file.read_bytes()).hexdigest(),seconds=seconds,usage=body.get('usage',{}))
    except Exception as error:
        record.update(status='failed',error=str(error).replace(key,'[redacted]')[:500]);raise
    finally:
        (stage/'run.json').write_bytes(encoded(record));db.archive_tree(config['budget_id']+'/'+slot,stage)
    return dict(cached=False,file=str(file),sha256=record['sha256'])


if __name__=='__main__':
    p=argparse.ArgumentParser(description=__doc__)
    for name in ('config','output'):p.add_argument('--'+name,type=Path,required=True)
    p.add_argument('--need',type=Path)
    p.add_argument('--preview-prompt',action='store_true')
    p.add_argument('--policy',type=Path,default=DEFAULT_POLICY)
    p.add_argument('--slot',required=True)
    p.add_argument('--attempt')
    a=p.parse_args()
    if a.preview_prompt:result=preview_prompt(a.config.resolve(),a.slot,a.output.resolve(),a.policy.resolve())
    else:
        if a.need is None:p.error('--need is required for an authorized acquisition')
        result=request_reference(a.config.resolve(),a.slot,a.output.resolve(),a.need.resolve(),a.attempt)
    print(json.dumps(result),flush=True)
