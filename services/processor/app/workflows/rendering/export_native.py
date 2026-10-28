"""Export immutable, CPU-ready RGBA resources without provider requests.

Original packages/images remain unchanged. The manifest binds every raw texture
to its source hash and retains the original preparation JSON verbatim as values.
"""
from __future__ import annotations

import argparse
import hashlib
import json
from pathlib import Path

from PIL import Image

ROOT = Path(__file__).resolve().parents[5]


def digest(data: bytes) -> str:
    return hashlib.sha256(data).hexdigest()


def source_name(path: Path) -> str:
    try:
        return path.resolve().relative_to(ROOT).as_posix()
    except ValueError:
        return path.resolve().as_posix()


def resource_paths(bundle: Path | None, expressions_dir: Path | None) -> tuple[dict[str, Path], dict | None]:
    web = ROOT / "apps/web/src/experiments"
    folders = {
        "oral": web / "viseme-a/characters/syntelle",
        "motion": web / "motion/rgbd/syntelle",
        "motion_assets": web / "motion/assets/syntelle",
        "gaze": web / "motion/gaze/syntelle",
        "expressions": web / "motion/expressions/syntelle",
        "backgrounds": web / "motion/backgrounds",
    }
    specification = None
    if bundle:
        specification = json.loads(bundle.read_text(encoding="utf-8"))
        if specification.get("version") != "character-resource-bundle/1":
            raise ValueError("Unsupported character resource bundle")
        if len(specification.get("source_sha256", "")) != 64:
            raise ValueError("Resource bundle requires an immutable portrait checksum")
        for key in folders:
            value = specification.get(key)
            if not isinstance(value, str) or not value:
                raise ValueError(f"Resource bundle is missing {key}")
            path = Path(value)
            folders[key] = (ROOT / path).resolve() if not path.is_absolute() else path.resolve()
    if expressions_dir:
        folders["expressions"] = expressions_dir.resolve()
    filenames = {"oral": "package.json", "motion": "package.json", "gaze": "gaze.json", "expressions": "expressions.json", "backgrounds": "catalog.json"}
    for key, filename in filenames.items():
        if folders[key].is_file():
            if folders[key].name != filename:
                raise ValueError(f"Resource bundle {key} must name {filename} or its directory")
            folders[key] = folders[key].parent
    return folders, specification


def validate_bindings(packages: dict, specification: dict | None) -> None:
    oral, motion, gaze, expressions = (packages[key] for key in ("oral", "motion", "gaze", "expressions"))
    source = motion["original_sha256"]
    if specification and specification["source_sha256"] != source:
        raise ValueError("Resource bundle belongs to a different portrait")
    if any(package.get("source_sha256") != source for package in (oral, gaze, expressions)):
        raise ValueError("Prepared resources belong to different portraits")
    if motion.get("oral_build_id") != oral["build_id"] or expressions.get("oral_build_id") != oral["build_id"]:
        raise ValueError("Prepared oral build binding mismatch")
    if any(package.get("motion_build_id") != motion["build_id"] for package in (gaze, expressions)):
        raise ValueError("Prepared motion build binding mismatch")


def export(destination: Path, expressions_dir: Path | None = None, motion_scale: float = 1, envelopes: Path | None = None, bundle: Path | None = None) -> dict:
    folders, specification = resource_paths(bundle, expressions_dir)
    oral_dir, motion_dir, expression_dir = (folders[key] for key in ("oral", "motion", "expressions"))
    package_paths = {
        "oral": oral_dir / "package.json",
        "motion": motion_dir / "package.json",
        "gaze": folders["gaze"] / "gaze.json",
        "expressions": expression_dir / "expressions.json",
    }
    packages = {name: json.loads(path.read_bytes()) for name, path in package_paths.items()}
    validate_bindings(packages, specification)
    from app.preparation.reference_policy import validate_export_coverage
    validate_export_coverage(packages['motion'])
    destination.mkdir(parents=True, exist_ok=True)
    data_dir = destination / "data"
    data_dir.mkdir(exist_ok=True)
    # Preserve the public API's character identity, including unused prepared
    # files. It must not depend on a worker implementation or raw RGBA export.
    identity_files = []
    for folder in (
        oral_dir, folders["motion_assets"], motion_dir, folders["gaze"], expression_dir,
    ):
        for path in sorted(folder.rglob("*"), key=lambda value: value.as_posix()):
            if path.is_symlink():
                raise ValueError("Prepared resources must be ordinary files")
            if path.is_file():
                original = path.read_bytes()
                identity_files.append({
                    "bytes": len(original), "path": source_name(path),
                    "sha256": digest(original),
                })
    identity_json = json.dumps(identity_files, separators=(",", ":"), ensure_ascii=False)
    package_hash = digest(identity_json.encode("utf-8"))
    records: dict[str, dict] = {}

    def texture(logical: str, source: Path) -> None:
        if logical in records:
            return
        original = source.read_bytes()
        with Image.open(source) as decoded:
            image = decoded.convert("RGBA")
            pixels = image.tobytes()
            width, height = image.size
        checksum = digest(pixels)
        target = data_dir / f"{checksum}.rgba"
        if target.exists() and target.read_bytes() != pixels:
            raise ValueError("Native export hash collision")
        if not target.exists():
            target.write_bytes(pixels)
        records[logical] = {
            "file": f"data/{target.name}", "sha256": checksum,
            "width": width, "height": height,
            "source": source_name(source),
            "source_sha256": digest(original),
        }

    oral = packages["oral"]
    for filename in (oral["reference"]["upper"], oral["reference"]["lower"]):
        texture("oral/" + filename, oral_dir / filename)
    for pose in oral["poses"].values():
        for field in ("surface", "mask", "cavity"):
            if pose.get(field):
                texture("oral/" + pose[field], oral_dir / pose[field])
    motion = packages["motion"]
    for filename in (motion["foreground"], motion["eyes"]["open"], motion["eyes"]["closed"], motion["eyes"]["mask"]):
        texture("motion/" + filename, folders["motion_assets"] / filename)
    for filename in ("sclera.png", "ownership.png"):
        texture("gaze/" + filename, folders["gaze"] / filename)
    expressions = packages["expressions"]
    texture("expressions/" + expressions["field"]["file"], expression_dir / expressions["field"]["file"])
    if expressions.get("emotion_rig"):
        filename = expressions["emotion_rig"]["field"]["file"]
        texture("expressions/" + filename, expression_dir / filename)
    if expressions.get("smile", {}).get("material"):
        filename = expressions["smile"]["material"]["file"]
        texture("expressions/" + filename, expression_dir / filename)
    for frame in expressions.get('joint_articulation',{}).get('skin_frames',[]):
        texture('expressions/'+frame['file'],expression_dir/frame['file'])
    for frame in expressions.get('joint_articulation',{}).get('laugh',{}).get('skin_frames',[]):
        texture('expressions/'+frame['file'],expression_dir/frame['file'])
    lip = expressions.get('joint_articulation',{}).get('laugh',{}).get('lip_material')
    if lip:
        texture('expressions/'+lip['file'],expression_dir/lip['file'])
    reference = expressions.get("reference_laughter")
    if reference:
        if reference.get("version") != "reference-laughter/1" or reference.get("source_sha256") != motion["original_sha256"] or reference.get("motion_build_id") != motion["build_id"] or reference.get("oral_build_id") != oral["build_id"]:
            raise ValueError("Reference laughter package binding mismatch")
        for asset in [*reference["frames"], reference["oral"]["cavity"],
                      *[reference['oral'][name] for name in ('observed_upper','observed_lower') if name in reference['oral']]]:
            source = expression_dir / asset["file"]
            if digest(source.read_bytes()) != asset["sha256"]:
                raise ValueError("Reference laughter asset checksum mismatch")
            texture("expressions/" + asset["file"], source)
    catalog = json.loads((folders["backgrounds"] / "catalog.json").read_bytes())
    for background in catalog["items"]:
        texture("background/" + background["id"], folders["backgrounds"] / background["image"])
    views = motion['projection'].get('multiview')
    if views:
        if views['version']!='registered-multiview/1' or views['source_sha256']!=motion['original_sha256'] or views['geometry_sha256']!=motion['projection']['geometry']['sha256']:
            raise ValueError('Multiview source/geometry binding mismatch')
        for view in views['views']:
            for kind in ('texture','uv'):
                item=view[kind]
                if digest((motion_dir/item['file']).read_bytes())!=item['sha256']:raise ValueError('Multiview resource checksum mismatch')
            texture('multiview/'+view['texture']['file'],motion_dir/view['texture']['file'])
    geometry = motion_dir / motion["projection"]["geometry"]["file"]
    geometry_bytes = geometry.read_bytes()
    geometry_hash = digest(geometry_bytes)
    if geometry_hash != motion["projection"]["geometry"]["sha256"]:
        raise ValueError("Prepared geometry checksum mismatch")
    (data_dir / f"{geometry_hash}.bin").write_bytes(geometry_bytes)
    manifest = {
        "version": "native-character/1", "packages": packages, "textures": records,
        "package_hash": package_hash, "identity_files": identity_files,
        "geometry": {"file": f"data/{geometry_hash}.bin", "sha256": geometry_hash},
        "package_sources": {name: {"path": source_name(path), "sha256": digest(path.read_bytes())} for name, path in package_paths.items()},
        "paid_provider_calls": 0,
    }
    if motion_scale!=1:
        if motion_scale not in (1.25,1.5) or envelopes is None:raise ValueError('A recorded wider envelope is required.')
        candidates=json.loads(envelopes.read_text(encoding="utf-8"))['candidates']
        candidate=next((r for r in candidates if r['source_build_id']==motion['build_id'] and r['scale']==motion_scale),None)
        if candidate is None or not candidate['validation']['valid'] or candidate.get('status')!='internal-visual-pass':raise ValueError('Wider envelope requires geometry and internal visual review.')
        manifest['motion_scale']=motion_scale
        manifest['motion_envelope']=candidate
    (destination / "manifest.json").write_text(json.dumps(manifest, indent=2) + "\n", encoding="utf-8")
    return {"directory": str(destination), "textures": len(records), "paid_provider_calls": 0}


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument('--expressions',type=Path)
    parser.add_argument('--bundle',type=Path,help='Explicit character-resource-bundle/1 manifest; relative resources resolve from the repository root.')
    parser.add_argument('--motion-scale',type=float,default=1)
    parser.add_argument('--envelopes',type=Path)
    args = parser.parse_args()
    print(json.dumps(export(args.output.resolve(),args.expressions.resolve() if args.expressions else None,args.motion_scale,args.envelopes,args.bundle.resolve() if args.bundle else None)))


if __name__ == "__main__":
    main()
