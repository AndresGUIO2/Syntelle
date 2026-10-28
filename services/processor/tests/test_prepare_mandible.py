import json
from pathlib import Path
import pytest
import numpy as np
from app.core.paths import ROOT
from app.preparation.faces import PreparationError
from app.workflows.motion import prepare_mandible as pipeline

SOURCE=ROOT/'.tools/paired-parallax-100/prepared-v2/package'
LANDMARKS=ROOT/'.tools/foreground-11/prepared/syntelle/base-landmarks.json'

@pytest.mark.skipif(not SOURCE.exists(),reason="private cached preparation fixture")
def test_optional_rig_preserves_source_and_joint_bounds(tmp_path,monkeypatch):
    monkeypatch.setattr(pipeline,'ROOT',tmp_path)
    report=pipeline.prepare(SOURCE,LANDMARKS,tmp_path/'candidate')
    assert report['provider_requests']==0
    assert report['validation']['minimum_area_ratio']>.25
    assert report['validation']['chin_travel_720_px']>=3
    before=json.loads((SOURCE/'manifest.json').read_bytes())
    after=json.loads((tmp_path/'candidate/package/manifest.json').read_bytes())
    assert before['textures']==after['textures']
    assert before['geometry']==after['geometry']
    rig=after['packages']['motion']['mandible']
    assert rig['corner_travel_px']>=0 and rig['pose_jaw']['MBP']==0
    assert not rig['provenance']['manual_masks']
    geometry=after['packages']['motion']['projection']['geometry']
    vertices=np.frombuffer((tmp_path/'candidate/package'/after['geometry']['file']).read_bytes()[:geometry['vertices']*20],'<f4').reshape(-1,5)
    canvas=after['packages']['motion']['canvas']
    xy=vertices[:,:2]*[canvas['width'],canvas['height']]
    points=np.asarray(json.loads(LANDMARKS.read_bytes())['landmarks_px'])
    face_height=points[152,1]-points[10,1]
    # Articulation may stretch the small adjoining under-chin transition, but
    # must leave the lower neck and its clothing attachment exactly stationary.
    lower_neck=xy[:,1]>=points[152,1]+face_height*.20
    assert np.max(np.asarray(rig['weights'])[lower_neck])==0
    with pytest.raises(PreparationError,match='Preserve earlier'):
        pipeline.prepare(SOURCE,LANDMARKS,tmp_path/'candidate')

@pytest.mark.skipif(not SOURCE.exists(),reason="private cached preparation fixture")
def test_rejects_other_source_anatomy(tmp_path):
    points=json.loads(LANDMARKS.read_bytes());points['landmarks_px'][152][1]+=3
    bad=tmp_path/'wrong.json';bad.write_text(json.dumps(points))
    with pytest.raises(PreparationError,match='another source'):
        pipeline.prepare(SOURCE,bad,tmp_path/'candidate')
    assert not (tmp_path/'candidate').exists()
