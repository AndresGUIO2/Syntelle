"""Archive the local fast-turn and intentional-motion candidate; verify restored bytes."""

import argparse
from hashlib import sha256
import json
from pathlib import Path
import shutil

from app.core.paths import ROOT
from app.preparation.archive import Archive


NAME = "fast-intentional-122"
DOCS = (
    "docs/experiments/motion/fast-turns-intentional-motion.md",
    *(f"specs/{area}/{file}.md"
      for area in ("004-coordinated-performance", "006-conversation-latency")
      for file in ("spec", "plan", "tasks", "verify")),
)


def read(path: Path):
    return json.loads(path.read_bytes())


def inventory(directory: Path):
    result = {}
    for path in directory.rglob("*"):
        if path.is_file() and "__pycache__" not in path.parts and path.suffix not in (".pyc", ".tmp"):
            result[path.relative_to(directory).as_posix()] = sha256(path.read_bytes()).hexdigest()
    return result


def copy_sources(folder: Path, paths):
    for name in paths:
        target = folder / name
        target.parent.mkdir(parents=True, exist_ok=True)
        shutil.copyfile(ROOT / name, target)


def qualify(folder: Path):
    summary = read(folder / "validation-summary.json")
    install = read(folder / "installation.json")
    prep = read(folder / "intentional-preparation.json")
    before = ROOT / ".tools/expressive-articulation-121"
    manifest = read(folder / "package/manifest.json")
    baseline = read(before / "package/manifest.json")
    assert install["package_hash"] == prep["package_hash"] == manifest["package_hash"]
    assert baseline["package_hash"] == prep["source_package_hash"]
    assert summary["provider_calls"] == 0 and summary["owner_naturalness"] == "pending"
    for item in prep["unchanged_files"]:
        rel = Path(item["file"])
        assert sha256((folder / "package" / rel).read_bytes()).hexdigest() == item["sha256"]
        assert sha256((before / "package" / rel).read_bytes()).hexdigest() == item["sha256"]
    # A selector and its package identity are the complete manifest delta.
    for key in ("package_hash", "identity_files"):
        manifest.pop(key)
        baseline.pop(key)
    manifest["packages"]["motion"]["conversational_motion"]["presence_version"] = prep["previous_presence_version"]
    assert manifest == baseline
    for lang in ("en-US", "es-CO"):
        report = read(folder / f"installed-{lang}/report.json")
        assert report["technical_route_passed"] and report["cleanup_status"] == 204
        assert all(case["complete_playback"] for case in report["cases"])
        assert report["worker_image_id"] == install["worker"]
    for name in ("baseline-en", "candidate-en-prefix", "baseline-es", "candidate-es-prefix",
                 "baseline-cadence", "candidate-cadence", "candidate-mobile-cadence"):
        report = read(folder / f"browser-{name}/report.json")
        assert report["pass"] and report["cleanup"] == 204 and report["provider_calls"] == 0
        assert report["presentation"]["rate"] == 1 and sorted(report["presentation"]["tracks"]) == ["audio", "video"]
        if "prefix" in name:
            assert report["decoded_audio_before_completion"]
    assert read(folder / "webkit-laughter.json")["passed"]
    assert read(folder / "candidate-device-native-timing.json")["passed"]
    assert inventory(folder / "source/services/render-worker/internal/nativerender") == inventory(before / "source/services/render-worker/internal/nativerender")
    return summary


def store_and_restore(directory: Path, name: str, metadata: dict):
    expected = inventory(directory)
    db = Archive()
    snapshot = db.archive_tree(name, directory, metadata)
    destination = ROOT / ".tools" / f"{name}-restored-{snapshot['snapshot_id']}"
    if not destination.exists():
        db.restore(snapshot["snapshot_id"], destination)
    if inventory(destination) != expected:
        raise ValueError("Restored file inventory or SHA-256 content differs")
    receipt = {**snapshot, "restored_files": len(expected), "all_hashes_match": True,
               "provider_calls": 0}
    db.event(name, "restoration-verified", receipt)
    (ROOT / ".tools" / f"{name}-archive.json").write_text(json.dumps(receipt, indent=2))
    print(json.dumps(receipt))


def main():
    cli = argparse.ArgumentParser(description=__doc__)
    cli.add_argument("--completion", action="store_true", help="Archive verification documents and the restored primary receipt.")
    args = cli.parse_args()
    folder = ROOT / ".tools" / NAME
    summary = qualify(folder)
    if args.completion:
        primary = read(ROOT / ".tools" / f"{NAME}-archive.json")
        assert primary["all_hashes_match"]
        completion = ROOT / ".tools" / f"{NAME}-completion"
        copy_sources(completion, DOCS)
        (completion / "primary-restoration.json").write_text(json.dumps(primary, indent=2))
        store_and_restore(completion, NAME + "-completion", {"parent": primary["snapshot_id"], "provider_calls": 0})
        return
    copy_sources(folder / "source", (*DOCS,
                 "services/processor/app/workflows/motion/prepare_intentional_motion.py",
                 "services/processor/app/audits/finalize_fast_intentional.py"))
    (folder / "source-manifest.json").write_text(json.dumps(inventory(folder / "source"), indent=2, sort_keys=True))
    store_and_restore(folder, NAME, {**summary, "parent_snapshots": [836, 837, 838]})


if __name__ == "__main__":
    main()
