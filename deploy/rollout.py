"""Explicit apply-only orchestration. Never imported by the offline check path."""

import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
from urllib.parse import quote

ROOT = Path(__file__).resolve().parents[1]


def private_json(path, value):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(value))
    path.chmod(0o600)


def run(args, **kw):
    kw.setdefault("shell", os.name == "nt" and args[0].lower() == "az")
    return subprocess.run(args, cwd=ROOT, check=True, **kw)


def output(args):
    return subprocess.check_output(
        args,
        cwd=ROOT,
        text=True,
        shell=os.name == "nt" and args[0].lower() == "az",
    ).strip()


def validate_turn_configuration(config):
    services = config["services"]
    runtime = services["runtime"]
    turn_urls = (runtime.get("SYNTELLE_TURN_URLS") or "").strip()
    turn_secret = services["secrets"].get("SYNTELLE_TURN_SECRET", "")
    if turn_urls and len(turn_secret) < 32:
        raise SystemExit("TURN URLs require a signing secret of at least 32 characters.")
    if config.get("live_acceptance") and not turn_urls:
        raise SystemExit("Public live-media publication requires configured TURN URLs.")


def verify_modal_credentials():
    token_id = os.environ.get("MODAL_TOKEN_ID", "")
    token_secret = os.environ.get("MODAL_TOKEN_SECRET", "")
    if bool(token_id) != bool(token_secret):
        raise SystemExit("Provide both Modal token environment variables or use the active profile.")
    try:
        subprocess.run(
            [
                "uv",
                "run",
                "--with-requirements",
                "deploy/requirements.lock",
                "python",
                "-m",
                "modal",
                "token",
                "info",
            ],
            cwd=ROOT,
            check=True,
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
        )
    except (OSError, subprocess.CalledProcessError):
        raise SystemExit(
            "Modal authentication failed. Activate the intended Modal workspace profile or provide both token environment variables."
        ) from None


def apply_all(config_path):
    path = Path(config_path).resolve()
    config = json.loads(path.read_text())
    services = config["services"]
    modal = config["modal"]
    cloudflare_token = os.environ.get("CLOUDFLARE_API_TOKEN", "")
    if not cloudflare_token:
        raise SystemExit("Provide CLOUDFLARE_API_TOKEN before apply-all.")
    if len(config.get("foundation", {}).get("database_password", "")) < 32:
        raise SystemExit("Configure a strong foundation database password before apply-all.")
    from deploy.packaging import validate_bundle

    verify_modal_credentials()
    assets = Path(modal["assets"]).resolve()
    profile = Path(config["performance_profile"]).resolve()
    if not assets.is_dir() or not profile.is_file():
        raise SystemExit(
            "Provide the frozen native assets and matching performance profile."
        )
    if len(services["runtime"].get("SYNTELLE_PACKAGE_HASH", "")) != 64:
        raise SystemExit("Supply the verified package hash.")
    validate_bundle(assets, services["runtime"]["SYNTELLE_PACKAGE_HASH"])
    # Validate complete cloud inputs before creating even the foundation.
    for name in [
        "SYNTELLE_AUTH_SECRET",
        "SYNTELLE_EDGE_TOKEN",
        "SYNTELLE_PROCESSOR_TOKEN",
        "SYNTELLE_RENDER_WORKER_TOKEN",
        "SYNTELLE_MODAL_LAUNCH_TOKEN",
    ]:
        if len(services["secrets"].get(name, "")) < 32:
            raise SystemExit("Missing deployment secret: " + name)
    validate_turn_configuration(config)
    from deploy.cloudflare import zone_identity

    cloudflare_account, cloudflare_zone = zone_identity(cloudflare_token)
    configured_account = services.get("cloudflare_account_id", "")
    configured_zone = services.get("cloudflare_zone_id", "")
    if (
        configured_account
        and not configured_account.startswith("REPLACE")
        and configured_account != cloudflare_account
    ):
        raise SystemExit("The configured Cloudflare account does not own syntelle.online.")
    if (
        configured_zone
        and not configured_zone.startswith("REPLACE")
        and configured_zone != cloudflare_zone
    ):
        raise SystemExit("The configured Cloudflare zone ID does not match syntelle.online.")
    services["cloudflare_account_id"] = cloudflare_account
    services["cloudflare_zone_id"] = cloudflare_zone
    private_json(path, config)
    capabilities = output(
        [
            "az",
            "postgres",
            "flexible-server",
            "list-skus",
            "--location",
            config["foundation"].get("location", "canadacentral"),
            "--output",
            "json",
        ]
    )
    if "B1ms" not in capabilities or '"18"' not in capabilities:
        raise SystemExit(
            "Regional PostgreSQL 18/B1ms availability is not verified; no resources created."
        )
    services["publish_frontend"] = False
    private_json(path, config)
    run(
        [
            sys.executable,
            "deploy.py",
            "apply",
            "--stack",
            "foundation",
            "--config",
            str(path),
        ]
    )
    foundation = json.loads(
        output(
            ["terraform", "-chdir=infra/foundation", "output", "-json", "foundation"]
        )
    )
    password = output(
        ["terraform", "-chdir=infra/foundation", "output", "-raw", "registry_password"]
    )
    services["foundation"] = foundation
    services["secrets"]["registry"] = password
    services["secrets"]["SYNTELLE_DATABASE_URL"] = (
        "postgresql://syntelle:"
        + quote(config["foundation"]["database_password"], safe="")
        + "@"
        + foundation["database_fqdn"]
        + ":5432/syntelle?sslmode=verify-full"
    )
    run(
        [
            "docker",
            "login",
            foundation["registry"],
            "-u",
            foundation["registry_name"],
            "--password-stdin",
        ],
        input=password,
        text=True,
        stdout=subprocess.DEVNULL,
    )
    images = {}
    for name, dockerfile, context_path in [
        ("api", "services/api/Dockerfile", "."),
        ("processor", "services/processor/Dockerfile", "services/processor"),
        ("renderer", "services/render-worker/native.Dockerfile", "."),
    ]:
        tag = foundation["registry"] + "/syntelle-" + name + ":deployment"
        run(
            [
                "docker",
                "build",
                "--platform",
                "linux/amd64",
                "-f",
                dockerfile,
                "-t",
                tag,
                context_path,
            ]
        )
        run(["docker", "push", tag])
        images[name] = json.loads(
            output(
                ["docker", "image", "inspect", tag, "--format", "{{json .RepoDigests}}"]
            )
        )[0]
    # The API needs the matching performance profile but no native character textures.
    context = ROOT / ".deploy-private" / "api-profile"
    context.mkdir(parents=True, exist_ok=True)
    shutil.copyfile(profile, context / "performance.json")
    (context / "Dockerfile").write_text(
        "ARG BASE\nFROM ${BASE}\nCOPY --chmod=0444 performance.json /opt/syntelle/performance.json\n"
    )
    tag = foundation["registry"] + "/syntelle-api:deployment-profile"
    run(
        [
            "docker",
            "build",
            "--build-arg",
            "BASE=" + images["api"],
            "-t",
            tag,
            str(context),
        ]
    )
    run(["docker", "push", tag])
    images["api"] = json.loads(
        output(["docker", "image", "inspect", tag, "--format", "{{json .RepoDigests}}"])
    )[0]
    services["images"] = {key: images[key] for key in ["api", "processor"]}
    services["runtime"]["SYNTELLE_PERFORMANCE_PROFILE"] = (
        "/opt/syntelle/performance.json"
    )
    private_json(path, config)
    run(
        [
            sys.executable,
            "deploy.py",
            "apply",
            "--stack",
            "services",
            "--config",
            str(path),
        ]
    )
    origin = output(
        ["terraform", "-chdir=infra/services", "output", "-raw", "api_origin"]
    )
    config["modal"].update({"image": images["renderer"], "api_origin": origin})
    private_json(path, config)
    endpoint = output(
        [
            "uv",
            "run",
            "--with-requirements",
            "deploy/requirements.lock",
            "python",
            "-m",
            "deploy.modal_configure",
            str(path),
        ]
    )
    services["runtime"]["SYNTELLE_MODAL_LAUNCH_URL"] = endpoint
    # Activation of the public domain is a separate live-acceptance decision.
    services["publish_frontend"] = bool(config.get("live_acceptance", False))
    private_json(path, config)
    run(["npm", "--prefix", "apps/web", "run", "build"], shell=os.name == "nt")
    run(
        [
            sys.executable,
            "deploy.py",
            "apply",
            "--stack",
            "services",
            "--config",
            str(path),
        ]
    )
