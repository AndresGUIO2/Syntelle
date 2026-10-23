"""Compatibility alias for app.workflows.corpus.corpus_prepare."""
from app.compat.redirect import redirect

redirect(__name__, 'app.workflows.corpus.corpus_prepare')
