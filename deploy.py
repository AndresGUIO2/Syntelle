"""Deployment entry point. Default checks never log in, apply, push or call GPUs."""

import argparse
import json
import os
from pathlib import Path
import subprocess
import sys
import uuid

from deploy.terraform import command as terraform_command

ROOT = Path(__file__).resolve().parent


def run(args, **kwargs):
    kwargs.setdefault("shell", os.name == "nt" and args[0].lower() == "az")
    subprocess.run(args, cwd=ROOT, check=True, **kwargs)


def select_azure_subscription(config):
    azure = config.get("azure", {})
    subscription_id = azure.get("subscription_id", "").strip()
    tenant_id = azure.get("tenant_id", "").strip()
    try:
        uuid.UUID(subscription_id)
        uuid.UUID(tenant_id)
    except ValueError:
        raise SystemExit(
            "Set valid azure.subscription_id and azure.tenant_id UUIDs in the private config."
        ) from None
    try:
        account = json.loads(
            subprocess.check_output(
                ["az", "account", "show", "--output", "json"],
                cwd=ROOT,
                text=True,
                stderr=subprocess.DEVNULL,
                shell=os.name == "nt",
            )
        )
    except (OSError, subprocess.CalledProcessError, json.JSONDecodeError):
        raise SystemExit("Sign in with Azure CLI before planning or applying.") from None
    if account.get("id") != subscription_id:
        run(["az", "account", "set", "--subscription", subscription_id])
        try:
            account = json.loads(
                subprocess.check_output(
                    ["az", "account", "show", "--output", "json"],
                    cwd=ROOT,
                    text=True,
                    stderr=subprocess.DEVNULL,
                    shell=os.name == "nt",
                )
            )
        except (OSError, subprocess.CalledProcessError, json.JSONDecodeError):
            raise SystemExit("Azure CLI could not select the configured subscription.") from None
    if account.get("id") != subscription_id or account.get("tenantId") != tenant_id:
        raise SystemExit("Azure CLI is signed in to a different subscription or tenant.")


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument(
        "command", choices=["check", "plan", "apply"], nargs="?", default="check"
    )
    parser.add_argument(
        "--stack",
        choices=["bootstrap", "foundation", "services", "all"],
        default="foundation",
    )
    parser.add_argument("--config", default=".deploy-private/config.json")
    parser.add_argument("--allow-inactive-release", action="store_true")
    options = parser.parse_args()
    if options.allow_inactive_release and (options.command, options.stack) != ("apply", "all"):
        raise SystemExit("--allow-inactive-release is only valid for apply --stack all.")
    if options.stack == "all" and options.command == "plan":
        raise SystemExit(
            "Plan foundation and services separately; apply-all wires computed outputs."
        )
    if options.command == "check":
        run([sys.executable, "-m", "unittest", "discover", "-s", "deploy/tests"])
        run(["node", "--test", "deploy/edge/worker.test.mjs", "deploy/edge/upgrade.test.mjs"])
        run(["terraform", "fmt", "-check", "-recursive", "infra"])
        for stack in ["bootstrap", "foundation", "services"]:
            run(
                [
                    "terraform",
                    f"-chdir=infra/{stack}",
                    "init",
                    "-backend=false",
                    "-input=false",
                ]
            )
            run(["terraform", f"-chdir=infra/{stack}", "validate"])
            if stack != "bootstrap":
                run(["terraform", f"-chdir=infra/{stack}", "test"])
        return
    config = json.loads((ROOT / options.config).read_text())
    if options.command == "apply" and options.stack == "all":
        from deploy.rollout import validate_release_selection

        validate_release_selection(config, allow_inactive_release=options.allow_inactive_release)
    select_azure_subscription(config)
    if options.command == "apply" and options.stack == "services":
        run(["npm", "--prefix", "apps/web", "run", "build"], shell=os.name == "nt")
        from deploy.rollout import validate_frontend_publication

        validate_frontend_publication(config)
    if options.stack == "all" and options.command == "apply":
        from deploy.rollout import apply_all

        apply_all(ROOT / options.config, allow_inactive_release=options.allow_inactive_release)
        return
    values = config[options.stack]
    private = ROOT / ".deploy-private"
    private.mkdir(exist_ok=True)
    private.chmod(0o700)
    variables = private / f"{options.stack}.tfvars.json"
    variables.write_text(json.dumps(values))
    variables.chmod(0o600)
    if options.stack == "bootstrap":
        state_path = (private / "bootstrap.tfstate").resolve()
        run(
            [
                "terraform",
                "-chdir=infra/bootstrap",
                "init",
                "-reconfigure",
                "-input=false",
                f"-backend-config=path={state_path}",
            ]
        )
        run(
            [
                "terraform",
                "-chdir=infra/bootstrap",
                options.command,
                "-input=false",
                *(["-auto-approve"] if options.command == "apply" else []),
                f"-var-file={variables.resolve()}",
            ]
        )
        return
    backend = dict(config["backend"], key=f"{options.stack}.tfstate")
    backend_file = private / "backend.json"
    backend_file.write_text(json.dumps(backend))
    backend_file.chmod(0o600)
    run(
        [
            "terraform",
            f"-chdir=infra/{options.stack}",
            "init",
            "-input=false",
            f"-backend-config={backend_file.resolve()}",
        ]
    )
    run(terraform_command(options.stack, options.command, variables.resolve()))


if __name__ == "__main__":
    main()
