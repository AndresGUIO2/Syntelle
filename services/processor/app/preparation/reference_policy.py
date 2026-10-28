"""Source-bound pose qualification; missing directions never imply symmetry."""
from hashlib import sha256
import json
from pathlib import Path

import numpy as np

from app.core.paths import ROOT
from .faces import PreparationError

DEFAULT_POLICY = ROOT / 'docs/characters/parallax-reference-policy-v5.json'


def load_policy(path=DEFAULT_POLICY):
    data=Path(path).read_bytes();policy=json.loads(data)
    if policy.get('version') not in ('parallax-reference-policy/2','parallax-reference-policy/3','parallax-reference-policy/4','parallax-reference-policy/5'):
        raise PreparationError('Use a versioned pose-reference qualification policy.')
    return policy,sha256(data).hexdigest()


def prompt_for(policy, slot):
    if slot not in policy['views']:raise PreparationError('Unsupported pose-reference slot.')
    return policy['prompt_prefix']+policy['views'][slot]['prompt']


def lateral_visibility(x, center, face_width, yaw):
    # Positive measured yaw exposes the image-left side. This is a conservative
    # pose-facing support estimate, not reconstructed hidden-surface visibility.
    return np.clip(.5+np.sign(yaw)*(center-np.asarray(x))/max(1,face_width*.1),0,1)


def assess_pose(policy, slot, achieved, source_points, reference_points, canvas):
    view=policy['views'][slot];angle=np.asarray(achieved,float);reasons=[]
    axis=view['axis'];signed=float(angle[axis])*view['sign']
    if not np.isfinite(angle).all():raise PreparationError('Non-finite measured reference pose.')
    if not view['minimum_degrees']<=signed<=view['maximum_degrees']:
        reasons.append('wrong-direction-or-insufficient-turn')
    for other in range(3):
        if other==axis:continue
        maximum=policy['maximum_roll_degrees'] if other==2 else policy['maximum_cross_axis_degrees']
        if abs(angle[other])>maximum:reasons.append('cross-axis-contamination')
    a=np.asarray(source_points,float)[[33,263],:2]
    b=np.asarray(reference_points,float)[[33,263],:2]
    distance=float(np.linalg.norm(a[1]-a[0]))
    if distance<=0:raise PreparationError('Missing source eye geometry for framing qualification.')
    ratio=float(np.linalg.norm(b[1]-b[0])/distance)
    shift=float(np.max(np.abs((b.mean(axis=0)-a.mean(axis=0))/np.asarray(canvas))))
    if not policy['eye_distance_ratio'][0]<=ratio<=policy['eye_distance_ratio'][1]:reasons.append('camera-scale-changed')
    if shift>policy['maximum_eye_center_shift_fraction']:reasons.append('framing-shift')
    return dict(accepted=not reasons,reasons=reasons,slot=slot,achieved_degrees=angle.tolist(),
        expected_axis=axis,expected_sign=view['sign'],axis=axis,
        eye_distance_ratio=ratio,eye_center_shift_fraction=shift,identity_visually_approved=False)


def direction_coverage(policy, records):
    usable={r['slot']:r for r in records if r.get('accepted')}
    missing=[slot for slot in policy['required_yaw_slots'] if slot not in usable]
    limits={}
    for slot in policy['required_yaw_slots']:
        if slot in usable:
            limits[slot]=max(0,abs(usable[slot]['achieved_degrees'][0])-policy['yaw_safety_margin_degrees'])
    return dict(version='direction-coverage/1',valid=not missing,missing_slots=missing,
        directional_yaw_limits=limits,maximum_symmetric_yaw=min(limits.values()) if not missing else 0,
        identity_visually_approved=False)


def require_direction_coverage(coverage):
    if not coverage['valid']:
        raise PreparationError('Incomplete measured yaw coverage: '+', '.join(coverage['missing_slots'])+'. No expanded package may be exported; preserve evidence without retrying.')


def validate_export_coverage(motion):
    views=motion.get('projection',{}).get('multiview')
    if not views:return
    policy,_=load_policy()
    measured=[dict(slot='yaw-left' if v['pose'][0]<0 else 'yaw-right',accepted=True,achieved_degrees=v['pose']) for v in views['views'] if v['axis']==0]
    coverage=direction_coverage(policy,measured)
    require_direction_coverage(coverage)
    if motion['projection']['bounds']['yaw']>coverage['maximum_symmetric_yaw']:
        raise PreparationError('Expanded yaw exceeds the measured reference coverage.')
