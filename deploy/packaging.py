"""Verify a private native export before any image/asset publication."""

import hashlib
import json
from pathlib import Path


def validate_bundle(directory, expected_hash):
    root = Path(directory).resolve()
    manifest = json.loads((root / "manifest.json").read_text())
    if (
        manifest.get("version") != "native-character/1"
        or manifest.get("package_hash") != expected_hash
    ):
        raise ValueError("Native export identity does not match the API.")
    files = {}
    for path in root.rglob("*"):
        if path.is_symlink():
            raise ValueError("Symlinks are not permitted in deployment assets.")
        if not path.is_file():
            continue
        if path.name.startswith(".env") or any(
            part in {".git", ".ssh", ".azure", "node_modules"}
            for part in path.relative_to(root).parts
        ):
            raise ValueError(
                "Private credentials/source tree cannot be packaged as assets."
            )
        files[path.relative_to(root).as_posix()] = hashlib.sha256(
            path.read_bytes()
        ).hexdigest()
    return files
