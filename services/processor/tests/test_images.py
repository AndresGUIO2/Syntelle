from io import BytesIO

import pytest
from fastapi import UploadFile
from fastapi.testclient import TestClient
from PIL import Image

from app.main import MAX_IMAGE_BYTES, ImageError, app, validate_image

client = TestClient(app)
ENDPOINT = "/internal/v1/images/validate"


def image_bytes(image_format="PNG", orientation=None):
    output = BytesIO()
    image = Image.new("RGB", (32, 48), "#0F766E")
    options = {}
    if orientation:
        exif = Image.Exif()
        exif[274] = orientation
        options["exif"] = exif
    image.save(output, format=image_format, **options)
    return output.getvalue()


@pytest.mark.parametrize("image_format,mime", [("JPEG", "image/jpeg"), ("PNG", "image/png"), ("WEBP", "image/webp")])
def test_actual_content_wins_over_filename_and_mime(image_format, mime):
    data = image_bytes(image_format)
    response = client.post(ENDPOINT, files={"image": ("misleading.txt", data, "text/plain")})
    assert response.status_code == 200
    assert response.json() == {"format": image_format, "mime_type": mime, "width": 32, "height": 48, "size_bytes": len(data)}


def test_exact_size_boundary():
    data = image_bytes()
    padded = data + b"\0" * (MAX_IMAGE_BYTES - len(data))
    response = client.post(ENDPOINT, files={"image": ("portrait.png", padded)})
    assert response.status_code == 200
    assert response.json()["size_bytes"] == MAX_IMAGE_BYTES
    response = client.post(ENDPOINT, files={"image": ("portrait.png", padded + b"\0")})
    assert response.status_code == 413
    assert response.json()["error"]["code"] == "file_too_large"


@pytest.mark.parametrize("orientation", range(1, 9))
def test_orientation_corrected_dimensions(orientation):
    response = client.post(ENDPOINT, files={"image": ("portrait.jpg", image_bytes("JPEG", orientation))})
    assert response.status_code == 200
    expected = (48, 32) if orientation >= 5 else (32, 48)
    assert (response.json()["width"], response.json()["height"]) == expected


@pytest.mark.parametrize("data", [b"not an image", image_bytes("GIF")])
def test_unsupported_content_disguised_as_png(data):
    response = client.post(ENDPOINT, files={"image": ("portrait.png", data, "image/png")})
    assert response.status_code == 415
    assert response.json()["error"]["code"] == "unsupported_format"


@pytest.mark.parametrize("data", [b"\x89PNG\r\n\x1a\n", image_bytes("JPEG")[:80], image_bytes("PNG")[:50]])
def test_corrupt_image(data):
    response = client.post(ENDPOINT, files={"image": ("portrait.png", data)})
    assert response.status_code == 422
    assert response.json()["error"]["code"] == "corrupt_image"


def test_empty_and_missing_image():
    assert client.post(ENDPOINT, files={"image": ("portrait.png", b"")}).status_code == 400
    assert client.post(ENDPOINT).status_code == 400


@pytest.mark.parametrize("data", [image_bytes(), b"not an image", b"x" * (MAX_IMAGE_BYTES + 1)], ids=["valid", "unsupported", "oversized"])
def test_upload_is_closed_after_success_or_failure(data):
    stream = BytesIO(data)
    upload = UploadFile(file=stream, filename="portrait.png")
    try:
        validate_image(upload)
    except ImageError:
        pass
    assert stream.closed


def test_decoder_pixel_protection(monkeypatch):
    monkeypatch.setattr(Image, "MAX_IMAGE_PIXELS", 1)
    response = client.post(ENDPOINT, files={"image": ("portrait.png", image_bytes())})
    assert response.status_code == 422
    assert response.json()["error"]["code"] == "image_dimensions_too_large"


def test_health():
    assert client.get("/healthz").json() == {"status": "ok"}
