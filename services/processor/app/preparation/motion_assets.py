"""One-time subject matting and relative depth; no inference during playback."""
from hashlib import sha256
import json
from pathlib import Path
import time
from urllib.request import urlopen

import cv2
import numpy as np
from PIL import Image

from .faces import PreparationError

SEGMENTER_URL = 'https://storage.googleapis.com/mediapipe-models/image_segmenter/selfie_multiclass_256x256/float32/1/selfie_multiclass_256x256.tflite'
DEPTH_REPO = 'depth-anything/Depth-Anything-V2-Small-hf'


def binding(image,models):
    return {'source_rgb_sha256':sha256(image.convert('RGB').tobytes()).hexdigest(),
            'algorithm_sha256':sha256(Path(__file__).read_bytes()).hexdigest(),'models':models}


def cached(output,name,key):
    path=output/name
    if not path.exists():return None
    record=json.loads(path.read_text())
    if record.get('cache_binding')!=key:return None
    for name,digest in record['outputs'].items():
        path=output/name
        if not path.is_file() or sha256(path.read_bytes()).hexdigest()!=digest:
            raise PreparationError('A cached motion resource changed; preserve and inspect it before rebuilding.')
    return record


def save_record(output,name,record,key,files):
    record={**record,'cache_binding':key,'outputs':{name:sha256((output/name).read_bytes()).hexdigest() for name in files}}
    (output/name).write_text(json.dumps(record,indent=2));return record


def fetch(url, target):
    target.parent.mkdir(parents=True, exist_ok=True)
    if not target.exists():
        with urlopen(url, timeout=120) as response:
            data = response.read(300_000_001)
        if len(data) > 300_000_000:
            raise PreparationError('Model exceeds its download limit.')
        temporary = target.with_suffix('.tmp'); temporary.write_bytes(data); temporary.replace(target)
    return sha256(target.read_bytes()).hexdigest()


def refine_subject(rgb, probability):
    """Compatibility entry point; use continuous source-resolution matting."""
    from .foreground import prepare_matte
    alpha, color, _ = prepare_matte(rgb, probability)
    return alpha, color


def subject(image, points, models, output):
    import mediapipe as mp
    import psutil
    from .foreground import prepare_matte, cache_settings, VERSION
    model = models/'selfie-multiclass-256-float32-v1.tflite'
    digest = fetch(SEGMENTER_URL, model); pin = models/'segmenter.json'
    if pin.exists() and json.loads(pin.read_text())['sha256'] != digest:
        raise PreparationError('Segmentation model checksum changed.')
    pin.write_text(json.dumps({'url': SEGMENTER_URL, 'sha256': digest}, indent=2))
    from .hair_matting import model_files
    _, hair_model=model_files(models)
    key=binding(image,{'segmentation_sha256':digest,'matting':cache_settings(),'hair_model':hair_model})
    previous=cached(output,'segmentation.json',key)
    if previous:return previous
    options = mp.tasks.vision.ImageSegmenterOptions(
        base_options=mp.tasks.BaseOptions(model_asset_path=str(model)),
        running_mode=mp.tasks.vision.RunningMode.IMAGE,
        output_category_mask=False, output_confidence_masks=True)
    rgb = np.ascontiguousarray(image.convert('RGB')); started = time.perf_counter()
    with mp.tasks.vision.ImageSegmenter.create_from_options(options) as segmenter:
        result = segmenter.segment(mp.Image(image_format=mp.ImageFormat.SRGB, data=rgb))
        probabilities = np.stack([mask.numpy_view().copy().squeeze() for mask in result.confidence_masks])
    if probabilities.shape[0] != 6:
        raise PreparationError('Unexpected semantic subject classes.')
    probability = 1-probabilities[0]
    probability = cv2.resize(probability, image.size, interpolation=cv2.INTER_LINEAR)
    alpha, foreground_rgb, trimap = prepare_matte(rgb, probability, points[4,:2], probabilities, models, output)
    np.savez_compressed(output/'class-confidence.npz', probabilities=probabilities)
    Image.fromarray(np.rint(trimap*255).astype(np.uint8)).save(output/'trimap.png')
    Image.fromarray(foreground_rgb).save(output/'foreground-color.png')
    if alpha.mean()<.05:
        raise PreparationError('Insufficient foreground coverage.')
    rgba = np.dstack((foreground_rgb, np.rint(alpha*255).astype(np.uint8)))
    rgba[rgba[...,3]==0,:3] = 0
    Image.fromarray(rgba).save(output/'foreground.png')
    Image.fromarray(rgba).save(output/'foreground.webp',lossless=True,exact=True,method=6)
    Image.fromarray(rgba[...,3]).save(output/'foreground-alpha.png')
    np.save(output/'subject-confidence.npy',probability)
    for color,name in [((255,255,255),'white'),((20,19,22),'dark')]:
        background=Image.new('RGBA',image.size,color+(255,))
        Image.alpha_composite(background,Image.fromarray(rgba)).convert('RGB').save(output/f'matte-{name}.jpg',quality=94)
    process = psutil.Process()
    record={'model_url':SEGMENTER_URL,'model_sha256':digest,'seconds':time.perf_counter()-started,
            'method':VERSION,'settings':cache_settings(),'manual_masks':False,
            'edge_rgb_decontamination':True,'refinement_size':list(image.size),
            'foreground_fraction':float(alpha.mean()),'class_names':['background','hair','body-skin','face-skin','clothes','other-accessories'],
            'peak_working_set_bytes':int(getattr(process.memory_info(),'peak_wset',process.memory_info().rss))}
    return save_record(output,'segmentation.json',record,key,['foreground.png','foreground.webp','foreground-alpha.png','foreground-color.png','trimap.png','class-confidence.npz','subject-confidence.npy','matte-white.jpg','matte-dark.jpg','hair-uncertain.png','hair-refinement.json'])



def depth(image, models, output):
    import torch
    import psutil
    from transformers import AutoImageProcessor, AutoModelForDepthEstimation
    torch.set_num_threads(2)
    directory = models/'depth-anything-v2-small-hf'; directory.mkdir(parents=True,exist_ok=True)
    pin = directory/'revision.json'
    if pin.exists():
        record=json.loads(pin.read_text())
    else:
        with urlopen(f'https://huggingface.co/api/models/{DEPTH_REPO}',timeout=60) as response:
            revision=json.load(response)['sha']
        record={'repo':DEPTH_REPO,'revision':revision,'files':{}}
    for name in ('config.json','preprocessor_config.json','model.safetensors'):
        value=fetch(f'https://huggingface.co/{DEPTH_REPO}/resolve/{record["revision"]}/{name}',directory/name)
        if name in record['files'] and value!=record['files'][name]:raise PreparationError('Pinned depth asset changed.')
        record['files'][name]=value
    pin.write_text(json.dumps(record,indent=2))
    key=binding(image,record)
    previous=cached(output,'depth.json',key)
    if previous:return previous
    processor=AutoImageProcessor.from_pretrained(directory,local_files_only=True,use_fast=False)
    model=AutoModelForDepthEstimation.from_pretrained(directory,local_files_only=True).eval()
    process=psutil.Process();before=process.memory_info().rss;started=time.perf_counter()
    inputs=processor(images=image,return_tensors='pt')
    with torch.inference_mode():
        result=model(**inputs).predicted_depth
        raw=torch.nn.functional.interpolate(result.unsqueeze(1),size=(image.height,image.width),mode='bicubic',align_corners=False)[0,0].cpu().numpy()
    if not np.isfinite(raw).all():raise PreparationError('Nonfinite depth map.')
    lo,hi=np.percentile(raw,[2,98]);normalized=np.clip((raw-lo)/max(hi-lo,1e-8),0,1)
    np.save(output/'relative-depth.npy',raw)
    Image.fromarray(np.rint(normalized*65535).astype(np.uint16)).save(output/'depth-16.png')
    Image.fromarray(np.rint(normalized*255).astype(np.uint8)).save(output/'depth.webp',lossless=True,exact=True)
    record={**record,'method':'depth-anything-v2-small-hf-cpu/1','input_shape':list(inputs['pixel_values'].shape),
        'seconds':time.perf_counter()-started,'rss_before_bytes':before,'rss_after_bytes':process.memory_info().rss,
        'peak_working_set_bytes':int(getattr(process.memory_info(),'peak_wset',process.memory_info().rss)),
        'normalization_percentiles':[float(lo),float(hi)],'torch':torch.__version__,'device':'cpu'}
    return save_record(output,'depth.json',record,key,['relative-depth.npy','depth-16.png','depth.webp'])
