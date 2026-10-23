import ast
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import patch

ROOT = Path(__file__).resolve().parents[2]

from deploy.modal_configure import runtime_secret_values
from deploy.packaging import validate_performance_profile
from deploy.rollout import (
    api_profile_dockerfile,
    publish_frontend_during_service_update,
    validate_frontend_publication,
    validate_release_selection,
    validate_turn_configuration,
    verify_modal_credentials,
)
from deploy.terraform import command as terraform_command


class DeploymentTests(unittest.TestCase):
    def test_full_redeploy_preserves_existing_public_domain(self):
        self.assertTrue(
            publish_frontend_during_service_update(
                True, "https://syntelle-launch.example.modal.run"
            )
        )
        self.assertFalse(publish_frontend_during_service_update(True, ""))
        self.assertFalse(
            publish_frontend_during_service_update(
                False, "https://syntelle-launch.example.modal.run"
            )
        )

    def test_apply_rejects_stale_export_before_provider_calls(self):
        config = {"services": {"runtime": {"SYNTELLE_PACKAGE_HASH": "a" * 64}}}
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / ".env").write_text("SYNTELLE_PACKAGE_HASH=" + "b" * 64 + "\n")
            with self.assertRaisesRegex(SystemExit, "Deployment selects package"):
                validate_release_selection(config, root)
            validate_release_selection(config, root, allow_inactive_release=True)
            config["services"]["runtime"]["SYNTELLE_PACKAGE_HASH"] = "b" * 64
            validate_release_selection(config, root)

    def test_api_profile_layer_keeps_nonroot_path_traversable(self):
        dockerfile = api_profile_dockerfile()
        self.assertIn("USER root", dockerfile)
        self.assertIn("chmod 0555 /opt/syntelle", dockerfile)
        self.assertIn("COPY --chmod=0444 performance.json", dockerfile)
        self.assertTrue(dockerfile.rstrip().endswith("USER syntelle"))

    def test_deployment_profile_requires_matching_owner_acceptance(self):
        package_hash = "a" * 64
        profile = {
            "version": "character-performance/1",
            "package_hash": package_hash,
            "reviewed": True,
            "owner_acceptance": False,
        }
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "performance.json"
            path.write_text(json.dumps(profile))
            with self.assertRaisesRegex(ValueError, "owner-accepted"):
                validate_performance_profile(path, package_hash)

            profile["owner_acceptance"] = True
            path.write_text(json.dumps(profile))
            self.assertEqual(
                validate_performance_profile(path, package_hash)["package_hash"],
                package_hash,
            )

            with self.assertRaisesRegex(ValueError, "owner-accepted"):
                validate_performance_profile(path, "b" * 64)

    def test_no_gpu_on_import(self):
        source = (ROOT / "deploy/modal_render.py").read_text()
        tree = ast.parse(source)
        self.assertIn('gpu="T4"', source)
        self.assertIn("min_containers=0", source)
        self.assertIn("max_containers=1", source)
        self.assertIn("retries=0", source)
        self.assertFalse(
            any(
                isinstance(x, ast.Expr) and isinstance(x.value, ast.Call)
                for x in tree.body
            )
        )

    def test_express_and_private_database(self):
        foundation = (ROOT / "infra/foundation/main.tf").read_text()
        self.assertIn('environmentMode = "Express"', foundation)
        self.assertIn("public_network_access_enabled = false", foundation)
        self.assertIn("prevent_destroy = true", foundation)
        service = (ROOT / "infra/services/main.tf").read_text()
        self.assertRegex(service, r"maxReplicas\s*=\s*1")
        self.assertRegex(service, r'SYNTELLE_LOCAL_OWNER\s*=\s*"0"')
        self.assertIn('transport = "Http"', service)

    def test_default_has_no_apply(self):
        text = (ROOT / "deploy.py").read_text()
        self.assertIn('default="check"', text)

    def test_services_terraform_commands_are_serialized(self):
        command = terraform_command("services", "apply", "private.tfvars.json")
        self.assertIn("-parallelism=1", command)
        self.assertIn("-auto-approve", command)
        foundation = terraform_command("foundation", "plan", "private.tfvars.json")
        self.assertNotIn("-parallelism=1", foundation)
        self.assertNotIn("-auto-approve", foundation)

    def test_deferred_turn_does_not_block_private_deployment(self):
        config = {
            "services": {"runtime": {"SYNTELLE_TURN_URLS": ""}, "secrets": {}},
            "live_acceptance": False,
        }
        validate_turn_configuration(config)

        runtime = runtime_secret_values(
            {"api_origin": "https://api.fixture.azurecontainerapps.io"},
            {"SYNTELLE_RENDER_WORKER_TOKEN": "worker-token"},
            {"SYNTELLE_TURN_URLS": "", "SYNTELLE_STUN_URLS": ""},
        )
        self.assertNotIn("SYNTELLE_TURN_SECRET", runtime)
        self.assertNotIn("SYNTELLE_TURN_URLS", runtime)

    def test_stun_only_frontend_publication_does_not_require_turn(self):
        stun_only = {
            "services": {
                "publish_frontend": True,
                "runtime": {
                    "SYNTELLE_MODAL_LAUNCH_URL": "https://render.fixture.modal.run",
                    "SYNTELLE_STUN_URLS": "stun:stun.cloudflare.com:3478",
                    "SYNTELLE_TURN_URLS": "",
                },
                "secrets": {},
            },
            "live_acceptance": False,
        }
        validate_turn_configuration(stun_only)
        validate_frontend_publication(stun_only)

        no_ice = {
            "services": {
                "publish_frontend": True,
                "runtime": {"SYNTELLE_MODAL_LAUNCH_URL": "https://render.fixture.modal.run"},
                "secrets": {},
            },
            "live_acceptance": False,
        }
        with self.assertRaisesRegex(SystemExit, "STUN or TURN"):
            validate_turn_configuration(no_ice)

        partial_turn = {
            "services": {
                "runtime": {"SYNTELLE_TURN_URLS": "turns:relay.example:443"},
                "secrets": {},
            },
            "live_acceptance": False,
        }
        with self.assertRaisesRegex(SystemExit, "TURN URLs require"):
            validate_turn_configuration(partial_turn)

        configured_turn = {
            "services": {
                "runtime": {"SYNTELLE_TURN_URLS": "turns:relay.example:443"},
                "secrets": {"SYNTELLE_TURN_SECRET": "x" * 32},
            },
            "live_acceptance": True,
        }
        validate_turn_configuration(configured_turn)

        no_launcher = {
            "services": {
                "publish_frontend": True,
                "runtime": {"SYNTELLE_STUN_URLS": "stun:stun.cloudflare.com:3478"},
                "secrets": {},
            },
            "live_acceptance": False,
        }
        with self.assertRaisesRegex(SystemExit, "Modal launcher"):
            validate_frontend_publication(no_launcher)

        runtime = runtime_secret_values(
            {"api_origin": "https://api.fixture.azurecontainerapps.io"},
            {
                "SYNTELLE_RENDER_WORKER_TOKEN": "worker-token",
                "SYNTELLE_TURN_SECRET": "turn-signing-secret",
            },
            {
                "SYNTELLE_TURN_URLS": "turns:relay.example:443",
                "SYNTELLE_STUN_URLS": "stun:stun.example:3478",
            },
        )
        self.assertEqual(runtime["SYNTELLE_TURN_SECRET"], "turn-signing-secret")
        self.assertEqual(runtime["SYNTELLE_TURN_URLS"], "turns:relay.example:443")
        self.assertEqual(runtime["SYNTELLE_STUN_URLS"], "stun:stun.example:3478")

    @patch("deploy.rollout.subprocess.run")
    def test_modal_auth_uses_active_profile_without_printing_credentials(self, run):
        with patch.dict(
            os.environ, {"MODAL_TOKEN_ID": "", "MODAL_TOKEN_SECRET": ""}
        ):
            verify_modal_credentials()

        args, kwargs = run.call_args
        self.assertEqual(
            args[0],
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
        )
        self.assertEqual(kwargs["stdout"], subprocess.DEVNULL)
        self.assertEqual(kwargs["stderr"], subprocess.DEVNULL)
        self.assertNotIn("profile-token", str(args))

    @patch("deploy.rollout.subprocess.run")
    def test_modal_auth_rejects_partial_environment_pair(self, run):
        with patch.dict(
            os.environ,
            {"MODAL_TOKEN_ID": "profile-token", "MODAL_TOKEN_SECRET": ""},
        ):
            with self.assertRaisesRegex(SystemExit, "both Modal token"):
                verify_modal_credentials()
        run.assert_not_called()
