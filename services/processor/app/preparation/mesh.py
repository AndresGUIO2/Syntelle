"""Local lip topology with fixed crop boundary and checked interpolation."""
import cv2
import numpy as np
from .faces import PreparationError

OUTER = [61,185,40,39,37,0,267,269,270,409,291,375,321,405,314,17,84,181,91,146]
INNER = [78,191,80,81,82,13,312,311,310,415,308,324,318,402,317,14,87,178,88,95]


def make_mesh(source: np.ndarray, target: np.ndarray, bounds: tuple[int, int, int, int]) -> dict:
    x, y, w, h = bounds
    border = np.array([[0,0],[w/2,0],[w,0],[w,h/2],[w,h],[w/2,h],[0,h],[0,h/2]])
    rest = np.vstack([source[OUTER+INNER, :2]-[x,y], border])
    opened = np.vstack([target[OUTER+INNER, :2]-[x,y], border])
    if not np.isfinite(rest).all() or not np.isfinite(opened).all():
        raise PreparationError("Mesh contains nonfinite landmarks.")
    adjustments = []
    for points in (rest, opened):
        original = points.copy()
        for i in range(1,10):
            upper, lower = 20+i, 40-i
            center = (points[upper]+points[lower])/2
            gap = max(0.75, points[lower,1]-points[upper,1])
            points[upper] = [center[0], center[1]-gap/2]
            points[lower] = [center[0], center[1]+gap/2]
        adjustments.append(float(np.linalg.norm(points-original, axis=1).max()))
    average = (rest+opened)/2
    ids = list(range(20))+list(range(40,48))
    subdivision = cv2.Subdiv2D((-1,-1,w+2,h+2))
    try:
        for i in ids: subdivision.insert(tuple(average[i]))
    except cv2.error:
        raise PreparationError("Lip points exceed the mouth crop.") from None
    triangles = []
    for triangle in subdivision.getTriangleList().reshape(-1,3,2):
        indices = [min(ids,key=lambda i: np.linalg.norm(average[i]-point)) for point in triangle]
        if max(np.linalg.norm(average[i]-point) for i,point in zip(indices,triangle)) < 0.01:
            if cv2.pointPolygonTest(average[:20].astype(np.float32),tuple(triangle.mean(axis=0)),False) < 0:
                triangles.append(indices)
    for i in range(20):
        j=(i+1)%20
        candidates=([[i,j,20+i],[j,20+j,20+i]], [[i,j,20+j],[i,20+j,20+i]])
        def valid(candidate):
            for t in np.linspace(0,1,101):
                vertices=((1-t)*rest+t*opened)[np.array(candidate)]
                u,v=vertices[:,1]-vertices[:,0],vertices[:,2]-vertices[:,0]
                area=u[:,0]*v[:,1]-u[:,1]*v[:,0]
                # A concave lip quad can have a non-inverting but backwards
                # diagonal. Require the ring's winding, not each triangle's own.
                if np.min(area)<=0: return False
            return True
        chosen=next((candidate for candidate in candidates if valid(candidate)),None)
        if chosen is None: raise PreparationError(f"Lip quad {i} has no non-inverting diagonal.")
        triangles.extend(chosen)
    cavity_start=len(triangles)
    triangles.extend([[20,21,39],[29,30,31]])
    for i in range(1,9): triangles.extend([[20+i,21+i,40-i],[21+i,39-i,40-i]])
    def areas(points):
        v=points[np.array(triangles)]; a,b=v[:,1]-v[:,0],v[:,2]-v[:,0]
        return a[:,0]*b[:,1]-a[:,1]*b[:,0]
    signs=np.sign(areas(average))
    c,mid,end=areas(rest)*signs,areas(average)*signs,areas(opened)*signs
    a=2*(end+c-2*mid); b=end-c-a
    minimum=float(min(c.min(),end.min()))
    for aa,bb,cc in zip(a,b,c):
        if aa>0 and 0 < -bb/(2*aa) < 1:
            t=-bb/(2*aa); minimum=min(minimum,float(aa*t*t+bb*t+cc))
    if minimum<=0 or abs(float(sum(abs(areas(average))))/2-w*h)>0.01:
        raise PreparationError(f"Transition mesh inverts or does not cover the crop (minimum={minimum:.6f}, coverage={float(sum(abs(areas(average))))/2-w*h:.6f}).")
    return {"width":w,"height":h,"rest":rest.round(6).tolist(),"open":opened.round(6).tolist(),
            "triangles":triangles,"cavity_triangles":list(range(cavity_start,len(triangles))),
            "landmark_ids":OUTER+INNER,"minimum_signed_double_area":minimum,"regularization_px":adjustments}
