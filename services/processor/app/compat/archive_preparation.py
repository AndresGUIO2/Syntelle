"""Compatibility alias for app.audits.archive_preparation."""
from app.compat.redirect import redirect

redirect(__name__, 'app.audits.archive_preparation')
