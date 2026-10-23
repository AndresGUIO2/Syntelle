"""Compare fixed canonical browser images with the native renderer offline."""

import argparse
import json
import math
from pathlib import Path

import numpy as np
from PIL import Image


def rgba(path):
    with Image.open(path) as image:
        return np.asarray(image.convert("RGBA"), dtype=np.float64)


def psnr(a, b):
    mse = float(np.mean((a - b) ** 2))
    return 10 * math.log10(255**2 / max(mse, 1e-10))


def component(reference, candidate):
    a, b = rgba(reference), rgba(candidate)
    if a.shape != b.shape:
        raise ValueError(f"Component dimensions differ: {reference.name}")
    # Hidden straight RGB is not visible when the patch is applied source-atop.
    # Compare covered colors, including the alpha channel, without rewarding a
    # darker or more transparent candidate.
    covered_a, covered_b = a.copy(), b.copy()
    covered_a[..., :3] *= a[..., 3:4] / 255
    covered_b[..., :3] *= b[..., 3:4] / 255
    return {
        "psnr_db": psnr(covered_a, covered_b),
        "maximum_covered_channel_difference": float(np.max(np.abs(covered_a - covered_b))),
        "maximum_alpha_difference": float(np.max(np.abs(a[..., 3] - b[..., 3]))),
    }


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--reference", type=Path, required=True)
    parser.add_argument("--candidate", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--minimum-face-psnr", type=float, default=45)
    args = parser.parse_args()
    states = json.loads((args.reference / "states.json").read_text(encoding="utf-8"))
    cases = []
    for case in states:
        name = case["name"]
        a, b = rgba(args.reference / f"{name}.png"), rgba(args.candidate / f"{name}.png")
        if a.shape != (960, 720, 4) or b.shape != a.shape:
            raise ValueError(f"Scene dimensions differ: {name}")
        delta = np.abs(a[..., :3] - b[..., :3])
        yy, xx = np.unravel_index(np.argmax(delta.max(axis=2)), delta.shape[:2])
        face = psnr(a[180:680, 180:540, :3], b[180:680, 180:540, :3])
        cases.append({
            "name": name,
            "face_psnr_db": face,
            "scene_psnr_db": psnr(a[..., :3], b[..., :3]),
            "maximum_scene_channel_difference": float(delta.max()),
            "maximum_difference_pixel": [int(xx), int(yy)],
            "mouth": component(args.reference / f"{name}-mouth.png", args.candidate / f"{name}-mouth.png"),
            "eyes": component(args.reference / f"{name}-eyes.png", args.candidate / f"{name}-eyes.png"),
            "passed": face >= args.minimum_face_psnr,
        })
    report = {
        "version": "native-render-pixel-review/1",
        "reference": str(args.reference),
        "candidate": str(args.candidate),
        "face_region": [180, 180, 540, 680],
        "minimum_face_psnr_db": args.minimum_face_psnr,
        "component_metric": "premultiplied RGBA; alpha differences retained",
        "cases": cases,
        "passed": bool(cases) and all(case["passed"] for case in cases),
        "paid_provider_calls": 0,
    }
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(json.dumps(report, indent=2) + "\n", encoding="utf-8")
    print(json.dumps({"output": str(args.output), "cases": len(cases), "passed": report["passed"],
                      "minimum_face_psnr_db": min(case["face_psnr_db"] for case in cases)}))
    return 0 if report["passed"] else 1


if __name__ == "__main__":
    raise SystemExit(main())
