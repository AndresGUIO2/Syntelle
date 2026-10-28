"""Deterministic, source-bound pose guide; never an active character material."""
from hashlib import sha256
import json
import math
from pathlib import Path

import numpy as np
from PIL import Image, ImageDraw
from scipy.spatial import Delaunay

from .faces import PreparationError,MODEL_SHA256


def prepare_guide(source, geometry, slot, output):
    if slot not in ('yaw-left','yaw-right'):raise PreparationError('Yaw orientation guide required.')
    if output.exists():raise PreparationError('Preserve existing orientation guides.')
    original=Image.open(source);points=np.asarray(geometry['landmarks_px'],float)[:468].copy()
    if points.shape!=(468,3) or not np.isfinite(points).all():raise PreparationError('Complete cached source landmarks are required.')
    sign=-1 if slot=='yaw-left' else 1
    angle=math.radians(-sign*12)
    center=points[[234,454,10,152]].mean(axis=0)
    p=points-center
    rotated=p.copy();rotated[:,0]=math.cos(angle)*p[:,0]+math.sin(angle)*p[:,2]
    # Keep the nose's visible turn consistent with the image-axis arrow.
    scale=min(400/np.ptp(rotated[:,0]),430/np.ptp(rotated[:,1]))
    xy=rotated[:,:2]*scale+[256,345]
    canvas=Image.new('RGB',(512,640),'white');draw=ImageDraw.Draw(canvas)
    for tri in Delaunay(points[:,:2]).simplices:
        draw.line([tuple(xy[i]) for i in (*tri,tri[0])],fill=(162,182,198),width=1)
    draw.line([tuple(xy[168]),tuple(xy[1])],fill=(20,30,45),width=5)
    draw.ellipse((xy[1,0]-5,xy[1,1]-5,xy[1,0]+5,xy[1,1]+5),fill=(20,30,45))
    draw.text((170,12),'NOSE DIRECTION',fill=(20,30,45))
    start=256-sign*90;end=256+sign*150;y=58
    draw.line((start,y,end,y),fill=(20,30,45),width=8)
    draw.polygon([(end,y),(end-sign*25,y-18),(end-sign*25,y+18)],fill=(20,30,45))
    output.parent.mkdir(parents=True,exist_ok=True);canvas.save(output)
    record=dict(version='source-pose-guide/1',slot=slot,source_sha256=sha256(Path(source).read_bytes()).hexdigest(),
        model_sha256=MODEL_SHA256,source_size=list(original.size),requested_yaw=sign*12,
        guide_sha256=sha256(output.read_bytes()).hexdigest(),landmark_sha256=sha256(json.dumps(geometry,sort_keys=True).encode()).hexdigest(),
        material=False,provider_calls=0)
    output.with_suffix('.json').write_text(json.dumps(record,indent=2)+'\n')
    return record
