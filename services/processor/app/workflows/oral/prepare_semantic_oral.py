"""Compose source-bound semantic mouths with one reusable oral material package."""
import argparse
from hashlib import sha256
import json
from pathlib import Path
import shutil
from importlib.metadata import version

import cv2
import numpy as np
from PIL import Image, ImageDraw

from app.workflows.oral.oral_semantics import load_inputs, digest, VERSION
from app.preparation.faces import PreparationError
from app.preparation.mesh import INNER, OUTER
from app.preparation.mouth import export_lossless_pair
from app.preparation.semantic_mouth import LABELS, rasterize, OralContours, overlay, validate_masks
from app.preparation.semantic_rig import aperture_sdf, fit_lower, fit_upper, measured_points, mesh_for, reconstruct_cavity, enforce_bite, pose_quality, register_oral_patch
from app.preparation.labiodental import fit_contact
from app.workflows.visemes.viseme_trial import write_report
from app.workflows.visemes.viseme_trial import ROOT

ALGORITHM_SOURCES = ('services/processor/app/workflows/oral/oral_semantics.py','services/processor/app/workflows/oral/prepare_semantic_oral.py',
    'services/processor/app/preparation/semantic_mouth.py','services/processor/app/preparation/semantic_rig.py',
    'services/processor/app/preparation/dental_coverage.py','services/processor/app/workflows/dentition/oral_repair.py',
    'services/processor/app/preparation/labiodental.py',
    'services/processor/app/preparation/reference_dentition.py',
    'services/processor/app/workflows/oral/prepare_reference_dentition.py',
    'services/processor/app/workflows/corpus/corpus_prepare.py','services/processor/app/preparation/detection.py',
    'services/processor/app/preparation/faces.py','services/processor/app/preparation/gemini.py',
    'services/processor/app/preparation/recorded_provider.py',
    'services/processor/app/preparation/mesh.py','services/processor/app/preparation/mouth.py',
    'apps/web/src/experiments/viseme-a/completedMorph.ts','apps/web/scripts/export-oral-review.mjs',
    'services/processor/uv.lock','apps/web/package-lock.json',
    'docs/prompts/oral-semantics-v1.txt','docs/prompts/nano-banana-source.txt',
    'docs/prompts/viseme-fv-contact-v2.txt','docs/prompts/viseme-fv-correction-v3.txt')


def algorithm_hash():
    hashes={name:digest(ROOT/name) for name in ALGORITHM_SOURCES}
    return sha256(json.dumps(hashes,sort_keys=True).encode()).hexdigest(),hashes


def selected_masks(folder, pose, image, evidence):
    selection = json.loads((folder/'selected.json').read_text())
    stage = folder/selection['stage']
    if selection['source'] != evidence or digest(stage/'contours.json') != selection['contours_sha256']:
        raise PreparationError('Semantic contours do not match original source evidence.')
    contours = OralContours.model_validate_json((stage/'contours.json').read_text())
    masks, _ = rasterize(contours, image)
    quality = validate_masks(masks, pose)
    if not quality['valid']: raise PreparationError(f'{pose}: {quality["issues"]}')
    return masks, {'stage': selection['stage'], 'contours_sha256': selection['contours_sha256'], 'quality': quality}


def preview(image, cavity, masks, upper, lower, upper_y, lower_y, closed):
    """Inspection only. Browser exports are the authoritative runtime endpoints."""
    if closed: return image.copy()
    layers = cavity.convert('RGBA')
    for row, dy in ((lower, lower_y), (upper, upper_y)):
        translated = row.transform(row.size, Image.Transform.AFFINE, (1,0,0,0,1,-dy), Image.Resampling.BILINEAR)
        layers = Image.alpha_composite(layers, translated)
    return Image.composite(layers.convert('RGB'), image, masks['aperture'])


def prepare(manifest, material_manifest, semantics, output, poses=None):
    spec, base, images, points, provenance = load_inputs(manifest)
    material = json.loads(material_manifest.read_text())
    if material.get('original_sha256') != spec['original_sha256']:
        raise PreparationError('The registered dentition belongs to another original portrait.')
    resolve = lambda name: (material_manifest.parent/material[name]).resolve()
    upper = Image.open(resolve('upper')).convert('RGBA'); lower = Image.open(resolve('lower')).convert('RGBA')
    protected_upper = Image.open(resolve('protected_upper')).convert('L')
    protected_lower = Image.open(resolve('protected_lower')).convert('L')
    crop = spec['crop']; size = (crop['width'], crop['height'])
    if any(image.size != size for image in (upper,lower,protected_upper,protected_lower)):
        raise PreparationError('Oral assets do not match their registered coordinate frame.')
    pose_ids = poses or list(spec['poses']); masks = {}; sources = {}
    for pose in pose_ids:
        folder = semantics/pose.lower()
        if (folder/'selected.json').exists():
            masks[pose], sources[pose] = selected_masks(folder, pose, images[pose], provenance[pose])
        elif pose == 'Rest' and poses:
            # Initial CHSHR-only inspection, not a completed nine-pose package.
            outer = Image.new('L',size); ImageDraw.Draw(outer).polygon([tuple(p) for p in points[pose][OUTER]],fill=255)
            masks[pose] = {k: outer if k=='outer_lips' else Image.new('L',size) for k in LABELS}
            sources[pose] = {'provisional_closed_baseline':True}
        else: raise PreparationError(f'{pose}: automatic semantic preparation is missing.')
    observations = [masks[p]['upper_teeth'] for p in pose_ids if p not in ('Rest','MBP','FV')]
    if material['reference_pose'] in pose_ids or material.get('external_reference'):
        upper_y, registration = fit_upper(protected_upper, observations)
    else:
        upper_y, registration = 0.0, {'method':'existing registered source material; preliminary CHSHR inspection','translation':[0,0]}
    if (output/'package.json').exists():
        previous=json.loads((output/'package.json').read_text())
        archive=output.parent/'packages'/previous['build_id']
        if not archive.exists():
            archive.parent.mkdir(parents=True,exist_ok=True);shutil.copytree(output,archive)
    output.mkdir(parents=True, exist_ok=True)
    export_lossless_pair(base,output,'base'); export_lossless_pair(upper,output,'upper-teeth'); export_lossless_pair(lower,output,'lower-teeth')
    w,h = size; x,y = crop['x'],crop['y']
    result = {'schema':'syntelle-viseme-package/3','pipeline_version':VERSION,'renderer_version':'semantic-oral/1',
              'source_sha256':spec['original_sha256'],'canvas':{'width':base.width,'height':base.height},'crop':crop,
              'detail':{'x':max(0,x-round(w*.35)),'y':max(0,y-round(h*.6)),
                        'width':min(base.width-x+round(w*.35),round(w*1.7)), 'height':min(base.height-y+round(h*.6),round(h*2.2))},
              'reference':{'upper':'upper-teeth.webp','lower':'lower-teeth.webp','registration':registration},
              'oral':{'contact_shadow_strength':.3,'contact_shadow_width_px':max(1,w*.009),'depth_strength':.6,'closed_gap_px':.8},
              'poses':{},'pairs':{},'inputs':provenance,'semantic_evidence':sources,
              'material_hashes':{name:digest(resolve(name)) for name in ('upper','lower','protected_upper','protected_lower')},
              'manifest_sha256':digest(manifest),'material_manifest_sha256':digest(material_manifest),
              'manual_masks':False,'lip_retargeting':False,'upper_translation':[0,upper_y]}
    result['endpoints_finalized']=False
    result['algorithm_sha256'],result['algorithm_sources']=algorithm_hash()
    result['dependencies']={name:version(name) for name in ('Pillow','numpy','pydantic','httpx','mediapipe')}
    result['dependencies']['opencv']=cv2.__version__
    result['export_settings']={'PNG':'lossless','WebP':{'lossless':True,'exact':True,'method':6},
                              'mask_channels':{'R':'edit alpha','G':'signed aperture distance, 128+8*distance_px','B':'reserved'},
                              'dental_alpha':'opaque material coverage, independent of color and shadows'}
    for pose in pose_ids:
        image = images[pose]; regions = masks[pose]; folder = output/pose.lower(); folder.mkdir(exist_ok=True)
        closed = pose in ('Rest','MBP')
        export_lossless_pair(image,folder,'face-aligned-source')
        image,regions,local_points,local_registration=register_oral_patch(image,regions,points[pose],protected_upper,upper_y,closed)
        source_quality=pose_quality(pose,regions)
        export_lossless_pair(image,folder,'registered-source')
        export_lossless_pair(overlay(image,regions),folder,'source-semantic-overlay')
        cavity, removal = reconstruct_cavity(image, regions) if not closed else (image.copy(),Image.new('L',size))
        contact=None
        if pose=='FV':
            image,cavity,regions,contact=fit_contact(image,cavity,regions,upper,upper_y)
            if contact['applied']:result['lip_retargeting']=True
        # Only fill the true opening in the hidden lip surface. Original lip RGB
        # remains exact; no tooth-removal polygon can repaint a visible lip.
        lipmask = (np.asarray(regions['aperture'])>=128).astype(np.uint8)*255
        surface = Image.fromarray(cv2.inpaint(np.asarray(image),lipmask,3,cv2.INPAINT_TELEA)) if lipmask.any() else image.copy()
        lower_y = fit_lower(protected_lower,lower.getchannel('A'),regions['aperture'],regions['lower_teeth'],closed)
        if not closed and np.asarray(regions['lower_teeth']).any():
            lower_y=enforce_bite(protected_upper,protected_lower,upper_y,lower_y,regions['aperture'])
        measured = measured_points(local_points,regions)
        # A bounded mask is derived from measured outer lips, never a manual
        # crop or pose-specific annotation. Base and target share the same mask.
        union = np.maximum(np.asarray(regions['outer_lips']),np.asarray(masks['Rest']['outer_lips']))
        radius = max(2,round(np.ptp(measured[OUTER,0])*.1))
        alpha = cv2.dilate(union,cv2.getStructuringElement(cv2.MORPH_ELLIPSE,(radius*2+1,radius*2+1)))
        alpha = cv2.GaussianBlur(alpha,(0,0),max(1,radius*.2)); alpha[[0,-1],:]=0;alpha[:,[0,-1]]=0
        export_lossless_pair(image,folder,'original'); removal.save(folder/'teeth-removal.png')
        for name, mask in regions.items(): mask.save(folder/f'{name}.png')
        export_lossless_pair(overlay(image,regions),folder,'semantic-overlay')
        export_lossless_pair(surface,folder,'surface'); export_lossless_pair(cavity,folder,'cavity')
        sdf = aperture_sdf(regions['aperture'], measured if closed else None)
        packed = np.stack((alpha,np.asarray(sdf),np.zeros_like(alpha)),axis=-1)
        Image.fromarray(packed).save(folder/'alpha.png'); sdf.save(folder/'aperture-sdf.png')
        still = preview(image,cavity,regions,upper,lower,upper_y,lower_y,closed)
        still.save(folder/'inspection.png')
        placed={**regions}
        visible=np.asarray(regions['aperture'])>=128
        for row,material_image,shift in [('upper',upper,upper_y),('lower',lower,lower_y)]:
            coverage=cv2.warpAffine(np.asarray(material_image.getchannel('A')),np.array([[1,0,0],[0,1,shift]],float),size,flags=cv2.INTER_NEAREST)
            coverage=((coverage>=128)&visible).astype(np.uint8)*255 if not closed else np.zeros_like(coverage)
            placed[f'{row}_teeth']=Image.fromarray(coverage);placed[f'{row}_teeth'].save(folder/f'placed-{row}.png')
        points[pose] = measured
        result['poses'][pose] = {'id':pose,'status':'experimental','notes':'Automatic semantic boundaries and constrained shared dentition; no manual masks.',
                                 'oral_registration':local_registration,
                                 'labiodental_contact':contact,'source_quality':source_quality,
                                 'preparation':pose_quality(pose,placed),
                                 'upper_visible':bool(np.asarray(regions['upper_teeth']).any()),'lower_visible':bool(np.asarray(regions['lower_teeth']).any()),
                                 'lower_translation':[0,lower_y],'upper_translation':[0,upper_y], 'points':measured.tolist(),
                                 'endpoint':f'{pose.lower()}/endpoint.png','surface':f'{pose.lower()}/surface.webp','cavity':f'{pose.lower()}/cavity.webp',
                                 'aperture_sdf':f'{pose.lower()}/aperture-sdf.png','mask':f'{pose.lower()}/alpha.png'}
        result['poses'][pose]['comparison']={'original':f'{pose.lower()}/registered-source.webp',
                                            'semantic':f'{pose.lower()}/semantic-overlay.webp'}
        if material.get('previous_package'):
            previous=(material_manifest.parent/material['previous_package']).resolve()
            prior=json.loads((previous/'package.json').read_text())
            if prior['crop']!=crop or prior['source_sha256']!=spec['original_sha256']:
                raise PreparationError('Before/after comparison belongs to another source frame.')
            base_crop=base.crop((x,y,x+w,y+h)).convert('RGBA')
            endpoint=Image.open(previous/prior['poses'][pose]['endpoint']).convert('RGBA')
            export_lossless_pair(Image.alpha_composite(base_crop,endpoint).convert('RGB'),folder,'previous')
            result['poses'][pose]['comparison']['previous']=f'{pose.lower()}/previous.webp'
    for a in pose_ids:
        for b in pose_ids:
            try:
                mesh = mesh_for(points[a],points[b],size)
                result['pairs'][f'{a}-{b}'] = {'safe':True,'reason':None,'mesh':mesh}
            except PreparationError as error:
                result['pairs'][f'{a}-{b}'] = {'safe':False,'reason':str(error),'mesh':None}
    result['build_id'] = sha256(json.dumps(result,sort_keys=True).encode()).hexdigest()
    result['ready']=all(p['preparation']['ready'] for p in result['poses'].values()) and all(p['safe'] for p in result['pairs'].values())
    if material.get('external_reference'):
        # A new canonical arch cannot inherit readiness from old phonetic pose
        # checks alone. Audit the full real intermediate aperture against its
        # observed crowns; retain failed packages for inspection.
        from app.workflows.oral.prepare_reference_dentition import audit_transitions
        audit=audit_transitions(result,output,{'upper':upper,'lower':lower},
                                {'upper':protected_upper,'lower':protected_lower})
        result['external_reference_validation']=audit
        result['reference']['source_reference_sha256']=material['reference_sha256']
        result['ready']=result['ready'] and audit['valid']
    result['promotion_policy']='Do not promote this candidate when ready=false. Failed poses remain inspectable with explicit failure reasons.'
    write_report(output/'package.json',result)
    snapshot=output.parent/'pipeline-snapshots'/result['algorithm_sha256']
    for name in ALGORITHM_SOURCES:
        target=snapshot/name;target.parent.mkdir(parents=True,exist_ok=True);shutil.copyfile(ROOT/name,target)
    run=snapshot/'runs'/result['build_id'];run.mkdir(parents=True,exist_ok=True)
    shutil.copyfile(manifest,run/'input-manifest.json');shutil.copyfile(material_manifest,run/'material-manifest.json')
    write_report(run/'package.json',result)
    print(json.dumps({'poses':pose_ids,'upper_fit':registration,'lower_fit':{p:result['poses'][p]['lower_translation'] for p in pose_ids},
                      'unsafe_pairs':{p:v['reason'] for p,v in result['pairs'].items() if not v['safe']}}))
    return result


def install(output, destination):
    """Install only a separate candidate; promotion is a later explicit step."""
    package = json.loads((output/'package.json').read_text())
    if not package.get('endpoints_finalized'):
        raise PreparationError('Finalize browser endpoints before installing a candidate; inspection previews are never runtime fallbacks.')
    if any(not p['safe'] for p in package['pairs'].values()): raise PreparationError('Candidate contains invalid geometry; not installed.')
    shutil.copytree(output,destination,dirs_exist_ok=True,ignore=shutil.ignore_patterns('*.png'))
    # Runtime SDF and edit masks are PNG; include those explicitly.
    for pose in package['poses'].values():
        for key in ('mask','aperture_sdf','endpoint'):
            target = destination/pose[key]; target.parent.mkdir(exist_ok=True); shutil.copyfile(output/pose[key],target)


def finalize(output, browser):
    """Publish endpoints only from the same compositor used for live frames."""
    package=json.loads((output/'package.json').read_text())
    report=json.loads((browser/'report.json').read_text())
    if report['build_id']!=package['build_id'] or report['maximumEndpointDelta']!=0 or report['maximumOutsideAlpha']!=0:
        raise PreparationError('Browser export failed identity/outside-mask checks.')
    if len(report['checks'])!=len(package['pairs'])*9:
        raise PreparationError('Incomplete directed transition export.')
    for pose in package['poses']:
        raw=browser/f'endpoint-{pose}.rgba'
        if not raw.exists() or digest(raw)!=report.get('raw_endpoint_sha256',{}).get(pose):
            raise PreparationError('Missing or changed straight RGBA endpoint export.')
        endpoint=Image.frombytes('RGBA',(package['crop']['width'],package['crop']['height']),raw.read_bytes())
        export_lossless_pair(endpoint,output/pose.lower(),'endpoint')
    package['browser_verification']={'report_sha256':digest(browser/'report.json'),'frames':len(report['checks']),
                                   'maximum_endpoint_delta':0,'outside_mask_alpha':0}
    package['endpoint_policy']='Unified browser compositor; preliminary inspection images are not runtime fallbacks.'
    package['endpoints_finalized']=True
    write_report(output/'package.json',package)
    make_review_sheet(output,package)


def make_review_sheet(output,package):
    w,h=package['crop']['width'],package['crop']['height']
    base=Image.open(output/'base.webp').convert('RGBA');x,y=package['crop']['x'],package['crop']['y']
    base_crop=base.crop((x,y,x+w,y+h))
    sheets={}
    for pose,data in package['poses'].items():
        sheet=Image.new('RGB',(w*4,h+50),'#17161b');draw=ImageDraw.Draw(sheet)
        for column,(label,file) in enumerate((('Source generation','registered-source.webp'),('Previous','previous.webp'),('Prepared masks','semantic-overlay.webp'),('Current','endpoint.webp'))):
            path=output/pose.lower()/file
            if not path.exists():continue
            frame=base_crop.copy();frame.alpha_composite(Image.open(path).convert('RGBA'))
            sheet.paste(frame.convert('RGB'),(column*w,30));draw.text((column*w+8,5),f'{pose} - {label}',fill='white',font_size=16)
        if not data['preparation']['ready']:draw.text((8,h+32),'Preparation failed: original pose is incompatible. Not promoted.',fill='#ffb4ab',font_size=14)
        sheet.save(output.parent/f'{pose.lower()}-comparison.png');sheets[pose]=sheet
    all_poses=Image.new('RGB',(w*4,(h+50)*len(sheets)),'#17161b')
    for index,sheet in enumerate(sheets.values()):all_poses.paste(sheet,(0,index*(h+50)))
    all_poses.save(output.parent/'all-endpoints-comparison.png')


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--manifest',type=Path,required=True);parser.add_argument('--material',type=Path,required=True)
    parser.add_argument('--semantics',type=Path,required=True);parser.add_argument('--output',type=Path,required=True)
    parser.add_argument('--poses',nargs='+'); parser.add_argument('--install',type=Path)
    parser.add_argument('--finalize',type=Path)
    args = parser.parse_args()
    try:
        if args.finalize: finalize(args.output.resolve(),args.finalize.resolve())
        else: prepare(args.manifest.resolve(),args.material.resolve(),args.semantics.resolve(),args.output.resolve(),args.poses)
        if args.install: install(args.output.resolve(),args.install.resolve())
    except (ValueError,PreparationError) as error: raise SystemExit(str(error)) from None
