"""GPU renderer function; this module intentionally has no FastAPI dependency."""

import os
import subprocess

import modal

from deploy.modal_shared import app

# Modal imports function modules on both the deploy machine and remote
# containers. The registry image and local asset mount are deploy-time inputs;
# remote imports need only a placeholder definition to load the function code.
if modal.is_local():
    image = (
        modal.Image.from_registry(
            os.environ["SYNTELLE_RENDER_IMAGE"],
            add_python="3.12",
            secret=modal.Secret.from_name("syntelle-registry"),
        )
        .entrypoint([])
        .add_local_dir(os.environ["SYNTELLE_RENDER_ASSETS"], remote_path="/assets")
    )
else:
    image = modal.Image.debian_slim().entrypoint([])

runtime = modal.Secret.from_name("syntelle-render-runtime")


@app.function(
    image=image,
    gpu="T4",
    cpu=2,
    memory=2048,
    region="us-east",
    min_containers=0,
    buffer_containers=0,
    max_containers=1,
    scaledown_window=2,
    timeout=1800,
    retries=0,
    secrets=[runtime],
)
def render(allocation_id: str):
    environment = dict(
        os.environ,
        SYNTELLE_DEPLOYMENT_MODE="cloud",
        SYNTELLE_RENDER_ALLOCATION_ID=allocation_id,
        SYNTELLE_RENDERER="gpu",
        SYNTELLE_VIDEO_ENCODER="nvenc",
    )
    subprocess.run(
        [
            "/usr/local/bin/syntelle-render-worker",
            "--assets",
            "/assets",
            "--capacity",
            "2",
        ],
        env=environment,
        check=True,
        timeout=1750,
    )
