"""Modal deployment entry point; runtime handlers live in separate modules."""

from deploy.modal_launcher import launch
from deploy.modal_render import render
from deploy.modal_shared import app
