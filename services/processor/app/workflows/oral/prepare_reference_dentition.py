"""Offline canonical dentition from a cached, immutable laughter reference."""
from __future__ import annotations

import argparse
from copy import deepcopy
from hashlib import sha256
import json
from pathlib import Path
import shutil

import cv2
import numpy as np
from PIL import Image

from app.core.paths import ROOT
from app.preparation.archive import encoded
from app.preparation.aperture_lighting import prepare_lighting
from app.preparation.mesh import INNER
from app.preparation.dental_coverage import continue_attachment
from app.preparation.eyes import align_eyes
from app.preparation.faces import PreparationError
from app.preparation.mouth import export_lossless_pair
from app.preparation.reference_dentition import VERSION, extract_reference_rows, mask_overlay
from app.preparation.semantic_rig import enforce_bite, fit_lower, fit_upper, row_profile


def digest(path: Path) -> str:
    return sha256(path.read_bytes()).hexdigest()


def placed(mask, translation, size):
    return cv2.warpAffine(np.asarray(mask), np.array([[1,0,translation[0]],[0,1,translation[1]]],float),
                         size, flags=cv2.INTER_NEAREST)


def transition_opening(mesh, a, b, amount):
    """Replay the semantic shader's barycentric SDF interpolation offline."""
    rest=np.asarray(mesh['rest'],np.float32).astype(float)
    target=np.asarray(mesh['open'],np.float32).astype(float)
    current=rest*(1-amount)+target*amount
    h,w=mesh['height'],mesh['width'];distance=np.full((h,w),-128.,np.float32)
    for ids in mesh['triangles']:
        ids=np.asarray(ids);p=current[ids]
        low=np.maximum(0,np.ceil(p.min(0)-.5).astype(int))
        high=np.minimum([w-1,h-1],np.floor(p.max(0)-.5).astype(int))
        if (high<low).any():continue
        yy,xx=np.mgrid[low[1]:high[1]+1,low[0]:high[0]+1]
        xy=np.stack([xx+.5,yy+.5,np.ones_like(xx)],-1)
        matrix=np.vstack([p.T,np.ones(3)])
        weights=xy@np.linalg.inv(matrix).T
        inside=(weights>=-1e-7).all(-1)
        sa=weights@rest[ids]-.5;sb=weights@target[ids]-.5
        av=cv2.remap(a.astype(np.float32),sa[...,0].astype(np.float32),sa[...,1].astype(np.float32),
                     cv2.INTER_LINEAR,borderMode=cv2.BORDER_REPLICATE)
        bv=cv2.remap(b.astype(np.float32),sb[...,0].astype(np.float32),sb[...,1].astype(np.float32),
                     cv2.INTER_LINEAR,borderMode=cv2.BORDER_REPLICATE)
        value=(av*(1-amount)+bv*amount-128)/8
        patch=distance[low[1]:high[1]+1,low[0]:high[0]+1];patch[inside]=value[inside]
    # Core visible enamel; the half-pixel antialiased boundary is reported by
    # rendered visual verification rather than mistaken for a full new crown.
    return distance>=0


def audit_transitions(package, source, material, masks):
    crop=package['crop'];size=crop['width'],crop['height'];checks={};issues=[]
    sdfs={pose:np.asarray(Image.open(source/config['aperture_sdf'])) for pose,config in package['poses'].items()}
    for name,pair in package['pairs'].items():
        a,b=name.split('-')
        if a==b:continue
        if not pair['safe']:
            issues.append(f'{name}: prior articulation geometry is unsafe.');continue
        samples=[]
        for amount in np.linspace(0,1,9):
            opening=transition_opening(pair['mesh'],sdfs[a],sdfs[b],float(amount))
            from_pose,to_pose=package['poses'][a],package['poses'][b]
            row_masks={};unsupported=0
            for row in ('upper','lower'):
                shift=np.asarray(from_pose[row+'_translation'])*(1-amount)+np.asarray(to_pose[row+'_translation'])*amount
                observed=placed(masks[row],shift,size)>=128
                complete=placed(material[row].getchannel('A'),shift,size)>=128
                row_masks[row]=observed&opening
                unsupported+=int((complete&~observed&opening).sum())
            overlap=int((row_masks['upper']&row_masks['lower']).sum())
            samples.append({'amount':float(amount),'unsupported_pixels':unsupported,'overlap_pixels':overlap})
        checks[name]={'samples':samples,'valid':all(not v['unsupported_pixels'] and not v['overlap_pixels'] for v in samples)}
        if not checks[name]['valid']:issues.append(f'{name}: intermediate aperture exceeds observed crown coverage or dental clearance.')
    return {'method':'barycentric-semantic-sdf-coverage/1','directed_pairs':len(checks),'samples_per_pair':9,
            'valid':not issues,'issues':issues,'pairs':checks}


def rebase_oral(source: Path, output: Path, fragments, material, masks, extraction, reference_hash, reference_gap=None):
    """Reuse articulation geometry and analytically audit the new opaque arch.

    The old arch supplies only a measured incisal registration and placement
    observations. No old dental pixels enter the new canonical material.
    """
    package = json.loads((source/'package.json').read_bytes())
    result = deepcopy(package)
    crop = package['crop']; size = crop['width'],crop['height']
    old = {row: Image.open(source/package['reference'][row]).convert('RGBA') for row in ('upper','lower')}
    old_upper = old['upper'].getchannel('A')
    upper_y, registration = fit_upper(masks['upper'], [old_upper])
    # A single common arch calibration must retain the measured FV contact.
    # Apply it to every articulation, rather than moving the arch per viseme.
    if 'FV' in package['poses']:
        fv=np.asarray(Image.open(source/package['poses']['FV']['aperture_sdf']))>=128
        lip=row_profile(Image.fromarray(fv.astype(np.uint8)*255),'bottom')
        incisal=row_profile(masks['upper'],'bottom')+upper_y
        common=np.isfinite(lip)&np.isfinite(incisal)
        if common.any():
            gap=float(np.median(lip[common]-incisal[common]))
            correction=max(0.,gap-1.)
            reference_width=int((np.asarray(masks['upper'])>0).any(0).sum())
            if correction>reference_width*.03:
                raise PreparationError('Shared reference arch cannot preserve FV contact within its measured registration bound.')
            upper_y+=correction
            registration['contact_calibration']={'method':'shared-measured-fv-contact/1',
                'initial_median_gap_px':gap,'global_upper_translation_correction_px':correction,
                'target_gap_px':1.,'all_poses_share_arch':True}
            registration['translation']=[0,upper_y]
    shutil.copytree(source, output)
    for row in ('upper','lower'):
        export_lossless_pair(material[row], output, row+'-teeth')
    result['reference'] = {'upper':'upper-teeth.webp','lower':'lower-teeth.webp',
                           'registration':registration,'source_reference_sha256':reference_hash,
                           'method':VERSION,'observed_masks':{row:f'../{row}-mask.png' for row in masks}}
    result['upper_translation'] = [0,upper_y]
    result['material_hashes'] = {row:digest(output/(row+'-teeth.webp')) for row in material}
    result['reference_dentition'] = {'version':VERSION,'source_reference_sha256':reference_hash,
                                     'manual_masks':False,'source_rgb_preserved':True,
                                     'previous_build_id':package['build_id']}
    # Freeze the relative articulation jaw movements first. Search only ONE
    # common lower-row calibration; never tweak individual visemes to pass.
    jaw = {}; apertures = {}; expected_lower = {}
    for pose, config in result['poses'].items():
        opening=np.asarray(Image.open(source/config['aperture_sdf']))>=128
        aperture=Image.fromarray(opening.astype(np.uint8)*255)
        previous_config=package['poses'][pose]
        old_lower=placed(old['lower'].getchannel('A'),previous_config['lower_translation'],size)
        observed_lower=Image.fromarray(((old_lower>=128)&opening).astype(np.uint8)*255)
        jaw[pose]=fit_lower(masks['lower'],material['lower'].getchannel('A'),aperture,
                            observed_lower,pose in ('Rest','MBP'))
        apertures[pose]=opening
        expected_lower[pose]=bool(np.asarray(observed_lower).any())
    lower_width=int((np.asarray(masks['lower'])>0).any(0).sum())
    alternatives=[]
    for delta in np.arange(0,lower_width*.03+.25,.5):
        unsupported=overlap=missing=0
        for pose,initial in jaw.items():
            opening=apertures[pose]
            u=placed(masks['upper'],[0,upper_y],size)>=128
            lo=placed(masks['lower'],[0,initial+delta],size)>=128
            complete=placed(material['lower'].getchannel('A'),[0,initial+delta],size)>=128
            unsupported+=int((complete&~lo&opening).sum())
            overlap+=int((u&lo&opening).sum())
            missing+=int(expected_lower[pose] and not (lo&opening).any())
        alternatives.append({'delta_px':float(delta),'unsupported_pixels':unsupported,
                             'overlap_pixels':overlap,'missing_rows':missing})
    selected=next((entry for entry in alternatives if not any(entry[k] for k in
                  ('unsupported_pixels','overlap_pixels'))),None)
    lower_calibration=selected['delta_px'] if selected else 0.
    result['reference_dentition']['shared_lower_calibration']={
        'method':'bounded-shared-observed-coverage/1','delta_px':lower_calibration,
        'maximum_delta_px':lower_width*.03,'candidates':alternatives,'valid':selected is not None,
        'per_pose_overrides':False}
    reports = {}; problems = []
    if selected is None:
        problems.append('No bounded common lower-row calibration preserves observed coverage and dental clearance.')
    for pose, config in result['poses'].items():
        sdf = np.asarray(Image.open(source/config['aperture_sdf']))
        opening = sdf >= 128
        aperture = Image.fromarray(opening.astype(np.uint8)*255)
        prior_config = package['poses'][pose]
        old_lower = placed(old['lower'].getchannel('A'), prior_config['lower_translation'], size)
        observed_lower = Image.fromarray(((old_lower>=128)&opening).astype(np.uint8)*255)
        lower_y = jaw[pose]+lower_calibration
        config['upper_translation'] = [0,upper_y]
        config['lower_translation'] = [0,lower_y]
        report = {'visible_pixels':{},'unsupported_visible_pixels':{},'rows_overlap':0}
        rows = {}
        for row, shift in [('upper',upper_y),('lower',lower_y)]:
            observed = placed(masks[row], [0,shift], size)>=128
            complete = placed(material[row].getchannel('A'), [0,shift], size)>=128
            rows[row] = observed & opening
            report['visible_pixels'][row] = int((complete&opening).sum())
            unsupported = int((complete&~observed&opening).sum())
            report['unsupported_visible_pixels'][row] = unsupported
            if unsupported:
                problems.append(f'{pose}/{row}: {unsupported} hidden attachment pixels would be exposed.')
        report['rows_overlap'] = int((rows['upper']&rows['lower']).sum())
        report['prior_lower_row_visible'] = expected_lower[pose]
        report['lower_visibility_changed'] = expected_lower[pose] != bool(rows['lower'].any())
        if report['rows_overlap']:
            problems.append(f'{pose}: upper and lower dental ownership overlaps.')
        if pose in ('Rest','MBP') and opening.any():
            problems.append(f'{pose}: closed-lip articulation exposes an aperture.')
        if pose == 'FV':
            top = row_profile(Image.fromarray(rows['upper'].astype(np.uint8)*255),'bottom')
            lip = row_profile(aperture,'bottom')
            common = np.isfinite(top)&np.isfinite(lip)
            contact = float(np.median(lip[common]-top[common])) if common.any() else None
            report['median_fv_contact_gap_px'] = contact
            report['fv_lower_row_visible'] = bool(rows['lower'].any())
            if contact is None or contact>max(2.,common.sum()*.015) or rows['lower'].any():
                problems.append('FV: the new arch does not preserve measured upper-incisor/lower-lip contact.')
        reports[pose] = report
    result['pipeline_version'] = package['pipeline_version']+'+'+VERSION
    result['ready'] = bool(package.get('ready')) and extraction['ready'] and not problems
    result['reference_dentition']['validation'] = {'ready':result['ready'],'issues':problems,'poses':reports,
        'existing_directed_pair_count':len(result['pairs']),
        'geometry_unchanged':result['pairs']==package['pairs'],
        'policy':'Do not expose hidden attachment continuation or substitute old dental pixels.'}
    transitions=audit_transitions(result,output,material,masks)
    result['reference_dentition']['validation']['transitions']=transitions
    if not transitions['valid']:
        result['ready']=False
        result['reference_dentition']['validation']['ready']=False
        result['reference_dentition']['validation']['issues'].extend(transitions['issues'])
    result['endpoints_finalized'] = False
    if reference_gap is not None:
        prepare_lighting(result, output, reference_gap)
    result.pop('build_id',None)
    result['build_id'] = sha256(encoded(result)).hexdigest()
    (output/'package.json').write_bytes(encoded(result))
    return result['reference_dentition']['validation']


def run(manifest: Path, cached_landmarks: Path, output: Path):
    if output.exists():
        raise PreparationError('Preserve earlier candidates; the output directory must be new.')
    settings = json.loads(manifest.read_text(encoding='utf-8'))
    resolve = lambda key: (ROOT/settings[key]).resolve()
    portrait, reference = resolve('original'), resolve('laugh')
    oral_manifest = resolve('oral')
    package = json.loads(oral_manifest.read_bytes())
    source_hash = digest(portrait)
    if package['source_sha256'] != source_hash:
        raise PreparationError('The oral package and immutable portrait differ.')
    base = Image.open(portrait).convert('RGB')
    ref = Image.open(reference).convert('RGB')
    points = np.asarray(json.loads(resolve('landmarks').read_bytes())['landmarks_px'])
    generated = np.asarray(json.loads(cached_landmarks.read_bytes())['landmarks_px'])
    aligned, target, registration = align_eyes(base,ref,points,generated,base.width/ref.width)
    crop = package['crop']; x,y,w,h = [crop[k] for k in ('x','y','width','height')]
    oral_image = aligned.crop((x,y,x+w,y+h))
    masks, fragments, extraction = extract_reference_rows(oral_image,target-[x,y])
    output.mkdir(parents=True)
    (output/'inputs.json').write_bytes(manifest.read_bytes())
    aligned.save(output/'registered-reference.png')
    oral_image.save(output/'reference-mouth.png')
    mask_overlay(oral_image,masks).resize((w*3,h*3)).save(output/'mask-overlay.png')
    material = {}; continuation = {}
    for row in ('upper','lower'):
        masks[row].save(output/(row+'-mask.png'))
        export_lossless_pair(fragments[row],output,row+'-fragment')
        material[row], continuation[row] = continue_attachment(fragments[row],row)
        # Every proposed attachment is audited against the real observed crown
        # mask. A runtime package cannot hide missing material by deleting it.
        continuation[row]['installed_only_when_hidden'] = True
        export_lossless_pair(material[row],output,row+'-teeth')
    (output/'extraction.json').write_bytes(encoded(extraction))
    material_spec = {'version':VERSION,'source_sha256':source_hash,'reference_sha256':digest(reference),
        'reference_landmarks_sha256':digest(cached_landmarks),'registration':registration,
        'crop':crop,'rows':{row:{'material':row+'-teeth.webp','observed':row+'-mask.png',
                               'fragment':row+'-fragment.webp','continuation':continuation[row]}
                          for row in ('upper','lower')},
        'ready':extraction['ready'],'review_status':'composition-review-required','manual_masks':False,
        'immutable_inputs':{key:digest(resolve(key)) for key in ('original','landmarks','oral','laugh')}}
    reference_inner = target[np.asarray(INNER)]
    reference_gap = float(reference_inner[15][1]-reference_inner[5][1])
    validation = rebase_oral(oral_manifest.parent,output/'oral',fragments,material,masks,extraction,digest(reference),reference_gap)
    material_spec['extraction_ready'] = extraction['ready']
    material_spec['ready'] = extraction['ready'] and validation['ready']
    material_spec['speech_validation'] = validation
    (output/'dental-material.json').write_bytes(encoded(material_spec))
    print(json.dumps({'output':str(output),'extraction_ready':extraction['ready'],
                      'speech_ready':validation['ready'],'issues':validation['issues']}))
    return material_spec


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--manifest',type=Path,required=True)
    parser.add_argument('--cached-landmarks',type=Path,required=True)
    parser.add_argument('--output',type=Path,required=True)
    args = parser.parse_args()
    try:
        run(args.manifest.resolve(),args.cached_landmarks.resolve(),args.output.resolve())
    except PreparationError as error:
        raise SystemExit(str(error)) from None
