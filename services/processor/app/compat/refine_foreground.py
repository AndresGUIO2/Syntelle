"""Compatibility alias for app.workflows.foreground.refine_foreground."""
from app.compat.redirect import redirect

redirect(__name__, 'app.workflows.foreground.refine_foreground')
