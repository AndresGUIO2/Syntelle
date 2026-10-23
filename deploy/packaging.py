"""Verify a private native export before any image/asset publication."""

import hashlib
import json
from pathlib import Path


def validate_bundle(directory, expected_hash):
    root = Path(directory).resolve()
    manifest = json.loads((root / "manifest.json").read_text(encoding="utf-8"))
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


def validate_performance_profile(path, expected_hash):
    profile = json.loads(Path(path).read_text(encoding="utf-8"))
    if not isinstance(profile, dict):
        raise ValueError("Performance profile must be a JSON object.")
    if (
        profile.get("version") != "character-performance/1"
        or profile.get("package_hash") != expected_hash
        or profile.get("reviewed") is not True
        or profile.get("owner_acceptance") is not True
    ):
        raise ValueError(
            "Deployment requires a reviewed, owner-accepted performance profile for the exact native package."
        )
    return profile
