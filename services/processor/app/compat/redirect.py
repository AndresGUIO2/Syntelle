"""Keep historical imports as the same module object, including monkeypatches."""
from importlib import import_module
import runpy
import sys


def redirect(name: str, target: str) -> None:
    if name == '__main__':
        runpy.run_module(target, run_name='__main__')
    else:
        sys.modules[name] = import_module(target)
