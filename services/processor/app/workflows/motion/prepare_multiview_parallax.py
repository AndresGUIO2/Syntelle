"""Register cached source-bound views and export a separate parallax candidate."""
import argparse
from hashlib import sha256
import json
from pathlib import Path
import shutil
from PIL import Image
from app.core.paths import ROOT
from app.preparation.archive import encoded
from app.preparation.faces import FaceDetector, PreparationError
from app.preparation.conversational_parallax import rebind, write_json
from app.preparation.multiview_parallax import pose, register_reference, extend_geometry
from app.preparation.reference_policy import DEFAULT_POLICY, load_policy, direction_coverage, require_direction_coverage
from app.workflows.rendering.export_native import resource_paths, validate_bindings


def prepare(bundle, config_path, references, models, output):
    if output.exists():raise PreparationError('Preserve prior candidates; choose a fresh output directory.')
    config=json.loads(config_path.read_bytes());original=(ROOT/config['original']).resolve()
    policy,policy_sha=load_policy(ROOT/config['reference_policy'] if config.get('reference_policy') else DEFAULT_POLICY)
    if config.get('reference_policy_sha256',policy_sha)!=policy_sha:raise PreparationError('Reference policy binding changed.')
    identity=sha256(original.read_bytes()).hexdigest()
    folders,specification=resource_paths(bundle,None)
    names={'oral':'package.json','motion':'package.json','gaze':'gaze.json','expressions':'expressions.json'}
    packages={k:json.loads((folders[k]/v).read_bytes()) for k,v in names.items()};validate_bindings(packages,specification)
    if identity!=specification['source_sha256'] or identity!=config['source_sha256']:raise PreparationError('Reference source mismatch.')
    for name in ('face_landmarker.task','selfie-multiclass-256-float32-v1.tflite'):
        if not (models/name).is_file():raise PreparationError('Pinned local models are required; no model download is allowed.')
    output.mkdir(parents=True)
    write_json(output/'reference-policy.json',policy)
    for key in ('motion','gaze','expressions'):shutil.copytree(folders[key],output/key)
    image=Image.open(original).convert('RGB')
    with FaceDetector(models/'face_landmarker.task') as detector:source=detector.detect(image)
    write_json(output/'source-landmarks.json',source.as_dict())
    foreground=Image.open(folders['motion_assets']/packages['motion']['foreground']).convert('RGBA')
    regions=[packages['expressions']['crop']]
    if packages['expressions'].get('reference_laughter'):regions.append(packages['expressions']['reference_laughter']['crop'])
    views=[];supports=[];records=[]
    for slot in config['slots']:
        if slot not in ('yaw-left','yaw-right','pitch-up','pitch-down'):continue
        ledger=references/slot/'run.json'
        if not ledger.exists():continue
        request=json.loads(ledger.read_bytes())
        if request['status']!='received':raise PreparationError('Stopped reference request; inspect preserved evidence.')
        settings=(ledger.parent/'settings.json').read_bytes()
        accepted_settings={sha256(config_path.read_bytes()).hexdigest(),*config.get('accepted_reference_settings_sha256',[])}
        if request.get('original_sha256')!=identity or sha256(settings).hexdigest() not in accepted_settings or json.loads(settings)['source_sha256']!=identity:raise PreparationError('Reference request belongs to different source/settings.')
        path=ledger.parent/request['file']
        if sha256(path.read_bytes()).hexdigest()!=request['sha256']:raise PreparationError('Reference bytes changed.')
        support,record=register_reference(image,path,source.points,pose(source.matrix),packages['motion'],packages['oral'],models,output/'registration'/slot,references.parent/'reference-preparation'/request['sha256'],foreground,regions,slot,policy)
        records.append(record)
        if support is None:continue
        index=len(views);name=f'multiview-{index}.webp';uv=f'multiview-{index}-uv.npz'
        shutil.copyfile(output/'registration'/slot/'registered-support.webp',output/'motion'/name)
        shutil.copyfile(output/'registration'/slot/'source-uv.npz',output/'motion'/uv)
        views.append(dict(pose=record['achieved_degrees'],axis=record['axis'],texture=dict(file=name,sha256=record['material_sha256']),uv=dict(file=uv,sha256=record['uv_sha256']),reference_sha256=request['sha256'],camera=record['camera']))
        supports.append(support)
    write_json(output/'registration-review.json',records)
    coverage=direction_coverage(policy,records)
    write_json(output/'coverage-review.json',coverage)
    require_direction_coverage(coverage)
    if len(views)>4 or len(views)*image.width*image.height*4>128*1024*1024:raise PreparationError('Registered support exceeds the bounded native atlas; inspect without activation.')
    motion=packages['motion'];previous=motion.pop('build_id')
    baseline_bounds=dict(motion['projection']['bounds'])
    geometry=extend_geometry(motion,folders['motion'],foreground,supports,output/'motion',coverage['maximum_symmetric_yaw'])
    motion['projection']['multiview']=dict(version='registered-multiview/1',source_sha256=identity,parent_motion_build_id=previous,geometry_sha256=motion['projection']['geometry']['sha256'],views=views,protected_eye_oral=True,hidden_geometry_measured=False,pose_method='relative-mediapipe-estimate',owner_acceptance=False)
    motion['projection']['multiview'].update(coverage=coverage,baseline_bounds=baseline_bounds,reference_policy_sha256=policy_sha)
    from app.preparation import multiview_parallax
    from app.preparation import reference_policy
    motion['projection']['multiview']['preparation_sources']={p.name:sha256(p.read_bytes()).hexdigest() for p in (Path(__file__),Path(multiview_parallax.__file__),Path(reference_policy.__file__))}
    motion['build_id']=sha256(encoded(motion)).hexdigest();result=dict(specification)
    for key in ('motion','gaze','expressions'):
        if key!='motion':rebind(packages[key],previous,motion['build_id'])
        write_json(output/key/names[key],packages[key]);result[key]=(output/key).as_posix()
    write_json(output/'resource-bundle.json',result);write_json(output/'geometry-review.json',geometry)
    return dict(bundle=str(output/'resource-bundle.json'),accepted_references=len(views),rejected_references=len(records)-len(views),geometry=geometry)


if __name__=='__main__':
    p=argparse.ArgumentParser(description=__doc__)
    for name in ('bundle','config','references','models','output'):p.add_argument('--'+name,type=Path,required=True)
    a=p.parse_args();print(json.dumps(prepare(a.bundle.resolve(),a.config.resolve(),a.references.resolve(),a.models.resolve(),a.output.resolve())))
