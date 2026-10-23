"""Compatibility alias for app.audits.finalize_head_turns."""
from app.compat.redirect import redirect

redirect(__name__, 'app.audits.finalize_head_turns')
