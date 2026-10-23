"""Invoked only by explicit apply-all. Creates secrets/deploys; never calls render."""

import json
import os
from pathlib import Path
import subprocess
import sys


def runtime_secret_values(settings, secrets, runtime):
    values = {
        "SYNTELLE_API_ORIGIN": settings["api_origin"],
        "SYNTELLE_RENDER_API_URL": settings["api_origin"],
        "SYNTELLE_RENDER_WORKER_TOKEN": secrets["SYNTELLE_RENDER_WORKER_TOKEN"],
    }
    turn_urls = (runtime.get("SYNTELLE_TURN_URLS") or "").strip()
    if turn_urls:
        values["SYNTELLE_TURN_URLS"] = turn_urls
        values["SYNTELLE_TURN_SECRET"] = secrets["SYNTELLE_TURN_SECRET"]
    stun_urls = (runtime.get("SYNTELLE_STUN_URLS") or "").strip()
    if stun_urls:
        values["SYNTELLE_STUN_URLS"] = stun_urls
    return values


def main():
    config = json.loads(Path(sys.argv[1]).read_text())
    settings = config["modal"]
    secrets = config["services"]["secrets"]
    runtime = config["services"]["runtime"]
    # Authentication is supplied through MODAL_TOKEN_ID / MODAL_TOKEN_SECRET.
    import modal

    def upsert(name, values, allow_existing=True):
        modal.Secret.objects.create(name, values, allow_existing=True)
        modal.Secret.from_name(name).update(values)

    username = settings["image"].split("/")[0].split(".")[0]
    upsert(
        "syntelle-registry",
        {"REGISTRY_USERNAME": username, "REGISTRY_PASSWORD": secrets["registry"]},
    )
    upsert(
        "syntelle-render-launch",
        {"SYNTELLE_MODAL_LAUNCH_TOKEN": secrets["SYNTELLE_MODAL_LAUNCH_TOKEN"]},
    )
    upsert(
        "syntelle-render-runtime",
        runtime_secret_values(settings, secrets, runtime),
        allow_existing=True,
    )
    environment = dict(
        os.environ,
        SYNTELLE_RENDER_IMAGE=settings["image"],
        SYNTELLE_RENDER_ASSETS=str(Path(settings["assets"]).resolve()),
    )
    subprocess.run(
        [sys.executable, "-m", "modal", "deploy", "deploy/modal_app.py"],
        env=environment,
        check=True,
        stdout=sys.stderr,
    )
    endpoint = modal.Function.from_name("syntelle-renderer", "launch").get_web_url()
    if not endpoint:
        raise RuntimeError("Modal returned no launcher URL")
    print(endpoint)


if __name__ == "__main__":
    main()
