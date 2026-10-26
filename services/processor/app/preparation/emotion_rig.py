"""Source-bound emotional fields with explicit articulation/material protection."""
from hashlib import sha256
import json

import cv2
import numpy as np
from PIL import Image

from app.preparation.expressions import BROWS, bump, validate, field_from_image
from app.preparation.eyes import EYE_RINGS
from app.preparation.faces import PreparationError
from app.preparation.emotion_articulation import prepare_emotion_articulation
from app.preparation.emotion_calibration import distinguish_negative_fields

STATES = ('neutral', 'joyful', 'playful', 'affectionate', 'curious', 'sad', 'angry', 'fearful', 'surprised', 'disgusted')


def prepare_emotion_rig(points, size, oral, oral_root, expressions, output, *, refine_states=None):
    previous = expressions.get('emotion_rig')
    changed = set(refine_states) if refine_states is not None else {'angry', 'fearful', 'disgusted'}
    if not changed or not changed.issubset(STATES) or 'neutral' in changed:
        raise PreparationError('Refinement requires supported non-neutral emotion states.')
    if refine_states is not None and (not previous or any(key not in previous for key in ('eyelid_tension', 'rest_opening'))):
        raise PreparationError('Scoped refinement requires existing explicit lid/opening calibration; preserve legacy resources.')
    retained = [i for i, state in enumerate(STATES) if state not in changed]
    points = np.asarray(points, float)[:, :2]
    if points.shape != (478, 2) or not np.isfinite(points).all():
        raise PreparationError('Finite cached MediaPipe landmarks are required.')
    crop = expressions['crop']
    width, height = expressions['field_size']
    yy, xx = np.indices((height, width), float)
    x = crop['x'] + (xx + .5) * crop['width'] / width
    y = crop['y'] + (yy + .5) * crop['height'] / height
    mw = float(np.linalg.norm(points[61] - points[291]))
    if mw < 16:
        raise PreparationError('Insufficient mouth resolution.')
    center = (points[61] + points[291]) / 2
    guard = np.zeros((size[1], size[0]), np.uint8)
    oc = oral['crop']
    # Preserve actual aperture contents through every existing articulation.
    # This includes tongue/cavity, rather than treating enamel as the only owner.
    for pose in oral['poses'].values():
        mask = np.asarray(Image.open(oral_root / pose['mask']).convert('RGBA'))
        aperture = np.uint8(mask[..., 1] > 127) * 255
        guard[oc['y']:oc['y']+oc['height'], oc['x']:oc['x']+oc['width']] |= aperture
        for row in ('upper', 'lower'):
            alpha = np.asarray(Image.open(oral_root / oral['reference'][row]).convert('RGBA'))[..., 3]
            shift = pose.get(row + '_translation', [0, 0])
            matrix = np.float32([[1, 0, oc['x'] + shift[0]], [0, 1, oc['y'] + shift[1]]])
            guard = np.maximum(guard, cv2.warpAffine(alpha, matrix, size))
    for ring in EYE_RINGS:
        cv2.fillPoly(guard, [np.rint(points[list(ring)]).astype(np.int32)], 255)
    cv2.ellipse(guard, tuple(np.rint(center).astype(int)), (round(mw*.22), round(mw*.12)), 0, 0, 360, 255, -1)
    padding = int(np.ceil(max(crop['width']/width, crop['height']/height)*2)) + 2
    guard = cv2.dilate(guard, cv2.getStructuringElement(cv2.MORPH_ELLIPSE, (padding*2+1,)*2))
    distance = cv2.distanceTransform(np.uint8(guard == 0), cv2.DIST_L2, 5)
    support = np.clip(distance / (mw*.1), 0, 1)
    support = cv2.remap(support, x.astype(np.float32), y.astype(np.float32), cv2.INTER_LINEAR)
    support = support*support*(3-2*support)
    # Channels: corners down, lip/cheek tension, inner brow up, outer brow up,
    # inner brow inward/down, cheek lift. All distances scale with measured anatomy.
    controls = np.zeros((6, height, width, 2), float)
    for corner in (points[61], points[291]):
        sign = -1 if corner[0] < center[0] else 1
        weight = bump(x, y, corner, [mw*.48, mw*.45])
        cheek = bump(x, y, corner+[sign*mw*.2, -mw*.3], [mw*.6, mw*.7])
        controls[0, ..., 1] += (weight+.3*cheek)*mw*.075
        controls[1, ..., 0] += sign*(weight+.15*cheek)*mw*.04
        controls[5, ..., 1] -= cheek*mw*.04
    for ids in BROWS:
        brow = points[list(ids)]
        bw = float(np.ptp(brow[:, 0]))
        inner = brow[np.argmin(abs(brow[:, 0]-center[0]))]
        outer = brow[np.argmax(abs(brow[:, 0]-center[0]))]
        wi = bump(x, y, inner, [bw*.9, bw*.9])
        wo = bump(x, y, outer, [bw*.9, bw*.9])
        controls[2, ..., 1] -= wi*bw*.19
        controls[3, ..., 1] -= wo*bw*.17
        controls[4, ..., 1] += wi*bw*.15
        controls[4, ..., 0] += np.sign(center[0]-inner[0])*wi*bw*.08
    combinations = np.asarray([
        # Positive moods reuse the registered/joint smile's cheek motion.
        # A second cheek field around its protected oral region creates a
        # disconnected band even when the field passes geometry checks.
        [0,0,0,0,0,0], [0,0,0,.15,0,0], [0,0,.1,.4,0,0],
        [0,0,.2,0,0,0], [0,0,.45,.6,0,0], [1,-.2,.9,-.25,0,-.25],
        [.25,.8,0,-.2,1,0], [.2,.8,1,.55,0,0], [0,.3,.85,1,0,0],
        [.45,.5,0,-.35,.65,.35],
    ])
    # Mouth actions are solved in the articulation mesh, not as a tooth-warping image field.
    controls[0:2] = 0
    fields = np.einsum('sc,chwk->shwk', combinations, controls) * support[None, ..., None]
    lip_top=oral['crop']['y']+min(float(np.asarray(pair['mesh'][key])[:40,1].min())
                                for pair in oral['pairs'].values() if pair.get('safe')
                                for key in ('rest','open'))
    fields = distinguish_negative_fields(fields, points, x, y, support, STATES,lip_top=lip_top)
    edge = np.minimum.reduce([x-crop['x'], crop['x']+crop['width']-x, y-crop['y'], crop['y']+crop['height']-y])
    taper = np.clip(edge/(np.linalg.norm(points[234]-points[454])*.12),0,1)
    fields *= (taper*taper*(3-2*taper))[None,...,None]
    fields[:, [0,-1]] = 0
    fields[:, :, [0,-1]] = 0
    fixed_units = None
    if previous:
        old = np.asarray(Image.open(output/previous['field']['file']).convert('RGBA'))
        if previous['field_size'] != [width, height] or previous['states'] != list(STATES):
            raise PreparationError('Existing emotion calibration has incompatible field geometry.')
        fixed_units = previous['units_per_byte']
        old = (old[..., :2].reshape(len(STATES), height, width, 2).astype(float)-128)*fixed_units
        fields[retained] = old[retained]
    if fixed_units:
        # Retain historical pages/units exactly; new pages cannot silently clip.
        for i, state in enumerate(STATES):
            if i not in retained:
                limit = fixed_units*115
                fields[i] = limit*np.tanh(fields[i]/limit)
    from itertools import combinations as state_pairs
    legacy = field_from_image(Image.open(output/expressions['field']['file']), expressions['units_per_byte'])
    reductions = np.ones(len(STATES))
    attempts = []
    blends = {}
    for _ in range(18):
        scaled = fields*reductions[:,None,None,None]
        units = fixed_units or max(.001, float(abs(scaled).max())/115)
        packed = np.uint8(np.clip(np.rint(scaled/units)+128,0,255))
        decoded = (packed.astype(float)-128)*units
        def check(field):
            value=np.zeros((height,width,3));value[...,:2]=field;value[...,2]=legacy[...,2]
            return validate(value,crop,inverse_iterations=12,brow_attenuation=.75)
        checks = [check(field) for field in decoded]
        invalid = {i for i,result in enumerate(checks) if not result['valid']}
        blends = {}
        if not invalid:
            for a,b in state_pairs(range(len(STATES)),2):
                result=check((decoded[a]+decoded[b])*.5)
                blends[STATES[a]+'-'+STATES[b]]=result
                if not result['valid']:
                    invalid.update((a,b))
        attempts.append({'reductions':dict(zip(STATES,reductions.tolist())), 'checks':dict(zip(STATES,checks)), 'blends':blends})
        (output/'emotion-validation-attempts.json').write_text(json.dumps(attempts))
        if not invalid:
            break
        if refine_states is not None:
            if any(not checks[i]['valid'] for i in retained):
                raise PreparationError('Frozen emotion field fails validation; do not silently alter it.')
            invalid.difference_update(retained)
            if not invalid:
                raise PreparationError('Frozen emotion transition fails validation.')
        for i in invalid:
            if i:reductions[i]*=.85
    if invalid:
        raise PreparationError('Emotion fields fail the unchanged deformation gates.')
    atlas = np.full((height*len(STATES), width, 4), 128, np.uint8)
    atlas[..., 2] = 0; atlas[..., 3] = 255
    atlas[..., :2] = packed.reshape(height*len(STATES), width, 2)
    filename = output/'emotion-fields.png'
    Image.fromarray(atlas).save(filename)
    Image.fromarray(guard).save(output/'emotion-protected.png')
    oral_rig=prepare_emotion_articulation(points,oral,expressions,STATES,output/'emotion-articulation-attempts.jsonl')
    lids=[0,.12,.11,.07,.01,.03,.28,.025,0,.24]
    if refine_states is not None:
        for i in retained:
            name=STATES[i]
            if name in previous['oral']['states'] and oral_rig['states'][name] != previous['oral']['states'][name]:
                raise PreparationError('Scoped refinement changed a frozen articulation: '+name)
            if 'eyelid_tension' in previous:
                lids[i]=previous['eyelid_tension'][i]
    return {'rest_opening':[0,0,0,0,0,0,0,.26,.4,.18], 'eyelid_tension':lids,
            'calibration':'negative-action-separation/2', 'refined_states':sorted(changed), 'edit_coverage':'deformed-oral-coverage/1', 'oral':oral_rig, 'version':'emotion-rig/1', 'source_sha256':expressions['source_sha256'],
            'motion_build_id':expressions['motion_build_id'], 'oral_build_id':oral['build_id'],
            'crop':crop, 'field':{'file':filename.name,'sha256':sha256(filename.read_bytes()).hexdigest()},
            'inverse_iterations':12, 'manual_brow_attenuation':.75, 'field_size':[width,height], 'states':list(STATES), 'units_per_byte':units,
            'amplitude_reduction':dict(zip(STATES,reductions.tolist())), 'validation':{'valid':True,'states':dict(zip(STATES,checks)), 'transition_midpoints':blends},
            'manual_brow_composition_validated':True, 'visual_status':'unreviewed', 'manual_masks':False, 'provider_requests':0}
