"""Scale-aware rigid jaw clearance, separate from lip translation and ranking."""
import numpy as np

from .faces import PreparationError
from .oral import dental_bite_stop


def calibrate_jaw(points, reference, upper_mask, lower_mask, mouth_width, settings):
    ratio=settings['minimum_clearance_mouth_width_ratio']
    if (settings['method']!='lip-aperture-delta' or not np.isfinite(ratio)
            or not 0<ratio<=0.1 or not np.isfinite(mouth_width) or mouth_width<=0):
        raise PreparationError('Invalid jaw calibration settings.')
    gap=ratio*mouth_width
    stop,profile=dental_bite_stop(upper_mask,lower_mask,gap)
    if stop>0:
        raise PreparationError('Requested clearance would change the reference pose; inspect calibration.')
    apertures={}
    for pose,landmarks in points.items():
        p=np.asarray(landmarks)
        if not np.isfinite(p[[13,14],:2]).all():raise PreparationError('Nonfinite jaw landmarks.')
        apertures[pose]=max(0.,float(p[14,1]-p[13,1]))
    origin=apertures[reference]
    if origin<=0:raise PreparationError('Jaw reference has no measurable lip aperture.')
    evidence={'method':'lip aperture delta with scale-aware per-column enamel clearance',
              'version':'jaw-clearance/1','reference_aperture_px':origin,
              'minimum_translation_y':stop,'minimum_gap_px':gap,'column_profile':profile,'poses':{}}
    translations={}
    for pose,aperture in apertures.items():
        proposed=int(round(aperture-origin));applied=max(stop,proposed)
        translations[pose]=[0,applied]
        evidence['poses'][pose]={'aperture_px':aperture,'aperture_delta_y':proposed,
                                'applied_offset_y':applied,'bite_stop_applied':applied!=proposed}
    return translations,evidence
