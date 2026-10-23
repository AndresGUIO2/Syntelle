"""Compatibility alias for app.workflows.motion.prepare_motion."""
from app.compat.redirect import redirect

redirect(__name__, 'app.workflows.motion.prepare_motion')
