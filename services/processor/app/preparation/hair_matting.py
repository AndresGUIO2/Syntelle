"""Pinned local ViTMatte refinement on automatic, native-resolution hair tiles."""
from hashlib import sha256
import json
from pathlib import Path

import numpy as np
from PIL import Image

from .faces import PreparationError

REPO = 'hustvl/vitmatte-small-composition-1k'
REVISION = '6a58ad7646403c1df626fbd746900aec7361ea1d'
SETTINGS = {'repo': REPO, 'revision': REVISION, 'license': 'apache-2.0',
            'tile_size': 512, 'stride': 320, 'cpu_threads': 2,
            'native_resolution': True, 'hair_unknown_band_fraction': .05}


def source_binding():
    return {**SETTINGS,'implementation_sha256':sha256(Path(__file__).read_bytes()).hexdigest()}


def model_files(models):
    from .motion_assets import fetch
    directory=models/'vitmatte-small-composition-1k'
    pin_path=directory/'revision.json'
    if pin_path.exists():
        pin=json.loads(pin_path.read_text())
        if pin['repo']!=REPO or pin['revision']!=REVISION:
            raise PreparationError('Pinned hair matting model revision differs.')
    else:
        pin={**SETTINGS,'files':{}}
    for name in ['config.json','preprocessor_config.json','model.safetensors','README.md']:
        digest=fetch(f'https://huggingface.co/{REPO}/resolve/{REVISION}/{name}',directory/name)
        if name in pin['files'] and pin['files'][name]!=digest:
            raise PreparationError('Pinned hair matting model checksum changed.')
        pin['files'][name]=digest
    pin_path.write_text(json.dumps(pin,indent=2))
    return directory,pin


def origins(length, size=512, stride=320):
    return sorted(set([*range(0,max(1,length-size+1),stride),max(0,length-size)]))


def refine_native(rgb, trimap, initial, active, models):
    import torch
    from transformers import VitMatteForImageMatting, VitMatteImageProcessor

    directory,pin=model_files(models)
    torch.set_num_threads(SETTINGS['cpu_threads'])
    processor=VitMatteImageProcessor.from_pretrained(directory,local_files_only=True)
    model=VitMatteForImageMatting.from_pretrained(directory,local_files_only=True,use_safetensors=True).eval()
    h,w=trimap.shape
    image=Image.fromarray(rgb)
    seeds=Image.fromarray(np.rint(trimap*255).astype(np.uint8))
    total=np.zeros((h,w));coverage=np.zeros((h,w));tiles=0
    size=SETTINGS['tile_size']
    for y in origins(h,size,SETTINGS['stride']):
        for x in origins(w,size,SETTINGS['stride']):
            box=(x,y,min(x+size,w),min(y+size,h))
            region=np.s_[y:box[3],x:box[2]]
            if not active[region].any(): continue
            inputs=processor(images=image.crop(box),trimaps=seeds.crop(box),return_tensors='pt')
            with torch.inference_mode():
                alpha=model(**inputs).alphas[0,0,:box[3]-y,:box[2]-x].cpu().numpy()
            if not np.isfinite(alpha).all():
                raise PreparationError('Nonfinite native hair matte.')
            weight=np.maximum(.025,np.hanning(alpha.shape[0]))[:,None]*np.maximum(.025,np.hanning(alpha.shape[1]))[None,:]
            total[region]+=alpha*weight;coverage[region]+=weight;tiles+=1
    if np.any(active & (coverage==0)):
        raise PreparationError('Native hair tiles did not cover the uncertain boundary.')
    result=initial.copy()
    result[active]=total[active]/coverage[active]
    result[trimap==1]=1;result[trimap==0]=0
    return np.clip(result,0,1),{'settings':source_binding(),'model':pin,'tiles':tiles,'modified_region_pixels':int(active.sum())}
