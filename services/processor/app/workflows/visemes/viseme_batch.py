"""Owner-operated bounded batch; exact immutable input, one attempt per remaining pose."""
import argparse
import json
from datetime import datetime, timezone
from hashlib import sha256
from pathlib import Path

from app.preparation.faces import PreparationError
from app.preparation.gemini import VISEMES
from app.workflows.visemes.viseme_trial import ROOT, run, write_report


def batch(args):
    output = args.output.resolve()
    output.mkdir(parents=True, exist_ok=True)
    source_hash = sha256(args.input.read_bytes()).hexdigest()
    reference = json.loads((args.reference / "run.json").read_text(encoding="utf-8"))
    if source_hash != reference["source_sha256"]:
        raise PreparationError("The approved A reference uses a different base.")
    path = output / "batch.json"
    report = json.loads(path.read_text()) if path.exists() else {
        "schema": "syntelle-viseme-batch/1", "source_sha256": source_hash,
        "reference_a": str(args.reference.resolve()), "request_limit": 7,
        "order": list(VISEMES[1:]), "poses": {}, "created_at": datetime.now(timezone.utc).isoformat(),
    }
    if report["source_sha256"] != source_hash:
        raise PreparationError("The batch belongs to a different input image.")
    for viseme in VISEMES[1:]:
        if sha256(args.input.read_bytes()).hexdigest()!=source_hash:
            raise PreparationError("The immutable input changed during the batch.")
        directory = output / viseme.lower()
        print(f"Preparing {viseme} (one request maximum, cached on resume).", flush=True)
        error = None
        try:
            run(argparse.Namespace(input=args.input, output=directory, viseme=viseme, generate=args.generate,
                                   variant=None, model=None, face_model=ROOT / ".tools/models/face_landmarker.task"))
        except PreparationError as exc:
            error = str(exc)
        pose_report = json.loads((directory / "run.json").read_text()) if (directory / "run.json").exists() else {}
        report["poses"][viseme] = {"directory": viseme.lower(), "status": pose_report.get("status", "failed"), "error": error}
        report["updated_at"] = datetime.now(timezone.utc).isoformat()
        write_report(path, report)
        if error and pose_report.get("status") != "composition_rejected":
            raise PreparationError(f"Batch stopped at {viseme}: {error}")
    print(f"Batch recorded at {path}", flush=True)


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--input", type=Path, required=True)
    parser.add_argument("--reference", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--generate", action="store_true")
    try:
        batch(parser.parse_args())
    except PreparationError as exc:
        raise SystemExit(str(exc)) from None
