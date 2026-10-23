import pytest
from app.preparation.faces import PreparationError
from app.workflows.motion.prepare_gesture_envelope import prepare, rebind_motion


def test_rebinding_covers_optional_expressions_without_changing_materials():
    package = {'motion_build_id': 'old', 'smile': {'material': 'original.webp'},
               'reference_laughter': {'motion_build_id': 'old', 'frames': ['cached.webp']}}
    rebind_motion(package, 'old', 'new')
    assert package['motion_build_id'] == package['reference_laughter']['motion_build_id'] == 'new'
    assert package['smile'] == {'material': 'original.webp'}
    assert package['reference_laughter']['frames'] == ['cached.webp']
    with pytest.raises(PreparationError, match='different motion'):
        rebind_motion(package, 'wrong', 'another')


def test_existing_evidence_and_unbounded_pitch_are_rejected(tmp_path):
    with pytest.raises(PreparationError, match='fresh output'):
        prepare(tmp_path/'missing-bundle', tmp_path)
    for pitch in (float('nan'), float('inf'), -1, 8):
        with pytest.raises(PreparationError, match='bounded'):
            prepare(tmp_path/'missing-bundle', tmp_path/'fresh', pitch)
    for values in ({'yaw':float('nan')},{'yaw':6},{'roll':4},{'roll':0}):
        with pytest.raises(PreparationError, match='bounded'):
            prepare(tmp_path/'missing-bundle', tmp_path/'fresh', **values)
