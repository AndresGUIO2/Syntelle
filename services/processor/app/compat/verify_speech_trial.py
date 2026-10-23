"""Compatibility alias for app.workflows.speech.verify_speech_trial."""
from app.compat.redirect import redirect

redirect(__name__, 'app.workflows.speech.verify_speech_trial')
