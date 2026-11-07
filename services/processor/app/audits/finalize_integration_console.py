"""Archive the integration console review without private customer databases."""

from hashlib import sha256
import json
from pathlib import Path
import shutil

from app.core.paths import ROOT
from app.preparation.archive import Archive


def inventory(folder: Path):
    return {
        path.relative_to(folder).as_posix(): sha256(path.read_bytes()).hexdigest()
        for path in folder.rglob("*") if path.is_file() and "__pycache__" not in path.parts and path.suffix not in {".pyc", ".tmp"}
    }


def main():
    name = "integration-console-126"
    evidence = ROOT / ".tools" / name
    summary = json.loads((evidence / "validation-summary.json").read_bytes())
    install = json.loads((evidence / "installation.json").read_bytes())
    if not summary["technical_checks_passed"] or summary["provider_calls"] != 0 or install["provider_calls"] != 0:
        raise ValueError("Verified local checks and zero-provider installation receipts are required")
    areas = ["apps/web/src", "apps/web/config", "apps/web/tests", "apps/web/public/widget",
             "services/api", "packages/render-contract", "services/processor/app/workflows/characters",
             "specs/011-integration-console", "deploy/edge"]
    suffixes = {".go", ".sql", ".mod", ".sum", ".ts", ".vue", ".css", ".md", ".json", ".mjs", ".js", ".py"}
    sources = [path for area in areas for path in (ROOT / area).rglob("*")
               if path.is_file() and path.suffix in suffixes and "__pycache__" not in path.parts]
    extras = ["AGENTS.md", "compose.yaml", "apps/web/package.json", "apps/web/package-lock.json",
              "apps/web/vite.config.ts", "apps/web/playwright.config.ts", "apps/web/tsconfig.json",
              "docs/development/integration-console.md",
              "docs/development/paddle-sandbox.md", "docs/development/navigation.md", "docs/product/brand.md",
              "services/processor/app/audits/finalize_integration_console.py"]
    extras.append("services/processor/tests/test_character_creation_worker.py")
    sources += [ROOT / relative for relative in extras]
    for source in sources:
        if not source.is_file():
            raise ValueError(f"Missing source: {source.relative_to(ROOT)}")
        target = evidence / "source" / source.relative_to(ROOT)
        target.parent.mkdir(parents=True, exist_ok=True)
        shutil.copyfile(source, target)
    # Operational binaries and local rollback DBs are never put in this folder.
    if any((path.suffix == ".dump" and path.relative_to(evidence).as_posix() != "billing/fixtures.dump")
           or path.name == ".env" for path in evidence.rglob("*")):
        raise ValueError("Private database/secret file cannot enter review evidence")
    expected = inventory(evidence)
    archive = Archive()
    result = archive.archive_tree(name, evidence, {**summary, "parent_snapshots": [846]})
    restored = ROOT / ".tools" / f"{name}-restored-{result['snapshot_id']}"
    archive.restore(result["snapshot_id"], restored)
    if inventory(restored) != expected:
        raise ValueError("Restored evidence bytes differ")
    receipt = {**result, "restored_files": len(expected), "all_hashes_match": True, "provider_calls": 0}
    archive.event(name, "restoration-verified", receipt)
    (ROOT / ".tools" / f"{name}-archive.json").write_text(json.dumps(receipt, indent=2))
    print(json.dumps(receipt))


if __name__ == "__main__":
    main()
