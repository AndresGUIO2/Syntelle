from pathlib import Path
import tempfile, json, unittest
from deploy.packaging import validate_bundle


class PackagingTests(unittest.TestCase):
    def test_bound_identity_and_secret_exclusion(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / "manifest.json").write_text(
                json.dumps({"version": "native-character/1", "package_hash": "a" * 64})
            )
            self.assertIn("manifest.json", validate_bundle(root, "a" * 64))
            with self.assertRaises(ValueError):
                validate_bundle(root, "b" * 64)
            (root / ".env").write_text("fixture")
            with self.assertRaises(ValueError):
                validate_bundle(root, "a" * 64)
