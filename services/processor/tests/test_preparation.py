import base64
import json
from pathlib import Path
from types import SimpleNamespace

import pytest
from PIL import Image, ImageDraw

np = pytest.importorskip("numpy", reason="Install the preparation extra for local image-processing tests.")
pytest.importorskip("cv2")
pytest.importorskip("mediapipe")

from app.preparation.faces import FaceDetector, FaceGeometry, PreparationError
from app.preparation.gemini import build_a_prompt, generate_a
from app.preparation.mouth import STABLE_ANCHORS, compose_mouth, export_overlay, fit_similarity, lip_indices, make_mouth_mask, transform_points
from app.viseme_trial import save_provider_image


def face(open_mouth=False):
    points = np.zeros((478, 3), dtype=float)
    for index, (x, y) in zip(STABLE_ANCHORS, [(70, 100), (100, 100), (250, 100), (220, 100), (160, 125), (160, 130), (158, 135), (162, 140), (160, 55), (95, 80), (225, 80), (60, 135), (260, 135)]):
        points[index, :2] = [x, y]
    for index, angle in zip(lip_indices(), np.linspace(0, 2 * np.pi, len(lip_indices()), endpoint=False)):
        points[index, :2] = [160 + 40 * np.cos(angle), 205 + (20 if open_mouth else 9) * np.sin(angle)]
    points[61, :2], points[291, :2] = [120, 205], [200, 205]
    points[13, :2], points[14, :2] = [160, 198 if open_mouth else 205], [160, 217 if open_mouth else 205.5]
    points[2, :2], points[152, :2] = [160, 165], [160, 280]
    return FaceGeometry(points, {}, [])


def test_similarity_recovers_rotation_scale_and_translation():
    source = face().points[list(STABLE_ANCHORS), :2]
    angle = 0.05
    rotation = np.array([[np.cos(angle), -np.sin(angle)], [np.sin(angle), np.cos(angle)]])
    target = source @ rotation.T * 1.03 + [4, -3]
    transform, residual = fit_similarity(source, target)
    np.testing.assert_allclose(transform_points(source, transform), target, atol=1e-8)
    assert residual < 1e-8


def test_degenerate_alignment_fails():
    with pytest.raises(PreparationError, match="cannot be aligned"):
        fit_similarity(np.zeros((5, 2)), np.zeros((5, 2)))


def test_mask_includes_changed_lips_and_excludes_nose_and_eyes():
    base, variant = face(), face(True)
    mask = np.asarray(make_mouth_mask((320, 400), base.points, variant.points[:, :2]))
    assert mask[205, 160] == 255
    assert mask[217, 160] == 255
    assert mask[165, 160] == 0
    assert mask[100, 100] == 0
    assert mask.min() == 0 and mask.max() == 255
    assert np.any((mask > 0) & (mask < 255))


def test_composition_preserves_every_pixel_outside_mask_and_exports_reusable_overlay(tmp_path):
    base = Image.new("RGB", (320, 400), "#C7B7A8")
    # Deliberate changes everywhere in the generated frame must not escape the mask.
    variant = Image.new("RGB", base.size, "#AD9F95")
    ImageDraw.Draw(variant).ellipse((140, 197, 180, 220), fill="#301922")
    result = compose_mouth(base, variant, face(), face(True))
    original, composed = np.asarray(base), np.asarray(result.image)
    outside = np.asarray(result.mask) == 0
    np.testing.assert_array_equal(original[outside], composed[outside])
    assert np.any(original[~outside] != composed[~outside])
    assert result.metrics["changed_pixels_outside_mask"] == 0
    restored = base.convert("RGBA")
    restored.alpha_composite(result.overlay, dest=result.bounds[:2])
    np.testing.assert_array_equal(np.asarray(restored.convert("RGB")), composed)
    assert result.overlay.size == (result.bounds[2] - result.bounds[0], result.bounds[3] - result.bounds[1])
    export_overlay(result, tmp_path)
    for filename in ["mouth-a.png", "mouth-a.webp"]:
        with Image.open(tmp_path / filename) as overlay:
            np.testing.assert_array_equal(np.asarray(overlay.convert("RGBA")), np.asarray(result.overlay))


def test_closed_mouth_is_not_accepted_as_a_viseme():
    image = Image.new("RGB", (320, 400))
    with pytest.raises(PreparationError, match="does not visibly open"):
        compose_mouth(image, image, face(), face())


def test_major_pose_or_scale_change_is_rejected():
    base, variant = face(), face(True)
    variant.points[:, :2] *= 1.4
    image = Image.new("RGB", (320, 400))
    with pytest.raises(PreparationError, match="changed too much"):
        compose_mouth(image, image, base, variant)


@pytest.mark.parametrize("count,message", [(0, "No face"), (2, "exactly one")])
def test_face_count_is_checked_before_generation(count, message):
    import mediapipe as mp

    detector = FaceDetector.__new__(FaceDetector)
    detector.mp = mp
    detector.landmarker = SimpleNamespace(detect=lambda _: SimpleNamespace(face_landmarks=[[]] * count))
    with pytest.raises(PreparationError, match=message):
        detector.detect(Image.new("RGB", (320, 400)))


def test_prompt_uses_original_a_description_without_modifying_source():
    path = Path(__file__).resolve().parents[3] / "docs/prompts/nano-banana-source.txt"
    original = path.read_bytes()
    prompt = build_a_prompt(path)
    assert "Subtle “A” vowel viseme, as in “father.”" in prompt
    assert "Edit only the mouth area" in prompt
    assert "[VISEME_DESCRIPTION]" not in prompt
    assert path.read_bytes() == original


@pytest.mark.parametrize("mime_type", ["image/jpeg", "image/png", "image/webp"])
def test_provider_gets_exact_base_bytes_once_without_conversation_state(monkeypatch, mime_type):
    import httpx

    original_client = httpx.Client
    calls = []

    def transport(request):
        assert request.headers["x-goog-api-key"] == "private-test-key"
        calls.append(json.loads(request.content))
        return httpx.Response(200, json={
            "id": "test-interaction", "status": "completed",
            "steps": [{"type": "model_output", "content": [{"type": "image", "mime_type": "image/jpeg", "data": base64.b64encode(b"generated-image").decode()}]}],
        })

    def make_client(**kwargs):
        kwargs["transport"] = httpx.MockTransport(transport)
        return original_client(**kwargs)

    monkeypatch.setattr(httpx, "Client", make_client)
    original = b"immutable-approved-image"
    data, metadata = generate_a(original, "A prompt", "private-test-key", "test-model", mime_type)
    assert data == b"generated-image" and metadata["model"] == "test-model"
    assert len(calls) == 1
    assert base64.b64decode(calls[0]["input"][1]["data"]) == original
    assert calls[0]["input"][1]["mime_type"] == mime_type
    assert calls[0]["store"] is False
    assert calls[0]["response_format"]["mime_type"] == "image/jpeg"
    assert "previous_interaction_id" not in calls[0]


def test_provider_failure_does_not_leak_secret_or_retry(monkeypatch):
    import httpx

    def fail(**_):
        raise httpx.ConnectError("private-test-key should not be disclosed")

    monkeypatch.setattr(httpx, "Client", fail)
    with pytest.raises(PreparationError) as error:
        generate_a(b"image", "prompt", "private-test-key", "model")
    assert "private-test-key" not in str(error.value)


def test_missing_key_fails_before_client_creation(monkeypatch):
    import httpx

    def unexpected(**_):
        pytest.fail("No client should be created without a key.")

    monkeypatch.setattr(httpx, "Client", unexpected)
    with pytest.raises(PreparationError, match="GEMINI_API_KEY"):
        generate_a(b"image", "prompt", "", "model")


def test_provider_transport_does_not_retry_a_server_failure(monkeypatch):
    import httpx

    original_client = httpx.Client
    calls = []

    def transport(request):
        calls.append(request.url.path)
        return httpx.Response(500, json={"error": {"code": 500, "message": "synthetic failure"}})

    def make_client(**kwargs):
        kwargs["transport"] = httpx.MockTransport(transport)
        return original_client(**kwargs)

    monkeypatch.setattr(httpx, "Client", make_client)
    with pytest.raises(PreparationError):
        generate_a(b"immutable-image", "prompt", "private-test-key", "model")
    assert calls == ["/v1beta/interactions"]


@pytest.mark.parametrize("image_format,suffix", [("JPEG", ".jpg"), ("PNG", ".png"), ("WEBP", ".webp")])
def test_provider_cache_preserves_bytes_and_actual_format(tmp_path, image_format, suffix):
    from io import BytesIO

    stream = BytesIO()
    Image.new("RGB", (32, 48), "#BBA3F5").save(stream, format=image_format)
    original = stream.getvalue()
    saved = save_provider_image(original, tmp_path)
    assert saved.suffix == suffix
    assert saved.read_bytes() == original
