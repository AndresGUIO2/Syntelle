"""Replay cached vision contours with explicit tooth-over-tongue occlusion."""
from hashlib import sha256
import json

import numpy as np
from PIL import Image

from app.gemini_contour_trial import Contours, polygon_is_simple
from app.gemini_contour_loop import checked_files
from .faces import PreparationError
from .mouth import transform_points
from .oral import polygon_mask


def resolve_occlusion(upper, lower, tongue):
    """Retain both enamel masks; keep the full tongue separately behind them."""
    u,l,t=(np.asarray(m,dtype=np.uint8) for m in (upper,lower,tongue))
    if u.shape!=l.shape or u.shape!=t.shape:
        raise PreparationError('Semantic masks have different dimensions.')
    if ((u>=128)&(l>=128)).any():
        raise PreparationError('Upper and lower enamel overlap; tooth ownership is ambiguous.')
    teeth=np.maximum(u,l)
    visible=np.where(teeth>0,0,t).astype(np.uint8)
    return Image.fromarray(visible),{
        'rule':'upper/lower enamel in front of tongue; no contour edits',
        'tongue_full_pixels':int((t>0).sum()),'tongue_visible_pixels':int((visible>0).sum()),
        'tongue_occluded_pixels':int(((t>0)&(teeth>0)).sum()),
        'tongue_deleted_from_source':False,'visible_semantic_overlap_pixels':int(((visible>0)&(teeth>0)).sum())}


def load_argon_masks(root, settings, selected, source_hash, raw_hash, crop, output):
    if selected!=settings['source_pose'] or settings['occlusion']!='teeth-over-tongue':
        raise PreparationError('Argon masks do not match the selected pose or occlusion policy.')
    experiment=root/settings['experiment'];record=json.loads((experiment/'run.json').read_text(encoding='utf-8'))
    if record['status']!='completed' or record['provider']['model']!='gemini-4-argon':
        raise PreparationError('Argon review is incomplete or uses another model.')
    checked_files(experiment,record['artifact_hashes'])
    source=root/settings['transform_experiment'];source_record=json.loads((source/'run.json').read_text(encoding='utf-8'))
    checked_files(source,source_record['input_hashes'])
    evidence=source_record['binding']['input']
    if evidence['source_sha256']!=source_hash or evidence['raw_sha256']!=raw_hash:
        raise PreparationError('Argon contours belong to another original image.')
    for name,value in record['binding']['source_sha256'].items():
        if sha256((source/name).read_bytes()).hexdigest()!=value:
            raise PreparationError('Argon source evidence changed.')
    data=(experiment/'review/contours.json').read_bytes()
    contours=Contours.model_validate_json(data)
    scale=settings['supersampling']
    if not isinstance(scale,int) or not 1<=scale<=8:
        raise PreparationError('Invalid Argon mask rasterization scale.')
    width,height=crop['width'],crop['height'];native=evidence['crop']
    transform=np.asarray(evidence['input_to_aligned_character'])[:2]
    masks={};converted={}
    for region in contours.regions:
        polygons=[]
        for polygon in region.polygons:
            if not polygon_is_simple(np.asarray(polygon,dtype=float)):raise PreparationError('Argon contour crosses itself.')
            points=transform_points(np.asarray(polygon)*[native['width']/1000,native['height']/1000],transform)-[crop['x'],crop['y']]
            if (points<0).any() or (points[:,0]>=width).any() or (points[:,1]>=height).any():
                raise PreparationError('Argon contour lies outside the package crop.')
            polygons.append(points.tolist())
        converted[region.label]=polygons
        masks[region.label]=polygon_mask((width*scale,height*scale),[(np.asarray(p)*scale).tolist() for p in polygons]).resize((width,height),Image.Resampling.LANCZOS)
    visible,resolution=resolve_occlusion(masks['upper_teeth'],masks['lower_teeth'],masks['tongue'])
    masks['tongue_visible']=visible
    output.mkdir(parents=True,exist_ok=True)
    for name,mask in masks.items():mask.save(output/f'{name}.png')
    provenance={'version':'argon-mask-replay/1','method':'Argon contours with deterministic teeth-over-tongue occlusion',
                'status':'owner-selected visual candidate; composition review pending','reviewer':'gemini-4-argon',
                'reviewed_at':record['finished_at'],'source_pose':selected,'source_sha256':source_hash,'raw_sha256':raw_hash,
                'contours_sha256':sha256(data).hexdigest(),'coordinates':{'space':'aligned-crop-pixels','crop':crop},
                'input_to_aligned_character':evidence['input_to_aligned_character'],'converted_contours':converted,
                'rasterization':{'supersampling':scale,'resampling':'LANCZOS'},'occlusion':resolution,
                'manual_contour_adjustments':False,'new_provider_requests':0,
                'notes':'Original model polygons and complete tongue mask retained. Only visible ownership is resolved. Pose-specific tissue is not replaced with the reference tongue.'}
    (output/'resolution.json').write_text(json.dumps(provenance,indent=2),encoding='utf-8')
    return masks,provenance
