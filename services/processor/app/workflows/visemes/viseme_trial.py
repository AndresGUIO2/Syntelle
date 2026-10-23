"""One cached local viseme experiment; no public generation endpoint."""

import argparse
import json
import os
import sys
from datetime import datetime, timezone
from hashlib import sha256
from io import BytesIO
from pathlib import Path

from dotenv import dotenv_values
from PIL import Image, ImageDraw, ImageOps, UnidentifiedImageError

from app.main import ImageError, inspect_image
from app.preparation.faces import MODEL_SHA256, MODEL_URL, FaceDetector, PreparationError
from app.preparation.gemini import DEFAULT_MODEL, REQUEST_SETTINGS, VISEMES, build_prompt, generate_a
from app.preparation.mouth import compose_mouth, draw_regions, export_lossless_pair, export_overlay, make_mouth_mask

from app.core.paths import ROOT


def write_report(path: Path, report: dict) -> None:
    temporary = path.with_suffix(".tmp")
    temporary.write_text(json.dumps(report, indent=2, ensure_ascii=False), encoding="utf-8")
    temporary.replace(path)


def read_generated(data: bytes, base_size: tuple[int, int]) -> Image.Image:
    try:
        with Image.open(BytesIO(data)) as image:
            if image.format not in {"PNG", "JPEG", "WEBP"}:
                raise PreparationError("Generated file is not a supported image.")
            image.verify()
        with Image.open(BytesIO(data)) as image:
            image.load()
            result = ImageOps.exif_transpose(image).convert("RGB")
    except (UnidentifiedImageError, OSError, ValueError, Image.DecompressionBombError):
        raise PreparationError("Generated image could not be decoded.") from None
    if abs((result.width / result.height) / (base_size[0] / base_size[1]) - 1) > 0.02:
        raise PreparationError("Generated framing changed aspect ratio; automatic stretching is not allowed.")
    return result.resize(base_size, Image.Resampling.LANCZOS)


def save_provider_image(data: bytes, output: Path, viseme: str = "A") -> Path:
    # Keep the paid response even if decoding or composition subsequently fails.
    pending = output / "provider-output.bin"
    pending.write_bytes(data)
    try:
        with Image.open(BytesIO(data)) as image:
            extension = {"JPEG": ".jpg", "PNG": ".png", "WEBP": ".webp"}.get(image.format)
    except (UnidentifiedImageError, OSError):
        raise PreparationError("Provider output could not be decoded; its original bytes were retained.") from None
    if extension is None:
        raise PreparationError("Unsupported provider output; its original bytes were retained.")
    destination = output / f"generated-{viseme.lower()}{extension}"
    pending.replace(destination)
    return destination


def comparison(base: Image.Image, composed: Image.Image, mask: Image.Image, viseme: str = "A") -> Image.Image:
    bounds = mask.getbbox()
    assert bounds is not None
    left, top, right, bottom = bounds
    padding = round((right - left) * 0.25)
    bounds = (max(0, left - padding), max(0, top - padding), min(base.width, right + padding), min(base.height, bottom + padding))
    height = round(520 * (bounds[3] - bounds[1]) / (bounds[2] - bounds[0]))
    result = Image.new("RGB", (1064, height + 52), "#FAFAF9")
    draw = ImageDraw.Draw(result)
    for offset, image, label in [(8, base, "Immutable base"), (536, composed, f"{viseme} viseme - mouth overlay only")]:
        draw.text((offset + 8, 12), label, fill="#24212B", font_size=16)
        result.paste(image.crop(bounds).resize((520, height), Image.Resampling.LANCZOS), (offset, 44))
    return result


def run(args: argparse.Namespace) -> int:
    viseme = getattr(args, "viseme", "A")
    if viseme not in VISEMES:
        raise PreparationError("Unknown viseme identifier.")
    slug = viseme.lower()
    input_path, output = args.input.resolve(), args.output.resolve()
    if input_path == output or output in input_path.parents:
        raise PreparationError("Keep the immutable input outside the experiment output directory.")
    source_bytes = input_path.read_bytes()
    metadata = inspect_image(source_bytes)
    source_hash = sha256(source_bytes).hexdigest()
    with Image.open(BytesIO(source_bytes)) as source:
        source.load()
        base = ImageOps.exif_transpose(source).convert("RGB")
    values = dotenv_values(ROOT / ".env")
    model = args.model or os.environ.get("GEMINI_IMAGE_MODEL") or values.get("GEMINI_IMAGE_MODEL") or DEFAULT_MODEL
    output.mkdir(parents=True, exist_ok=True)
    report_path = output / "run.json"
    report = json.loads(report_path.read_text(encoding="utf-8")) if report_path.exists() else {"status": "base_ready"}
    if report.get("source_sha256", source_hash) != source_hash:
        raise PreparationError("This output belongs to a different base image. Use a separate directory.")
    if report.get("viseme", viseme) != viseme:
        raise PreparationError("This output belongs to a different viseme.")
    report.update({
        "schema": "syntelle-viseme-trial/1", "source_sha256": source_hash,
        "canvas": {"width": base.width, "height": base.height},
        "model_bundle": {"url": MODEL_URL, "sha256": MODEL_SHA256},
        "updated_at": datetime.now(timezone.utc).isoformat(),
        "viseme": viseme, "pipeline_version": "syntelle-preparation/2",
        "export_settings": {"png": "lossless", "webp": {"lossless": True, "exact": True, "method": 6}, "alignment": "similarity-stable-anchors/1", "mask": "lip-union-feather/1"},
    })
    export_lossless_pair(base, output, "base")
    report["base_exports"] = {"png": "base.png", "lossless_webp": "base.webp", "decoded_pixels_identical": True}
    with FaceDetector(args.face_model.resolve()) as detector:
        base_face = detector.detect(base)
        (output / "base-landmarks.json").write_text(json.dumps(base_face.as_dict(), indent=2), encoding="utf-8")
        draw_regions(base, base_face).save(output / "detected-regions.png")
        base_mask = make_mouth_mask(base.size, base_face.points, base_face.points)
        base_mask.save(output / "base-mouth-mask.png")
        bounds = base_mask.getbbox()
        assert bounds is not None
        report["base_detection"] = {
            "faces": 1, "landmarks": len(base_face.points), "blendshapes": len(base_face.blendshapes),
            "mouth_region": {"x": bounds[0], "y": bounds[1], "width": bounds[2] - bounds[0], "height": bounds[3] - bounds[1]},
        }
        write_report(report_path, report)
        print(f"Base ready: one face, {len(base_face.points)} landmarks, automatic mouth region {bounds}.")
        if not args.generate and args.variant is None:
            print(f"Inspection artifacts: {output}")
            return 0

        raw_name = report.get("generation", {}).get("file", f"generated-{slug}.jpg")
        if raw_name not in {f"generated-{slug}{extension}" for extension in (".jpg", ".png", ".webp")}:
            raise PreparationError("Invalid provider file reference in the local run record.")
        raw_path = output / raw_name
        if args.variant is not None:
            variant_bytes = args.variant.resolve().read_bytes()
            if len(variant_bytes) > 20_000_000:
                raise PreparationError("Generated image is too large.")
            raw_path = save_provider_image(variant_bytes, output, viseme)
            report["generation"] = {"provider": "manual", "status": "received", "file": raw_path.name}
        elif raw_path.exists():
            expected = report.get("generation", {}).get("raw_sha256")
            if expected and sha256(raw_path.read_bytes()).hexdigest() != expected:
                raise PreparationError("The cached provider image changed; restore its original bytes.")
            print("Reusing the saved provider image; no new paid request.")
        else:
            if report.get("generation", {}).get("status") in {"request_started", "failed"} or report.get("generation", {}).get("attempts", 0) >= 1:
                raise PreparationError("A previous request may have incurred cost. Inspect run.json and use a new output directory for an intentional retry.")
            api_key = os.environ.get("GEMINI_API_KEY") or values.get("GEMINI_API_KEY") or ""
            if not api_key.strip():
                raise PreparationError("GEMINI_API_KEY is empty. Save it in the local .env file; do not paste it into chat.")
            prompt_source = ROOT / "docs/prompts/nano-banana-source.txt"
            prompt = build_prompt(prompt_source, viseme)
            (output / f"prompt-{slug}.txt").write_bytes(prompt.encode("utf-8"))
            report["generation"] = {"status": "request_started", "model": model, "prompt_sha256": sha256(prompt.encode()).hexdigest(), "source_prompt_sha256": sha256(prompt_source.read_bytes()).hexdigest(), "attempts": 1, "started_at": datetime.now(timezone.utc).isoformat(), "request_settings": {"response_format": REQUEST_SETTINGS.copy(), "store": False, "retries": 0, "timeout_seconds": 180, "input_mime_type": metadata.mime_type, "input_sha256": source_hash}}
            write_report(report_path, report)
            try:
                variant_bytes, generation = generate_a(source_bytes, prompt, api_key, model, metadata.mime_type)
            except PreparationError as exc:
                report["status"] = "provider_request_failed"
                report["generation"]["status"] = "failed"
                report["generation"]["error"] = str(exc)
                report["generation"]["finished_at"] = datetime.now(timezone.utc).isoformat()
                write_report(report_path, report)
                raise
            # Persist the costly output before alignment or mask validation can fail.
            raw_path = save_provider_image(variant_bytes, output, viseme)
            saved_request = report["generation"]["request_settings"]
            report["generation"].update(generation)
            report["generation"]["request_settings"].update(saved_request)
            report["generation"]["finished_at"] = datetime.now(timezone.utc).isoformat()
            report["generation"]["status"] = "received"
            report["generation"]["file"] = raw_path.name

        report["generation"]["raw_sha256"] = sha256(raw_path.read_bytes()).hexdigest()
        write_report(report_path, report)
        try:
            generated = read_generated(raw_path.read_bytes(), base.size)
            variant_face = detector.detect(generated)
            (output / "variant-landmarks.json").write_text(json.dumps(variant_face.as_dict(), indent=2), encoding="utf-8")
            result = compose_mouth(base, generated, base_face, variant_face, viseme)
        except PreparationError as exc:
            report["status"] = "composition_rejected"
            report["composition_error"] = str(exc)
            write_report(report_path, report)
            raise
        result.image.save(output / f"composed-{slug}.png")
        export_overlay(result, output, f"mouth-{slug}")
        result.mask.save(output / "mouth-mask.png")
        comparison(base, result.image, result.mask, viseme).save(output / "mouth-comparison.png")
        left, top, right, bottom = result.bounds
        report.update({
            "status": "ready_for_visual_review", "viseme": viseme,
            "overlay": {"file": f"mouth-{slug}.png", "lossless_webp": f"mouth-{slug}.webp", "x": left, "y": top, "width": right-left, "height": bottom-top},
            "metrics": result.metrics,
        })
        write_report(report_path, report)
        if sha256(input_path.read_bytes()).hexdigest() != source_hash:
            raise PreparationError("The input changed during this experiment.")
        print("Composition ready: zero changed pixels outside the mouth mask.")
        print(f"Review: {output / 'mouth-comparison.png'}")
        return 0


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--input", type=Path, required=True, help="Owned or authorized immutable base image.")
    parser.add_argument("--output", type=Path, default=ROOT / ".tools/viseme-a")
    parser.add_argument("--face-model", type=Path, default=ROOT / ".tools/models/face_landmarker.task")
    parser.add_argument("--model", help="Explicit Gemini model override for this local experiment.")
    parser.add_argument("--viseme", choices=VISEMES, default="A")
    action = parser.add_mutually_exclusive_group()
    action.add_argument("--generate", action="store_true", help="Make at most one paid Gemini request, then compose the mouth.")
    action.add_argument("--variant", type=Path, help="Compose an existing generated variant without contacting Gemini.")
    args = parser.parse_args()
    try:
        return run(args)
    except (PreparationError, ImageError) as exc:
        message = exc.message if isinstance(exc, ImageError) else str(exc)
        print(f"Preparation stopped: {message}", file=sys.stderr)
        return 1
    except OSError:
        print("Preparation stopped: an input, model, or output file could not be accessed.", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
