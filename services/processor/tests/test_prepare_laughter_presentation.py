import json,os,pytest
from pathlib import Path
from app.core.paths import ROOT

def test_cached_laughter_has_one_gross_mandible_contribution():
    if not os.environ.get('SYNTELLE_LAUGHTER_PACKAGE'):pytest.skip('Explicit source-bound private package required.')
    package=Path(os.environ['SYNTELLE_LAUGHTER_PACKAGE'])
    manifest=json.loads((package/'manifest.json').read_bytes())
    reference=manifest['packages']['expressions']['reference_laughter']
    companion=reference.get('presentation')
    assert companion is not None,'Reference still bakes gross chin travel on top of the shared rig.'
    assert companion['version']=='laughter-presentation/2'
    assert companion['validation']['valid']
    assert companion['validation']['gross_chin_residual_px']<.1
    assert companion['validation']['expression_amplitude']==1
    assert companion['validation']['minimum_triangle_ratio']>0
    assert not companion['manual_masks']
