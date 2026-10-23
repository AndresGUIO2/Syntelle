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


def api_profile_dockerfile():
    return (
        "ARG BASE\n"
        "FROM ${BASE}\n"
        "USER root\n"
        "RUN mkdir -p /opt/syntelle && chmod 0555 /opt/syntelle\n"
        "COPY --chmod=0444 performance.json /opt/syntelle/performance.json\n"
        "USER syntelle\n"
    )


def verify_api_profile_image(image):
    run(
        [
            "docker",
            "run",
            "--rm",
            "--entrypoint",
            "/bin/sh",
            image,
            "-c",
            "test -x /opt/syntelle && test -r /opt/syntelle/performance.json",
        ]
    )


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
    stun_urls = (runtime.get("SYNTELLE_STUN_URLS") or "").strip()
    turn_secret = services["secrets"].get("SYNTELLE_TURN_SECRET", "")
    if turn_urls and len(turn_secret) < 32:
        raise SystemExit("TURN URLs require a signing secret of at least 32 characters.")
    if config.get("live_acceptance") and not (turn_urls or stun_urls):
        raise SystemExit("Live-media acceptance requires configured STUN or TURN URLs.")
    if services.get("publish_frontend") and not (turn_urls or stun_urls):
        raise SystemExit("Frontend publication requires configured STUN or TURN URLs.")


def validate_frontend_publication(config):
    services = config["services"]
    if not services.get("publish_frontend"):
        return
    runtime = services["runtime"]
    if not (runtime.get("SYNTELLE_MODAL_LAUNCH_URL") or "").strip():
        raise SystemExit("Frontend publication requires the deployed Modal launcher URL.")
    if not (
        (runtime.get("SYNTELLE_STUN_URLS") or "").strip()
        or (runtime.get("SYNTELLE_TURN_URLS") or "").strip()
    ):
        raise SystemExit("Frontend publication requires configured STUN or TURN URLs.")


def validate_release_selection(config, root=ROOT, allow_inactive_release=False):
    """Catch an old private export when this checkout selects a newer local package."""
    if allow_inactive_release:
        return
    dotenv = Path(root) / ".env"
    if not dotenv.is_file():
        return
    active_hash = ""
    for line in dotenv.read_text(encoding="utf-8-sig").splitlines():
        key, separator, value = line.partition("=")
        if separator and key.strip() == "SYNTELLE_PACKAGE_HASH":
            active_hash = value.strip().strip('"\'')
    selected_hash = config["services"]["runtime"].get("SYNTELLE_PACKAGE_HASH", "")
    if active_hash and selected_hash and active_hash != selected_hash:
        raise SystemExit(
            "Deployment selects package "
            + selected_hash[:12]
            + " while this checkout uses "
            + active_hash[:12]
            + ". Update the private release assets, profile and package hash before applying; "
            "use --allow-inactive-release only for an intentional rollback."
        )


def publish_frontend_during_service_update(requested, launch_url):
    """Keep an already wired public domain attached during a full redeploy."""
    return bool(requested and (launch_url or "").strip())


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


def apply_all(config_path, allow_inactive_release=False):
    path = Path(config_path).resolve()
    config = json.loads(path.read_text())
    validate_release_selection(config, allow_inactive_release=allow_inactive_release)
    services = config["services"]
    publish_frontend = bool(services.get("publish_frontend", False))
    modal = config["modal"]
    cloudflare_token = os.environ.get("CLOUDFLARE_API_TOKEN", "")
    if not cloudflare_token:
        raise SystemExit("Provide CLOUDFLARE_API_TOKEN before apply-all.")
    if len(config.get("foundation", {}).get("database_password", "")) < 32:
        raise SystemExit("Configure a strong foundation database password before apply-all.")
    from deploy.packaging import validate_bundle, validate_performance_profile

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
    try:
        validate_performance_profile(
            profile, services["runtime"]["SYNTELLE_PACKAGE_HASH"]
        )
    except (OSError, ValueError):
        raise SystemExit(
            "Supply a reviewed, owner-accepted performance profile matching the native package."
        ) from None
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
    services["publish_frontend"] = publish_frontend_during_service_update(
        publish_frontend, services["runtime"].get("SYNTELLE_MODAL_LAUNCH_URL")
    )
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
    (context / "Dockerfile").write_text(api_profile_dockerfile())
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
    verify_api_profile_image(tag)
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
    services["publish_frontend"] = publish_frontend
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
