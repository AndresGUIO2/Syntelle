"""Measured cheek motion for a boundary laugh, independent of manual smiling."""
import cv2
import numpy as np
from .expressions import field_from_image, validate
from .faces import PreparationError


def eye_closure(points, target):
    ratios = []
    for upper, lower in ((159, 145), (386, 374)):
        before = np.linalg.norm(points[upper, :2]-points[lower, :2])
        after = np.linalg.norm(target[upper, :2]-target[lower, :2])
        ratios.append(1-after/max(1, before))
    return float(np.clip(np.mean(ratios), 0, .4))


def prepare_boundary(oral, smile, points, target):
    """A laugh uses one resting jaw release, never arbitrary speech visemes."""
    from .mesh import OUTER, INNER
    mesh = oral['pairs']['Rest-A']['mesh']
    a, b = np.asarray(mesh['rest']), np.asarray(mesh['open'])
    sa = np.asarray(smile['pairs']['Rest-A']['rest_delta'])
    sb = np.asarray(smile['pairs']['Rest-A']['open_delta'])
    full = np.zeros_like(a)
    full[:40] = target[OUTER+INNER, :2]-points[OUTER+INNER, :2]
    def area(v, t):
        p, q = v[t[:, 1]]-v[t[:, 0]], v[t[:, 2]]-v[t[:, 0]]
        return p[:, 0]*q[:, 1]-p[:, 1]*q[:, 0]
    for reduction in (1., .9, .8, .7):
        delta = full*reduction
        t = np.asarray(smile['pairs']['Rest-A']['triangles']).copy()
        samples = []
        for u in np.linspace(0, .06, 5):
            base = a*(1-u)+b*u
            for laugh in np.linspace(0, 1, 9):
                d = (sa*(1-u)+sb*u)*(1-laugh)+delta*laugh
                for strength in np.linspace(0, 1, 9):
                    samples.append(base+d*strength)
        start = len(t)-18-40
        for i in range(20):
            j = (i+1)%20
            for choice in ([[i,j,20+i],[j,20+j,20+i]], [[i,j,20+j],[i,20+j,20+i]]):
                choice = np.asarray(choice)
                if all(np.all(area(v, choice)>0) for v in samples):
                    t[start+i*2:start+i*2+2] = choice
                    break
        minimum = 1.
        for u in np.linspace(0, .06, 5):
            base = a*(1-u)+b*u
            original = area(base, t)
            relevant = np.abs(original)>.05
            for laugh in np.linspace(0, 1, 9):
                d = (sa*(1-u)+sb*u)*(1-laugh)+delta*laugh
                for strength in np.linspace(0, 1, 9):
                    minimum = min(minimum, float((area(base+d*strength,t)[relevant]/original[relevant]).min()))
        if minimum > .2:
            return {'version':'joint-expression-articulation/1', 'mode':'boundary-laugh/1',
                    'maximum_articulation':.06, 'amplitude_reduction':reduction,
                    'pairs':{'Rest-A':{'rest_delta':delta.tolist(), 'open_delta':delta.tolist(), 'triangles':t.tolist()}},
                    'dental_translation':[0.,0.],
                    'rest_lower_translation':[0.,float(target[14,1]-points[14,1])*.5*reduction],
                    'rest_opening':float(target[14,1]-target[13,1])*reduction,
                    'validation':{'valid':True,'configurations':405,'minimum_triangle_ratio':minimum},
                    'manual_masks':False, 'generated_dental_installed':False}
    raise PreparationError('Boundary laughter exceeds its measured topology envelope.')


def prepare_soft_laughter(oral, points, reference_target, registered, registered_dir, foreground, output):
    """Reuse the accepted resting expression for a restrained brief chuckle."""
    from PIL import Image
    from .mesh import OUTER, INNER
    crop = registered['crop']
    x,y,w,h = (crop[k] for k in ('x','y','width','height'))
    field = field_from_image(Image.open(registered_dir/'field.png'),registered['units_per_byte']).astype(np.float32)
    fx = ((points[:,0]-x+.5)*field.shape[1]/w-.5).astype(np.float32)
    fy = ((points[:,1]-y+.5)*field.shape[0]/h-.5).astype(np.float32)
    delta = cv2.remap(field,fx[None],fy[None],cv2.INTER_LINEAR,borderMode=cv2.BORDER_CONSTANT)[0]
    target = points.copy()
    target[:,:2] += delta[:,:2]
    opening = max(0.,float(reference_target[14,1]-reference_target[13,1]))*.35
    for ring in (OUTER, INNER):
        for i, landmark in enumerate(ring):
            phase = i/10 if i<=10 else (20-i)/10
            target[landmark,1] += (-1 if i<=10 else 1)*opening*.5*np.sin(np.pi*phase)
    material = np.asarray(Image.open(registered_dir/'smile-material.png').convert('RGBA'))
    oc = oral['crop']
    lip = Image.fromarray(material).crop((oc['x']-x,oc['y']-y,oc['x']-x+oc['width'],oc['y']-y+oc['height']))
    lip.save(output/'laugh-lips.png')
    yy,xx = np.indices((h,w),np.float32)
    original = np.asarray(foreground.convert('RGBA'))
    skin = original[y:y+h,x:x+w]
    frames=[]
    for index,amount in enumerate(np.linspace(0,1,25)):
        sx,sy = xx.copy(),yy.copy()
        for _ in range(5):
            d=cv2.remap(field,((sx+.5)*field.shape[1]/w-.5).astype(np.float32),((sy+.5)*field.shape[0]/h-.5).astype(np.float32),cv2.INTER_LINEAR,borderMode=cv2.BORDER_CONSTANT)
            sx=xx-d[...,0]*amount;sy=yy-d[...,1]*amount
        alpha=material[...,3,None]/255*amount
        normalized=np.rint(skin[...,:3]*(1-alpha)+material[...,:3]*alpha).astype(np.uint8)
        rgba=cv2.remap(original,(sx+x).astype(np.float32),(sy+y).astype(np.float32),cv2.INTER_LINEAR,borderMode=cv2.BORDER_REFLECT_101)
        rgba[...,:3]=cv2.remap(normalized,sx.astype(np.float32),sy.astype(np.float32),cv2.INTER_LINEAR,borderMode=cv2.BORDER_REFLECT_101)
        name=f'laugh-skin-{index:02d}.webp'
        Image.fromarray(rgba).save(output/name,lossless=True,exact=True,method=6)
        frames.append({'file':name})
    return target,frames,{'file':'laugh-lips.png','registration':'existing-registered-closed-smile/1',
                          'generated_dental_pixels_installed':0,'reference_opening_fraction':.35}
