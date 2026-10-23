"""Compatibility alias for app.workflows.speech.run_speech_trial."""
from app.compat.redirect import redirect

redirect(__name__, 'app.workflows.speech.run_speech_trial')
