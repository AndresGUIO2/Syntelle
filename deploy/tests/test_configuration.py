import ast
import os
from pathlib import Path
import subprocess
import unittest
from unittest.mock import patch

ROOT = Path(__file__).resolve().parents[2]

from deploy.modal_configure import runtime_secret_values
from deploy.rollout import validate_turn_configuration, verify_modal_credentials


class DeploymentTests(unittest.TestCase):
    def test_no_gpu_on_import(self):
        source = (ROOT / "deploy/modal_app.py").read_text()
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

    def test_default_has_no_apply(self):
        text = (ROOT / "deploy.py").read_text()
        self.assertIn('default="check"', text)

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

    def test_turn_pair_is_required_for_publication(self):
        no_turn = {
            "services": {"runtime": {"SYNTELLE_TURN_URLS": ""}, "secrets": {}},
            "live_acceptance": True,
        }
        with self.assertRaisesRegex(SystemExit, "publication requires"):
            validate_turn_configuration(no_turn)

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
