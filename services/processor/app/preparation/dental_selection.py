"""Local reference ranking from original pixels; no generative or remote calls."""
from hashlib import sha256
from io import BytesIO
import json

import cv2
import numpy as np
from PIL import Image, ImageOps, ImageDraw

from .faces import PreparationError
from .mouth import transform_points
from .oral import cavity_mask


VERSION = 'visible-dentition/1'


def load_originals(reference, batch, size, bounds, poses):
    """Decode original provider bytes and replay recorded stable-anchor alignment."""
    images, points, provenance = {}, {}, {}
    source_hash = None
    for pose in poses:
        if pose == 'Rest':
            continue
        folder = reference if pose == 'A' else batch/pose.lower()
        record_path = folder/'run.json'
        record = json.loads(record_path.read_text(encoding='utf-8'))
        generation = record['generation']
        path = folder/generation['file']
        if not path.is_file():
            raise PreparationError(f'{pose}: original provider image is missing; no regeneration or fallback is allowed.')
        data = path.read_bytes()
        digest = sha256(data).hexdigest()
        if digest != generation['raw_sha256']:
            raise PreparationError(f'{pose}: original provider cache changed.')
        if source_hash is not None and record['source_sha256'] != source_hash:
            raise PreparationError('Dental candidates belong to different originals.')
        source_hash = record['source_sha256']
        with Image.open(BytesIO(data)) as source:
            source.load()
            decoded = ImageOps.exif_transpose(source).convert('RGB')
        if abs((decoded.width/decoded.height)/(size[0]/size[1])-1) > 0.02:
            raise PreparationError('A candidate changed framing.')
        decoded = decoded.resize(size,Image.Resampling.LANCZOS)
        transform = np.array(record['metrics']['transform'])
        aligned = cv2.warpAffine(np.asarray(decoded),transform,size,flags=cv2.INTER_LANCZOS4,borderMode=cv2.BORDER_REFLECT_101)
        left,top,right,bottom = bounds
        images[pose] = Image.fromarray(aligned[top:bottom,left:right])
        landmark_bytes=(folder/'variant-landmarks.json').read_bytes()
        raw_points = np.array(json.loads(landmark_bytes)['landmarks_px'])
        points[pose] = transform_points(raw_points,transform)-[left,top]
        provenance[pose] = {'raw_sha256':digest,'run_sha256':sha256(record_path.read_bytes()).hexdigest(),
                            'source_sha256':source_hash,'landmarks_sha256':sha256(landmark_bytes).hexdigest(),'alignment':record['metrics']['transform'],
                            'anchor_rmse_px':record['metrics']['anchor_rmse_px']}
    return images,points,provenance


def detect_dental_rows(image, points, settings):
    """Conservative neutral-enamel proposal inside the landmark mouth aperture.

    This is a color/geometry heuristic, not semantic teeth/tongue segmentation.
    A dark separation is required; ambiguous touching rows fail closed.
    """
    raw = np.asarray(image.convert('RGB'),dtype=np.float32)
    width = float(np.linalg.norm(points[61,:2]-points[291,:2]))
    empty = Image.new('L',image.size)
    if width < 10 or not np.isfinite(points).all():
        return empty,empty,{'eligible':False,'reasons':['Unusable mouth landmarks.'],'score':0.0}
    aperture = np.asarray(cavity_mask(image.size,points)) > 0
    red,green,blue = raw.transpose(2,0,1)
    brightness = cv2.cvtColor(raw.astype(np.uint8),cv2.COLOR_RGB2GRAY)
    strength = np.minimum.reduce([
        np.clip((green/(red+1)-settings['minimum_g_over_r'])/settings['color_ramp'],0,1),
        np.clip((blue/(red+1)-settings['minimum_b_over_r'])/settings['color_ramp'],0,1),
        np.clip((brightness.astype(float)-settings['minimum_brightness'])/settings['brightness_ramp'],0,1),
    ])
    core = (strength >= settings['mask_threshold']) & aperture
    proposed = core.astype(np.uint8)
    if core.sum() >= settings['grabcut_minimum_seed_pixels']:
        # Learn the current mouth's foreground/background colors from automatic seeds.
        # No manual polygons, poses or per-character RGB samples are used here.
        labels = np.full(aperture.shape,cv2.GC_BGD,dtype=np.uint8)
        labels[aperture] = cv2.GC_PR_BGD
        labels[(strength >= settings['growth_threshold']) & aperture] = cv2.GC_PR_FGD
        labels[core] = cv2.GC_FGD
        red_tissue = (green/(red+1)<settings['hard_background_g_over_r']) | (brightness<settings['hard_background_brightness'])
        labels[red_tissue | ~aperture] = cv2.GC_BGD
        if np.any(labels==cv2.GC_FGD):
            cv2.setRNGSeed(settings['grabcut_seed'])
            cv2.grabCut(raw.astype(np.uint8),labels,None,np.zeros((1,65)),np.zeros((1,65)),settings['grabcut_iterations'],cv2.GC_INIT_WITH_MASK)
            proposed = (np.isin(labels,[cv2.GC_FGD,cv2.GC_PR_FGD]) & aperture).astype(np.uint8)
    count,labels,stats,_ = cv2.connectedComponentsWithStats(proposed,8)
    minimum_area = max(2,round(width*width*settings['minimum_component_area_ratio']))
    keep = [i for i in range(1,count) if stats[i,cv2.CC_STAT_AREA]>=minimum_area]
    mask = np.isin(labels,keep) & aperture
    yy,xx = np.nonzero(mask)
    if not len(yy):
        return empty,empty,{'eligible':False,'reasons':['No confident enamel pixels.'],'score':0.0}
    profile = mask.sum(axis=1).astype(float)
    smooth = cv2.GaussianBlur(profile[None],(0,0),max(0.5,width*settings['profile_smoothing_ratio']))[0]
    center_x = float((points[61,0]+points[291,0])/2)
    central = np.abs(np.arange(image.width)-center_x)<width*settings['central_fraction']/2
    splits = []
    for cut in range(int(yy.min())+1,int(yy.max())):
        upper = mask.copy(); upper[cut:] = False
        lower = mask.copy(); lower[:cut] = False
        a,b = upper.sum(),lower.sum()
        if min(a,b)<width*width*settings['minimum_row_area_ratio']:
            continue
        peak = min(float(smooth[:cut].max()),float(smooth[cut:].max()))
        separation = max(0.0,1-float(smooth[cut])/max(1,peak))
        balance = min(a,b)/max(a,b)
        splits.append((separation*np.sqrt(balance),cut,upper,lower,separation))
    if not splits:
        return Image.fromarray((mask*255).astype(np.uint8)),empty,{'eligible':False,'reasons':['Two usable rows were not found.'],'score':0.0}
    _,cut,upper,lower,separation = max(splits,key=lambda item:(item[0],-item[1]))
    upper_y,upper_x = np.nonzero(upper)
    lower_y,lower_x = np.nonzero(lower)
    a = np.nonzero(upper[:,central])[0]; b = np.nonzero(lower[:,central])[0]
    gap = float(np.percentile(b,5)-np.percentile(a,95)) if len(a) and len(b) else 0.0
    upper_span = (upper_x.max()-upper_x.min()+1)/width
    lower_span = (lower_x.max()-lower_x.min()+1)/width
    coverage = min(upper_span,lower_span)
    balance = min(upper.sum(),lower.sum())/max(upper.sum(),lower.sum())
    area = float(mask.sum()/width**2)
    # Texture sharpness is a small capped tiebreaker, never a substitute for coverage.
    laplacian = cv2.Laplacian(brightness,cv2.CV_32F)
    sharpness = min(1.0,float(np.mean(laplacian[mask]**2))/settings['sharpness_scale'])
    score = 60*min(coverage,1) + 20*min(area/settings['target_area_ratio'],1) + 10*separation + 6*balance + 4*sharpness
    reasons = []
    if score < settings['minimum_score']: reasons.append('Insufficient dental extraction utility.')
    if coverage < settings['minimum_row_span_ratio']: reasons.append('Insufficient visible width in one dental row.')
    if separation < settings['minimum_separation']: reasons.append('Dental rows cannot be separated confidently.')
    if gap/width < settings['minimum_gap_ratio']: reasons.append('No reliable central gap between the rows.')
    metrics = {'eligible':not reasons,'reasons':reasons,'score':round(float(score),6),
               'upper_span_ratio':round(float(upper_span),6),'lower_span_ratio':round(float(lower_span),6),
               'area_ratio':round(area,6),'row_balance':round(float(balance),6),
               'separation':round(float(separation),6),'central_gap_px':round(gap,4),
               'gap_ratio':round(gap/width,6),'sharpness':round(sharpness,6),'split_y':int(cut),
               'upper_pixels':int(upper.sum()),'lower_pixels':int(lower.sum()),'mouth_width_px':round(width,4)}
    # Keep original enamel pixels; there is no color correction, scaling or tooth warp.
    return Image.fromarray((upper*255).astype(np.uint8)),Image.fromarray((lower*255).astype(np.uint8)),metrics


def select_reference(images, points, settings):
    candidates = {}
    masks = {}
    for pose in sorted(images):
        upper,lower,metrics = detect_dental_rows(images[pose],points[pose],settings)
        candidates[pose] = metrics
        masks[pose] = (upper,lower)
    ranked = sorted((pose for pose,entry in candidates.items() if entry['eligible']),key=lambda pose:(-candidates[pose]['score'],pose))
    selected = ranked[0] if ranked else None
    margin = candidates[ranked[0]]['score']-candidates[ranked[1]]['score'] if len(ranked)>1 else None
    if margin is not None and margin < settings['minimum_score_margin']:
        selected = None
    return {'method':VERSION,'selected_pose':selected,'status':'candidate_selected' if selected else 'needs_review',
            'ranking':ranked,'score_margin':round(margin,6) if margin is not None else None,
            'candidates':candidates,'mask_method':'landmark aperture + neutral-color seeds + deterministic GrabCut proposal',
            'new_provider_requests':0},masks


def save_selection_evidence(output,images,masks,report):
    output.mkdir(parents=True,exist_ok=True)
    sheet=Image.new('RGB',(1316,max(1,(len(images)+1)//2)*260),'#FAFAF9');draw=ImageDraw.Draw(sheet)
    for i,pose in enumerate(images):
        folder=output/pose.lower();folder.mkdir(exist_ok=True)
        upper,lower=masks[pose]
        upper.save(folder/'upper-proposal.png');lower.save(folder/'lower-proposal.png')
        images[pose].save(folder/'original-aligned.png')
        annotated=mask_preview(images[pose],upper,lower)
        annotated.save(folder/'mask-overlay.png')
        x=(i%2)*658;y=(i//2)*260
        draw.text((x+8,y+4),f"{pose}: {report['candidates'][pose]['score']:.1f} - {'eligible' if report['candidates'][pose]['eligible'] else 'review'}",fill='#24212B',font_size=17)
        sheet.paste(images[pose].resize((329,221)),(x,y+30));sheet.paste(annotated.resize((329,221)),(x+329,y+30))
    sheet.save(output/'selection-contact.png')
    (output/'selection.json').write_text(json.dumps(report,indent=2)+'\n',encoding='utf-8')


def mask_preview(image,upper,lower):
    rgb=np.asarray(image.convert('RGB')).copy()
    u,l=np.asarray(upper)>0,np.asarray(lower)>0
    rgb[u]=np.rint(rgb[u]*0.55+np.array([30,185,240])*0.45).astype(np.uint8)
    rgb[l]=np.rint(rgb[l]*0.55+np.array([215,120,230])*0.45).astype(np.uint8)
    return Image.fromarray(rgb)
