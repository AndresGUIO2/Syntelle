"""Operator-authorized single-portrait preparation using the existing measured rigs."""
import io
from pathlib import Path
import shutil

import numpy as np
from PIL import Image, ImageOps

from app.core.paths import ROOT
from app.preparation.archive import encoded
from app.preparation.detection import PreparationDetector
from app.preparation import expressions, gaze, head_rig, motion_assets, projection_rig
from app.preparation.faces import MODEL_SHA256, PreparationError
from app.workflows.corpus import corpus_prepare as corpus
from app.workflows.motion import prepare_motion as motion
from app.workflows.oral import oral_semantics, prepare_semantic_oral
from .preparation import CreationError, digest, read, write

POSES = ("EI", "A", "O", "U", "MBP", "SZ", "FV", "CHSHR")
LIMITS = {"image": 9, "analysis": 9}
IMAGE_MODEL = "gemini-nano-banana-2.1"


def validate_models(models):
    """Check the complete local pin set before adapters with download fallbacks run."""
    pins = {
        "face_landmarker.task": MODEL_SHA256,
        "selfie-multiclass-256-float32-v1.tflite": read(models / "segmenter.json")["sha256"],
    }
    for name, files in (
        ("depth-anything-v2-small-hf", ("config.json", "preprocessor_config.json", "model.safetensors")),
        ("vitmatte-small-composition-1k", ("config.json", "preprocessor_config.json", "model.safetensors", "README.md")),
    ):
        record = read(models / name / "revision.json")
        for file in files:
            pins[f"{name}/{file}"] = record["files"][file]
    for name, expected in pins.items():
        path = models / name
        if not path.is_file() or digest(path.read_bytes()) != expected:
            raise CreationError("models_unavailable")


def build_bundle(source, work, approval, providers):
    if approval["source_sha256"] != digest(source) or approval["limits"] != LIMITS or providers is None:
        raise CreationError("generation_not_enabled")
    models = ROOT / ".tools/models"
    try:
        validate_models(models)
    except (OSError, KeyError, ValueError):
        raise CreationError("models_unavailable") from None
    originals = work / "base"
    originals.mkdir()
    with Image.open(io.BytesIO(source)) as upload:
        suffix = {"JPEG": ".jpg", "PNG": ".png", "WEBP": ".webp"}.get(upload.format)
        if suffix is None or max(upload.size) > 4096 or upload.width * upload.height > 4_194_304:
            raise CreationError("portrait_resolution_unsupported")
        image = ImageOps.exif_transpose(upload).convert("RGB")
    ratio = image.width / image.height
    allowed = (1, 3/4, 4/3, 9/16, 16/9)
    if min(abs(ratio / a - 1) for a in allowed) > .02:
        raise CreationError("portrait_aspect_unsupported")
    original = originals / ("source" + suffix)
    original.write_bytes(source)
    with PreparationDetector(models / "face_landmarker.task") as detector:
        try:
            face = detector.detect(image)
        except PreparationError:
            raise CreationError("portrait_face_unsuitable") from None
        # The native landmarker supplies a rigid face transform. Reject strong
        # out-of-plane rotations before the first remote request.
        forward = np.asarray(face.matrix)[:3, 2]
        if not np.isfinite(forward).all() or np.linalg.norm(forward) < 1e-6 or abs(forward[2])/np.linalg.norm(forward) < .94:
            raise CreationError("portrait_face_unsuitable")
        write(originals / "base-landmarks.json", face.as_dict())
        for pose in POSES:
            corpus.generate_pose(original, image, face, detector, work / "poses" / pose.lower(),
                                 pose, providers["image"], IMAGE_MODEL, True)
    oral_input = corpus.input_manifest(original, image, work, POSES)
    oral_semantics.analyze(oral_input, work / "semantics", ["Rest", *POSES], request=True,
                           provider=providers["analysis"], allow_correction=False)
    material = corpus.select_material(oral_input, work / "semantics", work / "material")
    oral_dir = work / "oral"
    prepare_semantic_oral.prepare(oral_input, material, work / "semantics", oral_dir)
    oral = read(oral_dir / "package.json")
    if not oral["ready"]:
        raise CreationError("geometry_failed")
    asset_dir = work / "motion-assets"
    asset_dir.mkdir()
    config = read(ROOT / "docs/characters/motion-development.json")
    motion.generation(original, image, face.points, asset_dir, providers["image"], config, True)
    motion.eye_assets(image, face, asset_dir, models)
    motion_assets.subject(image, face.points, models, asset_dir)
    motion_assets.depth(image, models, asset_dir)
    m = motion.package(config, {"oral_path": oral_dir}, original, image, face, asset_dir)
    classes = np.load(asset_dir / "class-confidence.npz")["probabilities"]
    rig = head_rig.prepare(face.points, classes, Image.open(asset_dir / "depth.webp").convert("L"), image.size)
    if not rig["geometry_validation"]["valid"]:
        raise CreationError("geometry_failed")
    rig.update(source_sha256=digest(source), previous_build_id=m["build_id"])
    settings = read(ROOT / "docs/characters/vitality-motion.json")
    m.update(rig=rig, behavior={k:v for k,v in settings.items() if k!="breathing"},
             breathing=settings["breathing"], animation_seed=int(m["build_id"][:8],16))
    rgbd_dir = work / "projection"
    rgbd_dir.mkdir()
    geom, vertices, faces, edges, _ = projection_rig.prepare(
        m, np.load(asset_dir / "relative-depth.npy"), face.points, classes, Image.open(asset_dir / "foreground.webp").convert("RGBA"))
    if not geom["ready"]:
        raise CreationError("geometry_failed")
    binary = vertices.tobytes() + faces.tobytes() + edges.tobytes()
    (rgbd_dir / "projection.bin").write_bytes(binary)
    geom["geometry"]["sha256"] = digest(binary)
    geom.update(source_sha256=digest(source), parent_build_id=m.pop("build_id"))
    m["projection"] = geom
    m["build_id"] = digest(encoded(m))
    write(rgbd_dir / "package.json", m)
    gaze_dir, expression_dir = work / "gaze", work / "expressions"
    gaze_dir.mkdir(); expression_dir.mkdir()
    gaze.prepare(image, face.points, m, gaze_dir)
    ex = expressions.prepare(image, face.points, m, oral, Image.open(oral_dir / oral["reference"]["upper"]),
                             Image.open(oral_dir / oral["reference"]["lower"]), expression_dir)
    ex["field"] = {"file":"field.png", "sha256":digest((expression_dir / "field.png").read_bytes())}
    write(expression_dir / "expressions.json", ex)
    # Export only runtime materials, not original uploads or intermediate request files.
    assets = work / "runtime-motion"
    for name in (m["foreground"], *(m["eyes"][k] for k in ("open","closed","mask"))):
        dest = assets / name
        dest.parent.mkdir(parents=True, exist_ok=True)
        shutil.copyfile(asset_dir / name, dest)
    return {"version":"character-resource-bundle/1", "source_sha256":digest(source), "oral":str(oral_dir),
            "motion":str(rgbd_dir), "motion_assets":str(assets), "gaze":str(gaze_dir),
            "expressions":str(expression_dir), "backgrounds":"apps/web/src/experiments/motion/backgrounds"}
