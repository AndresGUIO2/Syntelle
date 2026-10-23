import numpy as np
import pytest
from app.preparation.faces import PreparationError
from app.preparation.reference_laughter import contact_shadow


def test_shadow_uses_each_characters_aperture_and_existing_lighting():
    for gap in (20.,60.,120.):
        inner=np.zeros((25,20,2));inner[-1,15,1]=gap
        result=contact_shadow({'oral':{'contact_shadow_strength':.3,'contact_shadow_width_px':2}},inner)
        assert result['strength']==.3
        assert result['width_px']==max(2,gap*.12)
        assert not inner[:-1].any()


@pytest.mark.parametrize('strength',[float('nan'),-.1,.8])
def test_bad_optional_lighting_is_rejected(strength):
    with pytest.raises(PreparationError,match='bounded'):
        contact_shadow({'oral':{'contact_shadow_strength':strength}},np.zeros((25,20,2)))
