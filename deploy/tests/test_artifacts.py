import hashlib
from pathlib import Path
import tempfile
import unittest
import zipfile

from deploy.artifacts import extract_bundle


class ArtifactTests(unittest.TestCase):
    def test_verified_private_export_and_hash_rejection(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            archive = root / "fixture.zip"
            with zipfile.ZipFile(archive, "w") as bundle:
                bundle.writestr("assets/manifest.json", '{"fixture":true}')
                bundle.writestr("performance.json", "{}")
            digest = hashlib.sha256(archive.read_bytes()).hexdigest()
            with self.assertRaisesRegex(ValueError, "hash mismatch"):
                extract_bundle(archive, root / "bad", "0" * 64)
            self.assertFalse((root / "bad").exists())
            extract_bundle(archive, root / "release", digest)
            self.assertTrue((root / "release/assets/manifest.json").is_file())

    def test_rejects_archive_escape_before_writing(self):
        for name in ["../outside", "/absolute", "C:/outside", "folder\\outside"]:
            with self.subTest(name=name), tempfile.TemporaryDirectory() as directory:
                root = Path(directory)
                archive = root / "fixture.zip"
                with zipfile.ZipFile(archive, "w") as bundle:
                    entry = zipfile.ZipInfo()
                    entry.filename = name
                    bundle.writestr(entry, "unsafe")
                digest = hashlib.sha256(archive.read_bytes()).hexdigest()
                with self.assertRaisesRegex(ValueError, "Unsafe"):
                    extract_bundle(archive, root / "release", digest)
                self.assertFalse((root / "release").exists())
