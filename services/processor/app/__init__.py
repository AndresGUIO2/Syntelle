"""HTTP application and grouped local workflows.

Historical ``app.<command>`` module names are resolved from the compatibility
directory. New code should import the explicit workflows/audits packages.
"""
from pathlib import Path

__path__.append(str(Path(__file__).with_name('compat')))
