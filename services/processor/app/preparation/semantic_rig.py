"""Measured mouth contours and shared opaque dentition; no character calibration."""
import cv2
import numpy as np
from PIL import Image

from .faces import PreparationError
from .mesh import INNER, OUTER, make_mesh


def row_profile(mask, edge):
    active = np.asarray(mask) >= 128
    profile = np.full(active.shape[1], np.nan)
    for x in np.flatnonzero(active.any(axis=0)):
        ys = np.flatnonzero(active[:, x]); profile[x] = ys.min() if edge == 'top' else ys.max()
    return profile


def fit_upper(protected, observations):
    """Robust shared incisal registration from broad, independently observed rows."""
    anchor = row_profile(protected, 'bottom'); offsets = []; weights = []
    widest = max((np.isfinite(row_profile(m, 'bottom')).sum() for m in observations), default=0)
    for mask in observations:
        target = row_profile(mask, 'bottom'); common = np.isfinite(anchor) & np.isfinite(target)
        if common.sum() < max(8, widest * .6): continue
        offsets.append(float(np.median(target[common]-anchor[common]))); weights.append(int(common.sum()))
    if not offsets: raise PreparationError('No broad upper incisal observation for shared registration.')
    order = np.argsort(offsets); cumulative = np.cumsum(np.asarray(weights)[order])
    shift = offsets[order[np.searchsorted(cumulative, cumulative[-1]/2)]]
    return shift, {'method':'shared-weighted-median-incisal-registration/1','observed_offsets':offsets,'weights':weights,'translation':[0,shift]}


def register_oral_patch(image, masks, points, protected_upper, upper_y, closed=False):
    """Register generated articulation to the fixed arch without opening lips.

    Independent image generations move their entire mouth by a few pixels even
    after stable face alignment. Moving teeth to follow that drift breaks the
    shared rig. Instead translate the local image and all semantic regions as
    one unit; preserve the observed aperture's area and shape exactly.
    """
    anchor=row_profile(protected_upper,'bottom')+upper_y
    observed=row_profile(masks['upper_teeth'],'bottom')
    common=np.isfinite(anchor)&np.isfinite(observed)
    width=np.isfinite(anchor).sum();dy=0
    if not closed and common.sum()>=max(3,width*.2):
        dy=int(round(float(np.median(anchor[common]-observed[common]))))
        if abs(dy)>width*.12:
            raise PreparationError('Generated mouth drift exceeds reliable shared dental registration.')
    matrix=np.array([[1,0,0],[0,1,dy]],float)
    shifted_image=Image.fromarray(cv2.warpAffine(np.asarray(image),matrix,image.size,flags=cv2.INTER_NEAREST,borderMode=cv2.BORDER_REFLECT_101))
    shifted={name:Image.fromarray(cv2.warpAffine(np.asarray(mask),matrix,image.size,flags=cv2.INTER_NEAREST)) for name,mask in masks.items()}
    for name,mask in masks.items():
        if np.count_nonzero(np.asarray(mask))!=np.count_nonzero(np.asarray(shifted[name])):
            raise PreparationError('Oral registration clipped a semantic region.')
    return shifted_image,shifted,points+[0,dy],{'method':'observed-incisal-local-registration/1','translation':[0,dy],
        'observed_columns':int(common.sum()),'aperture_shape_unchanged':True,'lip_scaling':False}


def fit_lower(protected, completed, aperture, observed, closed=False):
    if closed: return 0.0
    anchor = row_profile(protected, 'top'); target = row_profile(observed, 'top')
    common = np.isfinite(anchor) & np.isfinite(target)
    if common.sum() >= 3:
        return float(np.median(target[common]-anchor[common]))
    # No lower row observed: place its complete attachment behind the measured
    # lower lip. This is derived coverage, not an arbitrary dental shutter.
    material = row_profile(completed, 'top'); opening = row_profile(aperture, 'bottom')
    common = np.isfinite(material) & np.isfinite(opening)
    return float(np.max(opening[common]-material[common])+1) if common.any() else 0.0


def enforce_bite(upper, lower, upper_y, lower_y, aperture):
    a=row_profile(upper,'bottom')+upper_y; b=row_profile(lower,'top')+lower_y
    opening=np.asarray(aperture)>=128
    common=np.isfinite(a)&np.isfinite(b)&opening.any(axis=0)
    if not common.any():return lower_y
    clearance=max(.5,np.flatnonzero(common).size*.003)
    return lower_y+max(0,float(np.max(a[common]-b[common]+clearance)))


def pose_quality(pose,masks):
    """Phonetic failures cannot be repaired by disguising a wrong generation."""
    issues=[]
    opening=np.asarray(masks['aperture'])>=128
    outer=np.flatnonzero((np.asarray(masks['outer_lips'])>=128).any(0))
    width=float(np.ptp(outer)+1) if len(outer) else 0
    top=row_profile(masks['aperture'],'top');bottom=row_profile(masks['aperture'],'bottom')
    columns=np.isfinite(top)&np.isfinite(bottom)
    gap=float(np.max(bottom[columns]-top[columns]+1)) if columns.any() else 0
    if pose in ('Rest','MBP') and opening.any():issues.append('Closed-lip pose exposes a mouth opening.')
    if pose=='A' and (width==0 or gap<width*.035):issues.append('A does not have a usable observed vowel opening.')
    if pose=='EI' and (width==0 or columns.sum()<width*.45):issues.append('EI is not a sufficiently spread observed mouth opening.')
    if pose in ('O','U') and width and columns.sum()>width*.85:issues.append('Rounded vowel opening is excessively spread.')
    if pose=='SZ':
        upper=row_profile(masks['upper_teeth'],'bottom');lower=row_profile(masks['lower_teeth'],'top')
        common=np.isfinite(upper)&np.isfinite(lower)
        if common.any() and float(np.median(lower[common]-upper[common]))>max(1.5,width*.04):
            issues.append('SZ dental rows are too widely separated for the close-row articulation.')
    if pose=='FV':
        lower=np.asarray(masks['lower_teeth'])>=128
        if lower.sum()>max(3,(np.asarray(masks['aperture'])>=128).sum()*.01):
            issues.append('Original FV exposes a lower dental row; it is not a valid upper-incisor/lower-lip contact pose.')
        upper=row_profile(masks['upper_teeth'],'bottom'); lip=row_profile(masks['aperture'],'bottom')
        common=np.isfinite(upper)&np.isfinite(lip)
        if not common.any() or float(np.median(lip[common]-upper[common]))>max(2,np.isfinite(lip).sum()*.015):
            issues.append('Original FV lacks measured upper-incisor contact with the lower lip.')
    return {'ready':not issues,'issues':issues}


def reconstruct_cavity(image, masks):
    raw = np.asarray(image.convert('RGB')); opening = np.asarray(masks['aperture']) >= 128
    dental = np.maximum(np.asarray(masks['upper_teeth']), np.asarray(masks['lower_teeth'])) >= 128
    tongue = (np.asarray(masks['tongue']) >= 128) & ~dental
    removal = (cv2.dilate(dental.astype(np.uint8), np.ones((3,3),np.uint8)) > 0) & opening & ~tongue
    if not removal.any(): return image.copy(), Image.fromarray(removal.astype(np.uint8)*255)
    seeds = opening & ~removal & ~tongue
    if seeds.sum() < 4: raise PreparationError('Insufficient observed cavity pixels for dental removal.')
    luminance=cv2.cvtColor(raw,cv2.COLOR_RGB2GRAY)
    # Donor selection is not alpha estimation. Dark observed cavity provides
    # hidden background; unsegmented bright crown gaps must not be copied into
    # a second dental row by the reconstruction.
    seeds &= luminance <= np.percentile(luminance[seeds],60)
    # Extend actual cavity RGB outside its boundary before inpainting, so the
    # reconstruction cannot borrow pale lip/skin pixels from across that edge.
    _, nearest = cv2.distanceTransformWithLabels((~seeds).astype(np.uint8), cv2.DIST_L2, 5, labelType=cv2.DIST_LABEL_PIXEL)
    lookup = np.vstack((np.zeros((1,3),np.uint8), raw[seeds]))
    canvas = lookup[np.minimum(nearest, len(lookup)-1)]
    canvas = cv2.GaussianBlur(canvas,(0,0),1)
    filled = cv2.inpaint(canvas, removal.astype(np.uint8)*255, 3, cv2.INPAINT_TELEA)
    result = raw.copy(); result[removal] = filled[removal]
    if not np.array_equal(result[tongue], raw[tongue]): raise PreparationError('Tongue changed during reconstruction.')
    return Image.fromarray(result), Image.fromarray(removal.astype(np.uint8)*255)


def contour_ring(mask, fallback=None):
    """Monotone upper/lower curves with common, left-to-right correspondence."""
    top = row_profile(mask, 'top'); bottom = row_profile(mask, 'bottom'); columns = np.flatnonzero(np.isfinite(top))
    if len(columns) < 3:
        if fallback is None: raise PreparationError('No usable semantic lip ring.')
        return fallback.copy()
    left, right = float(columns.min()), float(columns.max())
    # Equidistant x samples avoid incompatible arc-length matches between narrow
    # and wide mouths. Corners have a small geometry-only separation later.
    xs = np.linspace(left, right, 11)
    upper = np.column_stack((xs, np.interp(xs, columns, top[columns])))
    lower = np.column_stack((xs, np.interp(xs, columns, bottom[columns])))
    upper[0] = lower[0] = [left, (top[int(left)]+bottom[int(left)])/2]
    upper[-1] = lower[-1] = [right, (top[int(right)]+bottom[int(right)])/2]
    return np.vstack((upper, lower[-2:0:-1]))


def measured_points(original_points, masks):
    points = original_points.copy()
    outer = contour_ring(masks['outer_lips'])
    opening = np.asarray(masks['aperture']) >= 128
    if opening.any():
        inner = contour_ring(masks['aperture'])
    else:
        # A closed-pose mesh needs finite area, but its visibility texture is
        # exactly zero. This scaffold never opens the rendered closed mouth.
        center_y = float((original_points[13,1]+original_points[14,1])/2)
        left, right = outer[0], outer[10]
        width = right[0]-left[0]
        xs = np.linspace(left[0]+width*.08, right[0]-width*.08, 11)
        fraction = (xs-left[0])/width
        baseline = left[1]*(1-fraction)+right[1]*fraction
        baseline += (center_y-(left[1]+right[1])/2)*np.sin(np.pi*fraction)
        upper = np.column_stack((xs, baseline-.4*np.sin(np.linspace(0,np.pi,11))))
        lower = np.column_stack((xs, baseline+.4*np.sin(np.linspace(0,np.pi,11))))
        inner = np.vstack((upper,lower[-2:0:-1]))
    points[OUTER] = outer; points[INNER] = inner
    return points


def mesh_for(source, target, size):
    """Keep inversion checks mandatory; never alter semantic visibility to pass."""
    return make_mesh(source, target, (0,0,*size))


def aperture_sdf(mask, closed_points=None):
    active = (np.asarray(mask) >= 128).astype(np.uint8)
    if not active.any():
        if closed_points is None: return Image.new('L', mask.size, 0)
        crease = np.ones_like(active)
        path = np.asarray(closed_points)[INNER[:11]]
        cv2.polylines(crease,[np.rint(path).astype(np.int32)],False,0,1)
        distance = cv2.distanceTransform(crease,cv2.DIST_L2,cv2.DIST_MASK_PRECISE)
        return Image.fromarray(np.clip(np.rint(128-8*(distance+.75)),0,127).astype(np.uint8))
    inside = cv2.distanceTransform(active, cv2.DIST_L2, cv2.DIST_MASK_PRECISE)
    outside = cv2.distanceTransform(1-active, cv2.DIST_L2, cv2.DIST_MASK_PRECISE)
    # Signed native pixel distance encoded at 8 levels/pixel; boundary=128.
    return Image.fromarray(np.clip(np.rint(128+8*(inside-outside)),0,255).astype(np.uint8))
