"""Archive the local character-creation review and verify byte-exact restoration.

Only known development portraits/fixtures enter this immutable review snapshot.
Customer upload/request bodies remain in the private expiring creation store.
"""
from hashlib import sha256
import json
from pathlib import Path
import shutil

from app.core.paths import ROOT
from app.preparation.archive import Archive

NAME = "character-creation-123"


def inventory(directory):
    return {p.relative_to(directory).as_posix(): sha256(p.read_bytes()).hexdigest()
            for p in directory.rglob("*") if p.is_file() and "__pycache__" not in p.parts}


def copy_file(source, destination):
    destination.parent.mkdir(parents=True, exist_ok=True)
    shutil.copyfile(source, destination)


def freeze_sources(folder):
    # Keep the native tree created by the explicit phase122 patch transplant.
    # Never overwrite it with the mixed historical native development tree.
    source = folder / "source"
    copy_file(ROOT / ".tools/private-characters/generation-campaign.json", folder / "approved-campaign.json")
    for area in ("services/api", "services/processor/app", "services/processor/tests",
                 "apps/web/src", "apps/web/config", "apps/web/tests", "scripts/development"):
        for path in (ROOT / area).rglob("*"):
            if (path.is_file() and path.suffix in (".go", ".sql", ".py", ".ts", ".vue", ".css", ".mjs", ".ps1")
                    and not any(part in ("__pycache__", "node_modules", ".venv") for part in path.parts)):
                copy_file(path, source / path.relative_to(ROOT))
    extras = ["services/api/go.mod", "services/api/go.sum", "apps/web/package.json",
              "apps/web/package-lock.json", "apps/web/vite.config.ts", "apps/web/tsconfig.json",
              "apps/web/tsconfig.app.json", "apps/web/tsconfig.node.json", "apps/web/playwright.config.ts",
              "apps/web/Dockerfile", "services/render-worker/scripts/start-local.mjs",
              "docs/characters/creation-source-cache.json", "docs/characters/creation-workflow.md",
              "docs/development/character-creation.md", "AGENTS.md"]
    extras += [p.relative_to(ROOT).as_posix() for p in (ROOT / "specs/009-character-creation").glob("*.md")]
    for name in extras:
        if (ROOT / name).is_file(): copy_file(ROOT / name, source / name)
    for case in ("cafe", "woodland"):
        copy_file(ROOT / f".tools/semantic-oral-06/corpus/{case}/original.png", folder / "inputs" / f"{case}.png")
    for name in ("request.json", "response.json"):
        copy_file(ROOT / f".tools/av-sync-emma-106/speech/en-US/{name}", folder / "inputs" / "cached-emma" / name)
    review = ROOT / ".tools/character-creation-review"
    for name in ("replay_generation.py", "replay-generation.log"):
        copy_file(review / name, folder / "offline-replay" / name)
    for path in (review / "replay-1793769081204307200").rglob("*"):
        if path.is_file(): copy_file(path, folder / "offline-replay" / "complete-preparation" / path.relative_to(review / "replay-1793769081204307200"))
    (folder / "source-manifest.json").write_text(json.dumps(inventory(source), sort_keys=True, indent=2))


def main():
    folder = ROOT / ".tools" / NAME
    summary = json.loads((folder / "validation-summary.json").read_bytes())
    install = json.loads((folder / "installation.json").read_bytes())
    if not summary["technical_route_passed"] or summary["provider_calls"] != 0 or install["provider_calls"] != 0:
        raise ValueError("Complete offline route and installation evidence required")
    freeze_sources(folder)
    expected = inventory(folder)
    archive = Archive()
    snapshot = archive.archive_tree(NAME, folder, {**summary, "parent_snapshots": [840, 841]})
    destination = ROOT / ".tools" / f"{NAME}-restored-{snapshot['snapshot_id']}"
    archive.restore(snapshot["snapshot_id"], destination)
    if inventory(destination) != expected:
        raise ValueError("Restored review files differ")
    receipt = {**snapshot, "restored_files": len(expected), "all_hashes_match": True, "provider_calls": 0}
    archive.event(NAME, "restoration-verified", receipt)
    (ROOT / ".tools" / f"{NAME}-archive.json").write_text(json.dumps(receipt, indent=2))
    print(json.dumps(receipt))


if __name__ == "__main__":
    main()
