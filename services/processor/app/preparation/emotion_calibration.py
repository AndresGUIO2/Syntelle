"""Anatomy-scaled action separation using only cached source geometry."""
import numpy as np

from app.preparation.expressions import BROWS


def distinguish_negative_fields(fields, points, x, y, support, states, *, lip_top=None):
    width = float(np.linalg.norm(points[61]-points[291]))
    center = (points[61]+points[291])*.5
    angry = np.zeros_like(fields[0])
    fearful = np.zeros_like(angry)
    disgusted = np.zeros_like(angry)

    def smooth(anchor, radius):
        return np.exp(-.5*(((x-anchor[0])/radius[0])**2 + ((y-anchor[1])/radius[1])**2))

    for ids in BROWS:
        brow = points[list(ids)]
        bw = float(np.ptp(brow[:, 0]))
        inner = brow[np.argmin(abs(brow[:, 0]-center[0]))]
        outer = brow[np.argmax(abs(brow[:, 0]-center[0]))]
        sign = np.sign(center[0]-inner[0])
        wi = smooth(inner, [bw*.55, bw*.70])
        wo = smooth(outer, [bw*.60, bw*.70])
        # AU4: bring inner brows together and lower them, keeping the outer
        # brow relatively still. Broad falloff avoids a sharp eye-mask hinge.
        angry[..., 0] += sign*wi*bw*.18
        angry[..., 1] += (wi*.13 + wo*.008)*bw
        # AU1/2 with a little inner tension: worried raised brows, rather
        # than the relaxed bilateral lift used by curiosity/surprise.
        fearful[..., 0] += sign*wi*bw*.045
        fearful[..., 1] -= (wi*.20 - wo*.025)*bw
        # Disgust uses nose/upper-lip actions, not anger's converging brows.
        asymmetry = 1. if inner[0] < center[0] else .55
        disgusted[..., 1] += wi*bw*.018 - wo*bw*.020*asymmetry
    for index in (98, 327):
        nose = points[index]
        weight = smooth(nose, [width*.32, width*.38])
        strength = 1. if nose[0] < center[0] else .65
        disgusted[..., 1] -= weight*width*.085*strength
        disgusted[..., 0] += np.sign(center[0]-nose[0])*weight*width*.016*strength
    # Nasal motion must not pull the vermilion through a second image warp.
    # Reserve every registered upper-lip contour with a smooth anatomy-scaled
    # falloff, rather than adding or blending a new compositing mask.
    if lip_top is None:
        lip_top=float(points[[61,185,40,39,37,0,267,269,270,409,291],1].min())
    clearance=np.clip((lip_top-width*.08-y)/(width*.20),0,1)
    disgusted *= (clearance*clearance*(3-2*clearance))[...,None]
    for state, field in [('angry', angry), ('fearful', fearful), ('disgusted', disgusted)]:
        fields[states.index(state)] = field*support[..., None]
    return fields
