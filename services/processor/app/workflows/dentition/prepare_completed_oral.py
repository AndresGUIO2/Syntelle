"""Stage the completed EI material with pose-specific soft tissue. No network calls."""
import json
import argparse
import shutil
from importlib.metadata import version
from hashlib import sha256
from pathlib import Path
import cv2
import numpy as np
from PIL import Image
from app.workflows.visemes.viseme_trial import ROOT, write_report
from app.preparation.mesh import INNER, OUTER, make_mesh
from app.preparation.mouth import export_lossless_pair
from app.preparation.oral import cavity_mask, polygon_mask, remove_teeth
from app.preparation.dental_selection import load_originals
from app.preparation.faces import PreparationError
from app.preparation.dental_seams import reconcile_completion

VERSION = 'completed-oral-rig/2'
EXPERIMENT = ROOT/'.tools/dentition-completion-04'
OUTPUT = ROOT/'.tools/oral-artifacts-05/package'


def warp_texture(image, mesh):
    """Bake recorded lip retargeting once; runtime endpoints use identity UVs."""
    raw=np.asarray(image); out=raw.copy().astype(np.float32)
    source=np.asarray(mesh['rest'],np.float32); target=np.asarray(mesh['open'],np.float32)
    for tri in mesh['triangles']:
        a,b=source[tri],target[tri]
        matrix=cv2.getAffineTransform(a,b)
        layer=cv2.warpAffine(raw,matrix,image.size,flags=cv2.INTER_LINEAR,borderMode=cv2.BORDER_REFLECT_101)
        mask=np.zeros(raw.shape[:2],np.uint8);cv2.fillConvexPoly(mask,np.rint(b*16).astype(np.int32),255,shift=4)
        out[mask>0]=layer[mask>0]
    return Image.fromarray(np.clip(out,0,255).astype(np.uint8))


def retarget(points, pose, config):
    """Keep each lip's thickness while fitting its inner edge to the fixed arch."""
    target=points.copy()
    settings=config['retarget'].get(pose,{})
    for name,center,ids in [('upper',13,range(1,10)),('lower',14,range(11,20))]:
        if f'{name}_inner_center_y' not in settings:continue
        delta=settings[f'{name}_inner_center_y']-float(points[center,1])
        for j in ids:
            weight=np.sin(np.pi*(j%10)/10)**settings['falloff_power']
            target[INNER[j],1]+=delta*weight
            target[OUTER[j],1]+=delta*weight*settings['outer_motion_fraction']
    return target


def soft_tissue(original, removal, tongue, opening):
    """Extend photographed cavity samples, never a constant black plate or lip RGB."""
    raw=np.asarray(original); cavity=np.asarray(opening)>0
    r,g,b=raw.astype(float).transpose(2,0,1)
    near=cv2.dilate(cavity.astype(np.uint8),np.ones((7,7),np.uint8))>0
    enamel=near&(g/(r+1)>.73)&(b/(r+1)>.64)&(g>45)
    mask=cv2.dilate(((np.asarray(removal)>0)|enamel).astype(np.uint8),np.ones((3,3),np.uint8))>0
    protected=np.asarray(tongue)>0;mask[protected]=False
    gray=cv2.cvtColor(raw,cv2.COLOR_RGB2GRAY)
    seeds=cavity&~mask&(gray<70)
    if seeds.sum()<8:raise PreparationError('Insufficient photographed cavity material; do not invent a flat fill.')
    _,nearest=cv2.distanceTransformWithLabels((~seeds).astype(np.uint8),cv2.DIST_L2,5,labelType=cv2.DIST_LABEL_PIXEL)
    lookup=np.vstack((np.zeros((1,3),np.uint8),raw[seeds]))
    extended=lookup[np.minimum(nearest,len(lookup)-1)]
    extended=cv2.GaussianBlur(extended,(0,0),1.2)
    # Pixels beyond the aperture belong to lips, so reconstruct them from lip
    # neighbours. Cavity extension is restricted to the photographed opening.
    fixed=raw.copy();fixed[mask]=extended[mask];fixed[protected]=raw[protected]
    # Rectangular dental annotations are cavity cleanup regions, not lip masks.
    # Only the aperture and actual neutral-enamel proposals may alter lip pixels.
    residual_cavity=near&(gray<75)
    lip_mask=cv2.dilate((enamel|cavity|residual_cavity).astype(np.uint8),np.ones((3,3),np.uint8))*255
    lips=cv2.inpaint(raw,lip_mask,5,cv2.INPAINT_TELEA)
    return Image.fromarray(lips),Image.fromarray(fixed),Image.fromarray(mask.astype(np.uint8)*255)


def prepare(output=OUTPUT):
    old=EXPERIMENT/'previous-runtime'; manifest=json.loads((old/'package.json').read_text())
    review=json.loads((ROOT/'docs/characters/syntelle/viseme-spacing-review.json').read_text())
    config_path=ROOT/'docs/characters/syntelle/completed-oral-rig.json'
    config=json.loads(config_path.read_text())
    crop=manifest['crop']; w,h=crop['width'],crop['height']; bounds=(crop['x'],crop['y'],crop['x']+w,crop['y']+h)
    base=Image.open(old/'base.webp').convert('RGB')
    images,points,provenance=load_originals(ROOT/'.tools/viseme-a-05',ROOT/'.tools/visemes-01',base.size,bounds,manifest['poses'])
    images['Rest']=base.crop(bounds);points['Rest']=np.asarray(manifest['poses']['Rest']['points'])
    output.mkdir(parents=True,exist_ok=True); export_lossless_pair(base,output,'base')
    for row in ('upper','lower'):
        material,evidence=reconcile_completion(Image.open(EXPERIMENT/f'correction/replay/{row}.png'),
            Image.open(ROOT/f'.tools/dental-spacing-03/package/{row}-teeth.png'))
        export_lossless_pair(material,output,f'{row}-teeth');write_report(output/f'{row}-continuity.json',evidence)
    for name in ('dental-source','dental-mask'):
        export_lossless_pair(Image.open(old/f'{name}.webp'),output,name)
    result={key:manifest[key] for key in ('canvas','crop','detail','source_sha256','reference')}
    result.update(schema='syntelle-viseme-package/2',pipeline_version=VERSION,renderer_version='completed-oral/2',poses={},pairs={})
    result['reference']['completion']={'model':'gemini-nano-banana-2.1','run_sha256':sha256((EXPERIMENT/'correction/run.json').read_bytes()).hexdigest(),
        'registration':json.loads((EXPERIMENT/'correction/replay/replay.json').read_text()),'material':'restored EI fragments + registered hidden completion'}
    result['oral']=config['lighting']
    result['oral_config']=config
    offset=np.asarray(review['coordinates']['annotation_origin'])-bounds[:2]
    jaw=config['lower_translation_y'].copy()
    for pose,data in manifest['poses'].items():
        folder=output/pose.lower();folder.mkdir(exist_ok=True)
        settings=review['poses'][pose]; original=images[pose]
        removal=polygon_mask((w,h),settings['remove'],offset)
        tongue=polygon_mask((w,h),settings['tongue'],offset)
        if pose=='EI':
            masks=ROOT/'.tools/dental-spacing-03/package/argon-masks'
            tongue=Image.open(masks/'tongue_visible.png').convert('L')
            removal=Image.fromarray(np.maximum(np.asarray(Image.open(old/'upper-teeth.webp'))[...,3],np.asarray(Image.open(old/'lower-teeth.webp'))[...,3]))
        opening=cavity_mask((w,h),points[pose])
        if pose in ('Rest','MBP'):surface,interior,removed=original.copy(),original.copy(),Image.new('L',(w,h))
        else:surface,interior,removed=soft_tissue(original,removal,tongue,opening)
        target=retarget(points[pose],pose,config)
        try: retarget_mesh=make_mesh(points[pose],target,(0,0,w,h))
        except PreparationError as error: raise PreparationError(f'{pose} retarget: {error}') from None
        if pose not in ('Rest','MBP'):
            surface=warp_texture(surface,retarget_mesh);interior=warp_texture(interior,retarget_mesh)
        # Use the exact regularized endpoint geometry used by this retarget.
        fitted=target.copy();fitted[OUTER+INNER,:2]=np.asarray(retarget_mesh['open'])[:40]
        export_lossless_pair(surface,folder,'surface')
        export_lossless_pair(interior,folder,'cavity')
        Image.open(old/data['mask']).save(folder/'alpha.png')
        removed.save(folder/'teeth-removal.png');tongue.save(folder/'tongue-protection.png')
        tissue=original.convert('RGBA');tissue.putalpha(tongue);tissue.save(folder/'tongue-source.png')
        aperture=cavity_mask((w,h),fitted)
        aperture.save(folder/'aperture.png')
        alpha=np.asarray(Image.open(folder/'alpha.png'))
        Image.fromarray(np.minimum(alpha,255-np.asarray(aperture))).save(folder/'lip-occluder.png')
        original.save(folder/'original.png');write_report(folder/'retarget.json',retarget_mesh)
        points[pose]=fitted
        if pose in ('O','U'):
            lower=np.asarray(Image.open(output/'lower-teeth.png'))[...,3]>=128
            aperture=np.asarray(cavity_mask((w,h),fitted))>0
            columns=np.flatnonzero(lower.any(0)&aperture.any(0))
            jaw[pose]=int(np.ceil(max(np.flatnonzero(aperture[:,x]).max()-np.flatnonzero(lower[:,x]).min() for x in columns)+2))
        result['poses'][pose]={**data,'status':'experimental','notes':'Completed EI material; joint lip fitting; geometry checked.',
            'lower_translation':[0,jaw[pose]],'dental_inset_px':0,'points':fitted.tolist(),'inner':fitted[INNER].tolist(),
            'retarget_max_px':float(np.linalg.norm(fitted-np.asarray(data['points']),axis=1).max()),
            'cavity':f'{pose.lower()}/cavity.webp',
            'layers':{'cavity':f'{pose.lower()}/cavity.webp','tongue':f'{pose.lower()}/tongue-source.png',
                      'aperture':f'{pose.lower()}/aperture.png','lip_occluder':f'{pose.lower()}/lip-occluder.png'},
            'tongue_source_pixels_preserved':True,
            'endpoint':f'{pose.lower()}/endpoint.webp','surface':f'{pose.lower()}/surface.webp','mask':f'{pose.lower()}/alpha.png'}
        # Temporary endpoint placeholder; browser export replaces it using the unified compositor.
        placeholder=surface.convert('RGBA');placeholder.putalpha(Image.open(folder/'alpha.png'));export_lossless_pair(placeholder,folder,'endpoint')
    for a in points:
        for b in points:
            try:
                mesh=make_mesh(points[a],points[b],(0,0,w,h))
                result['pairs'][f'{a}-{b}']={'safe':True,'reason':None,'mesh':mesh}
            except PreparationError as error:
                result['pairs'][f'{a}-{b}']={'safe':False,'reason':str(error),'mesh':None}
    result['inputs']=provenance
    result['dependencies']={name:version(name) for name in ('Pillow','numpy')};result['dependencies']['opencv']=cv2.__version__
    sources=['services/processor/app/workflows/dentition/prepare_completed_oral.py','services/processor/app/workflows/dentition/complete_dentition.py',
             'services/processor/app/preparation/mesh.py','apps/web/src/experiments/viseme-a/completedMorph.ts',
             'services/processor/app/preparation/dental_seams.py',
             'apps/web/src/experiments/viseme-a/VisemeTrial.vue','apps/web/src/experiments/viseme-a/VisemeStage.vue',
             'apps/web/src/experiments/viseme-a/usePosePackage.ts','docs/characters/syntelle/completed-oral-rig.json',
             'apps/web/src/experiments/viseme-a/poseTypes.ts','apps/web/scripts/export-oral-review.mjs',
             'services/processor/app/workflows/oral/oral_review_sheet.py','services/processor/tests/test_completed_dentition.py',
             'apps/web/tests/e2e/completed-oral.spec.ts','apps/web/tests/e2e/viseme-trial.spec.ts',
             'apps/web/playwright.config.ts','docs/prompts/dentition-completion-v2.txt']
    result['pipeline_source_hashes']={name:sha256((ROOT/name).read_bytes()).hexdigest() for name in sources}
    result['export_settings']={'PNG':'lossless','WebP':{'lossless':True,'exact':True,'method':6},'endpoints':'browser shared compositor; not a separate Pillow composition'}
    result['build_id']=sha256(json.dumps(result,sort_keys=True).encode()).hexdigest()
    write_report(output/'package.json',result)
    snapshot=output.parent/'pipeline-snapshots'/result['build_id']
    for name in sources:
        destination=snapshot/name;destination.parent.mkdir(parents=True,exist_ok=True);shutil.copyfile(ROOT/name,destination)
    write_report(snapshot/'package.json',result)
    print('Staged',len(result['poses']),'poses;',sum(v['safe'] for v in result['pairs'].values()),'of',len(result['pairs']),'non-inverting pairs.')
    return result


def install_candidate(output=OUTPUT):
    """Explicit, development-only candidate; the default/previous trial stays intact."""
    package=json.loads((output/'package.json').read_text())
    names={'package.json','base.webp','upper-teeth.webp','lower-teeth.webp','dental-source.webp','dental-mask.webp'}
    for pose in package['poses'].values():names.update(pose[k] for k in ('endpoint','surface','mask','cavity'))
    target=ROOT/'apps/web/src/experiments/viseme-a/completed'
    for name in names:
        path=Path(name)
        if path.is_absolute() or '..' in path.parts:raise PreparationError('Invalid browser asset name.')
        destination=target/path;destination.parent.mkdir(parents=True,exist_ok=True)
        shutil.copyfile(output/path,destination)


def finalize_browser_exports(output=OUTPUT, review=None):
    """Promote only exports verified against this exact prepared build."""
    review=review or output.parent/'browser-review'
    package=json.loads((output/'package.json').read_text());report=json.loads((review/'report.json').read_text())
    if (report['build_id']!=package['build_id'] or len(report['checks'])!=648 or not all(v['nonempty'] for v in report['checks'])
            or report['maximumEndpointDelta']!=0 or report['maximumOutsideAlpha']!=0):
        raise PreparationError('Browser exports failed endpoint/mask checks or belong to another build.')
    for pose in package['poses']:
        export_lossless_pair(Image.open(review/f'endpoint-{pose}.png'),output/pose.lower(),'endpoint')
    package['browser_exports']={'renderer':package['renderer_version'],'frames_checked':648,'maximum_endpoint_delta':0,'maximum_outside_alpha':0,
                                'report_sha256':sha256((review/'report.json').read_bytes()).hexdigest()}
    package['artifact_hashes']={str(path.relative_to(output)).replace('\\','/'):sha256(path.read_bytes()).hexdigest()
                                for path in output.rglob('*') if path.is_file() and path.name!='package.json'}
    package['prepared_build_id']=package['build_id']
    package['build_id']=sha256(json.dumps(package,sort_keys=True).encode()).hexdigest()
    write_report(output/'package.json',package)
    write_report(output.parent/'installed-package.json',package);install_candidate(output)


if __name__=='__main__':
    parser=argparse.ArgumentParser(description=__doc__);parser.add_argument('--install-candidate',action='store_true')
    parser.add_argument('--finalize-browser-exports',action='store_true')
    parser.add_argument('--output',type=Path,default=OUTPUT)
    args=parser.parse_args()
    if args.finalize_browser_exports:finalize_browser_exports(args.output)
    else:
        prepare(args.output)
        if args.install_candidate:install_candidate(args.output)
