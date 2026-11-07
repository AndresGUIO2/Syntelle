"""Build a native speaking portrait from its own image, landmarks and pose materials."""
from hashlib import sha256
import json
from pathlib import Path
import re
import subprocess

from app.core.paths import ROOT
from app.workflows.rendering.export_native import export, validate_bindings

REVISION = "character-creation-preparation/1"


class CreationError(Exception):
    """Allowlisted failure code suitable for account UI."""
    def __init__(self, code):
        super().__init__(code)
        self.code = code


def digest(data):
    return sha256(data).hexdigest()


def read(path):
    return json.loads(path.read_bytes())


def write(path, obj):
    path.write_text(json.dumps(obj, ensure_ascii=False, indent=2), encoding="utf-8")


def source_bundle(source_hash, catalog):
    if catalog.get("version") != "creation-source-cache/1":
        raise CreationError("invalid_cache")
    matches = [entry for entry in catalog["items"] if entry["source_sha256"] == source_hash]
    if not matches:
        return None
    if (len(matches) != 1 or matches[0]["bundle"].get("source_sha256") != source_hash or
            not re.fullmatch("[0-9a-f]{64}", matches[0]["package_hash"])):
        raise CreationError("invalid_cache")
    return matches[0]


def published_source(source_hash, saved):
    """Reuse only a complete package made by this preparation revision."""
    if saved is None:
        return None
    matches = []
    for path in saved.iterdir():
        if not re.fullmatch("[0-9a-f]{64}",path.name) or path.is_symlink() or not path.is_dir():
            continue
        marker = path / ".creation-managed.json"
        if not marker.is_file() or marker.is_symlink():
            continue
        record = read(marker)
        if record.get("source_sha256") == source_hash and record.get("revision") == REVISION:
            matches.append(validate_native(path, source_hash, path.name))
    if len(matches) > 1:
        raise CreationError("ambiguous_cache")
    return matches[0] if matches else None


def validate_native(package, source_hash, expected_hash=None):
    m = read(package / "manifest.json")
    identity = json.dumps(m["identity_files"], separators=(",", ":"), ensure_ascii=False).encode()
    package_hash = digest(identity)
    if (m["version"] != "native-character/1" or m["package_hash"] != package_hash or
            expected_hash is not None and expected_hash != package_hash):
        raise CreationError("invalid_package")
    validate_bindings(m["packages"], {"source_sha256": source_hash})
    parts = m["packages"]
    oral, motion = parts["oral"], parts["motion"]
    expected_poses = {"Rest", "A", "EI", "FV", "O", "U", "MBP", "SZ", "CHSHR"}
    required_pairs = {f"{a}-{b}" for a in expected_poses for b in expected_poses if a != b}
    if (set(oral["poses"]) != expected_poses or not required_pairs <= set(oral["pairs"]) or not oral["ready"] or
            not all(p.get("safe") and p.get("mesh") for p in oral["pairs"].values()) or
            not motion["projection"]["ready"] or not motion["projection"]["validation"]["valid"] or
            not all(parts[name]["validation"]["valid"] for name in ("expressions", "gaze"))):
        raise CreationError("geometry_failed")
    for asset in [*m["textures"].values(), m["geometry"]]:
        path = asset["file"]
        if not re.fullmatch(r"data/[0-9a-f]{64}\.(rgba|bin)", path):
            raise CreationError("invalid_package")
        target = package / path
        if not target.resolve().is_relative_to(package.resolve()) or digest(target.read_bytes()) != asset["sha256"]:
            raise CreationError("invalid_package")
    return package_hash


def render_check(executable, package, work):
    """Use the same native loader, oral compositor and head rig as the player."""
    from PIL import Image
    states = [{"name": pose, "state": {"from": "Rest", "to": pose, "amount": amount}}
              for pose, amount in (("Rest", 0), ("A", 1), ("FV", 1), ("MBP", 1))]
    # Rest -> Rest is not a morph pair. Keep the neutral endpoint of Rest -> A.
    states[0]["state"]["to"] = "A"
    states_path = work / "states.json"
    write(states_path, states)
    out = work / "native-check"
    import os
    env = {**os.environ, "SYNTELLE_RENDERER": "cpu"}
    try:
        result = subprocess.run([str(executable.resolve()), "-package", str(package), "-states",
                                 str(states_path), "-output", str(out), "-height", "720", "-packed"],
                                stdin=subprocess.DEVNULL, capture_output=True, timeout=120, env=env)
    except (OSError, subprocess.TimeoutExpired):
        raise CreationError("native_checker_unavailable") from None
    if result.returncode:
        raise CreationError("native_render_failed")
    frames = [digest(Image.open(out / f"{row['name']}.png").convert("RGB").tobytes()) for row in states]
    if len(set(frames)) < 3 or frames[0] == frames[1]:
        raise CreationError("articulation_failed")


def build(source, source_hash, work, package, checker, *, approval=None, providers=None,
          catalog=None, saved=None):
    if digest(source) != source_hash:
        raise CreationError("source_mismatch")
    cache = source_bundle(source_hash, read(ROOT / "docs/characters/creation-source-cache.json") if catalog is None else catalog)
    published = published_source(source_hash, saved) if cache is None else None
    if cache is None and published is None and approval is None:
        raise CreationError("generation_not_enabled")
    if not checker.is_file():
        raise CreationError("native_checker_unavailable")
    if published is not None:
        render_check(checker, saved / published, work)
        return published
    expected = None
    if cache:
        expected = cache["package_hash"]
        if saved is not None and (saved / expected).is_dir():
            validate_native(saved / expected, source_hash, expected)
            render_check(checker, saved / expected, work)
            return expected
        bundle = cache["bundle"]
    else:
        from .novel import build_bundle
        bundle = build_bundle(source, work, approval, providers)
    spec = work / "bundle.json"
    write(spec, bundle)
    export(package, bundle=spec)
    package_hash = validate_native(package, source_hash, expected)
    render_check(checker, package, work)
    return package_hash
