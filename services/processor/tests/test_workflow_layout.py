import importlib
from pathlib import Path
import subprocess
import sys

import pytest

pytest.importorskip('mediapipe')


@pytest.mark.parametrize(('legacy', 'canonical'), [
    ('app.viseme_trial', 'app.workflows.visemes.viseme_trial'),
    ('app.gemini_contour_loop', 'app.workflows.contours.gemini_contour_loop'),
    ('app.prepare_motion', 'app.workflows.motion.prepare_motion'),
    ('app.finalize_contextual_motion', 'app.audits.finalize_contextual_motion'),
])
def test_historical_imports_share_the_canonical_module_object(legacy, canonical):
    assert importlib.import_module(legacy) is importlib.import_module(canonical)


def test_local_paths_are_independent_of_command_nesting():
    from app.core.paths import ROOT, APP_DIR, PREPARATION_DIR, PROCESSOR_DIR
    from app.preparation import faces
    assert Path(faces.__file__).parent == PREPARATION_DIR
    assert APP_DIR == PROCESSOR_DIR/'app'
    if (ROOT/'compose.yaml').exists():
        assert ROOT/'services/processor' == PROCESSOR_DIR
    else:
        assert ROOT == PROCESSOR_DIR


def test_historical_cli_help_works_without_running_preparation():
    result = subprocess.run([sys.executable, '-m', 'app.prepare_contextual_motion', '--help'],
                            capture_output=True, text=True, timeout=15, check=False)
    assert result.returncode == 0, result.stderr
    assert '--input' in result.stdout and '--output' in result.stdout
