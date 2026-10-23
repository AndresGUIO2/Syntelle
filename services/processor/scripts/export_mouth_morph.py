"""Export a local landmark morph from cached trial files; never calls a provider."""

import argparse
import json
from hashlib import sha256
from pathlib import Path

import cv2
import numpy as np
from PIL import Image

OUTER = [61, 185, 40, 39, 37, 0, 267, 269, 270, 409, 291, 375, 321, 405, 314, 17, 84, 181, 91, 146]
INNER = [78, 191, 80, 81, 82, 13, 312, 311, 310, 415, 308, 324, 318, 402, 317, 14, 87, 178, 88, 95]


def export(run: Path, output: Path) -> None:
    report = json.loads((run / "run.json").read_text(encoding="utf-8"))
    base = np.array(json.loads((run / "base-landmarks.json").read_text())["landmarks_px"])[:, :2]
    variant = np.array(json.loads((run / "variant-landmarks.json").read_text())["landmarks_px"])[:, :2]
    transform = np.array(report["metrics"]["transform"])
    variant = variant @ transform[:, :2].T + transform[:, 2]
    crop = report["overlay"]
    w, h = crop["width"], crop["height"]
    boundary = np.array([[0, 0], [w/2, 0], [w, 0], [w, h/2], [w, h], [w/2, h], [0, h], [0, h/2]])
    rest = np.vstack([base[OUTER + INNER] - [crop["x"], crop["y"]], boundary])
    opened = np.vstack([variant[OUTER + INNER] - [crop["x"], crop["y"]], boundary])
    original = rest.copy()
    # A closed lip mesh may cross itself by fractions of a pixel. Keep its
    # upper/lower samples ordered instead of stretching inverted triangles.
    for i in range(1, 10):
        upper, lower = 20 + i, 40 - i
        center = (rest[upper] + rest[lower]) / 2
        gap = max(0.75, rest[lower, 1] - rest[upper, 1])
        rest[upper] = [center[0], center[1] - gap/2]
        rest[lower] = [center[0], center[1] + gap/2]
    average = (rest + opened) / 2
    # Delaunay only for the surrounding skin. The lip rings and cavity use
    # explicit connections, so triangulation cannot bridge across the lips.
    outside_ids = list(range(20)) + list(range(40, 48))
    subdivision = cv2.Subdiv2D((-1, -1, w + 2, h + 2))
    for i in outside_ids:
        subdivision.insert(tuple(average[i]))
    triangles = []
    for triangle in subdivision.getTriangleList().reshape(-1, 3, 2):
        ids = [min(outside_ids, key=lambda i: np.linalg.norm(average[i] - point)) for point in triangle]
        if max(np.linalg.norm(average[i] - point) for i, point in zip(ids, triangle)) < 0.01:
            if cv2.pointPolygonTest(average[:20].astype(np.float32), tuple(triangle.mean(axis=0)), False) < 0:
                triangles.append(ids)
    for i in range(20):
        j = (i + 1) % 20
        triangles.extend([[i, j, 20+i], [j, 20+j, 20+i]])
    cavity_start = len(triangles)
    triangles.extend([[20, 21, 39], [29, 30, 31]])
    for i in range(1, 9):
        triangles.extend([[20+i, 21+i, 40-i], [21+i, 39-i, 40-i]])

    def area(points):
        vertices = points[np.array(triangles)]
        u, v = vertices[:, 1] - vertices[:, 0], vertices[:, 2] - vertices[:, 0]
        return u[:, 0] * v[:, 1] - u[:, 1] * v[:, 0]

    signs = np.sign(area(average))
    c, mid, end = area(rest)*signs, area(average)*signs, area(opened)*signs
    a = 2*(end + c - 2*mid)
    b = end - c - a
    minimum = float(min(c.min(), end.min()))
    for aa, bb, cc in zip(a, b, c):
        if aa > 0 and 0 < -bb/(2*aa) < 1:
            t = -bb/(2*aa)
            minimum = min(minimum, float(aa*t*t + bb*t + cc))
    if minimum <= 0 or abs(float(sum(abs(area(average))))/2 - w*h) > 0.01:
        raise ValueError("The cached sample cannot form a non-inverting mouth mesh.")

    output.mkdir(parents=True, exist_ok=True)
    # The opaque texture avoids losing lip RGB through browser premultiplication.
    # Alpha remains in the original mouth layer, sampled in fixed canvas space.
    with Image.open(run / crop["file"]) as mouth:
        mouth.convert("RGB").save(output / "mouth-texture.webp", lossless=True, exact=True, method=6)
    mesh = {
        "source_sha256": report["source_sha256"],
        "overlay_sha256": sha256((run / crop["lossless_webp"]).read_bytes()).hexdigest(),
        "width": w, "height": h,
        "rest": rest.round(6).tolist(), "open": opened.round(6).tolist(), "triangles": triangles,
        "cavity_triangles": list(range(cavity_start, len(triangles))),
        "landmark_ids": OUTER + INNER,
        "minimum_signed_double_area": minimum,
        "maximum_rest_regularization_px": float(np.linalg.norm(rest-original, axis=1).max()),
    }
    (output / "mouth-mesh.json").write_text(json.dumps(mesh, indent=2) + "\n", encoding="utf-8")
    print(f"Exported {len(rest)} vertices and {len(triangles)} triangles; no provider request.")


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--run", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    export(args.run.resolve(), args.output.resolve())
