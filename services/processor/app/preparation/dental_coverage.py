"""Conservative hidden enamel coverage from observed fragments, without new rows.

This is a 2D continuation of the selected material, not reconstructed dental anatomy.
It is deliberately independent of portrait names and native pixel coordinates.
"""
import argparse
from hashlib import sha256
import json
from pathlib import Path
import numpy as np
from PIL import Image

from .faces import PreparationError


def extract_fragment(image, coverage, excluded=None):
    """Retain semantic subpixel edges instead of binarizing a low-res crown.

    Foreground coverage comes solely from geometry. Fully covered enamel stays
    opaque, including dark enamel; only the contour's fractional edge is soft.
    """
    rgba=np.asarray(image.convert('RGBA')).copy()
    alpha=np.asarray(coverage.convert('L')).copy()
    if excluded is not None:
        alpha[np.asarray(excluded)>=128]=0
    rgba[...,3]=alpha
    rgba[alpha==0,:3]=0
    return Image.fromarray(rgba)


def neutralize_shared_lighting(upper,lower):
    """Separate a shared enamel reference from differently baked row shadows.

    Estimate controlled RGB gains from the selected pair only. Foreground alpha
    is unchanged; dark pixels remain material, never a transparency heuristic.
    This is a bounded illumination estimate, not recovered physical albedo.
    """
    rows=[np.asarray(image.convert('RGBA')).copy() for image in (upper,lower)]
    visible=[row[row[...,3]>=128,:3].astype(float) for row in rows]
    if any(len(pixels)<12 for pixels in visible):raise PreparationError('Insufficient enamel for shared illumination estimation.')
    pixels=np.concatenate(visible);light=pixels@[.2126,.7152,.0722]
    target=np.median(pixels[light>=np.percentile(light,60)],axis=0)
    outputs=[];records=[]
    for row,samples in zip(rows,visible):
        median=np.median(samples,axis=0)
        gain=np.clip((target+1)/(median+1),.8,2.6)
        row[...,:3]=np.rint(np.clip(row[...,:3].astype(float)*gain,0,255)).astype(np.uint8)
        row[row[...,3]==0,:3]=0
        outputs.append(Image.fromarray(row));records.append({'rgb_gain':gain.tolist(),'source_median_rgb':median.tolist()})
    return outputs,{'method':'paired-enamel-illumination/1','target_rgb':target.tolist(),'rows':records,
        'alpha_unchanged':True,'texture_operation':'one multiplicative color gain per row; no tooth geometry changes'}


def continue_attachment(fragment, row):
    source=np.asarray(fragment.convert('RGBA')); support=source[...,3]>=128
    columns=np.flatnonzero(support.any(0))
    if row not in ('upper','lower') or len(columns)<8:
        raise PreparationError('Insufficient observed dental material for continuation.')
    result=np.zeros_like(source); h,w=support.shape
    # Extend away from the incisal edge, behind the same lip. The edge itself
    # and every selected foreground pixel remain fixed. No second crown body
    # can create a horizontal rail or mismatched seam.
    depth=max(4,round(len(columns)*.18)); sign=-1 if row=='upper' else 1
    for x in columns:
        ys=np.flatnonzero(support[:,x]); edge=int(ys.min() if row=='upper' else ys.max())
        sample=ys[:min(3,len(ys))] if row=='upper' else ys[-min(3,len(ys)):]
        color=np.mean(source[sample,x,:3],axis=0)
        for step in range(1,depth+1):
            y=edge+sign*step
            if not 0<=y<h: continue
            # Gentle material shading is RGB-only; exposed enamel stays opaque.
            result[y,x,:3]=np.rint(color*(1-.12*(step/depth)**2)).astype(np.uint8)
            result[y,x,3]=255
    result[support]=source[support]; result[support,3]=255
    # Preserve antialiased source edges where no attachment was continued.
    fringe=(source[...,3]>0)&~support&(result[...,3]==0);result[fringe]=source[fringe]
    return Image.fromarray(result), {'method':'observed-attachment-continuation/1','row':row,
        'depth_px':depth,'source_pixels':int(support.sum()),'completed_pixels':int(((result[...,3]>0)&~support).sum()),
        'selected_rgb_unchanged':bool(np.array_equal(result[support,:3],source[support,:3])),
        'limitation':'Hidden 2D coverage extrapolated from observed enamel; not a complete anatomical denture.'}


if __name__=='__main__':
    from .mouth import export_lossless_pair
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--upper-fragment',type=Path,required=True)
    parser.add_argument('--lower-fragment',type=Path,required=True)
    parser.add_argument('--output',type=Path,required=True)
    args=parser.parse_args();args.output.mkdir(parents=True,exist_ok=True);records={}
    for row,path in [('upper',args.upper_fragment),('lower',args.lower_fragment)]:
        image,record=continue_attachment(Image.open(path),row)
        record['source_sha256']=sha256(path.read_bytes()).hexdigest()
        export_lossless_pair(image,args.output,f'{row}-teeth');records[row]=record
    (args.output/'continuation.json').write_text(json.dumps(records,indent=2))
