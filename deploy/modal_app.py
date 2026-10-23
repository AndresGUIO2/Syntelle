"""Private Modal launcher; importing this module performs no deployment or GPU call."""

import hmac
import os
import subprocess
from pathlib import Path
import modal
from fastapi import Request
from fastapi.responses import JSONResponse

app = modal.App("syntelle-renderer")
# The exported rootfs includes the compiled Go worker and private frozen bundle.
image = (
    modal.Image.from_registry(
        os.environ["SYNTELLE_RENDER_IMAGE"],
        add_python="3.12",
        secret=modal.Secret.from_name("syntelle-registry"),
    )
    .entrypoint([])
    .add_local_dir(os.environ["SYNTELLE_RENDER_ASSETS"], remote_path="/assets")
)
runtime = modal.Secret.from_name("syntelle-render-runtime")
launch_auth = modal.Secret.from_name("syntelle-render-launch")


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


@app.function(
    secrets=[launch_auth],
    min_containers=0,
    max_containers=1,
    image=modal.Image.debian_slim().pip_install("fastapi==0.115.12"),
)
@modal.fastapi_endpoint(method="POST")
async def launch(request: Request):
    supplied = request.headers.get("Authorization", "")
    expected = "Bearer " + os.environ["SYNTELLE_MODAL_LAUNCH_TOKEN"]
    if not hmac.compare_digest(supplied, expected):
        return JSONResponse({"error": "unauthorized"}, status_code=403)
    data = await request.json()
    identifier = data.get("allocation_id", "")
    if not identifier.startswith("allocation_") or len(identifier) > 128:
        return JSONResponse({"error": "invalid allocation"}, status_code=400)
    await render.spawn.aio(identifier)
    return JSONResponse({"accepted": True}, status_code=202)
