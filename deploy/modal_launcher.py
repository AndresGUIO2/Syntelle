"""Authenticated, CPU-only Modal control endpoint for renderer allocations."""

import hmac
import os

import modal
from fastapi import Request
from fastapi.responses import JSONResponse

from deploy.modal_shared import app

launch_auth = modal.Secret.from_name("syntelle-render-launch")


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

    renderer = modal.Function.from_name("syntelle-renderer", "render")
    await renderer.spawn.aio(identifier)
    return JSONResponse({"accepted": True}, status_code=202)
