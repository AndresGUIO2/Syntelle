from io import BytesIO
import os
import hmac
from typing import Annotated, Literal

from fastapi import FastAPI, File, Request, UploadFile
from fastapi.exceptions import RequestValidationError
from fastapi.responses import JSONResponse
from PIL import Image, ImageOps, UnidentifiedImageError
from pydantic import BaseModel

MAX_IMAGE_BYTES = 5_000_000
MIME_TYPES = {"JPEG": "image/jpeg", "PNG": "image/png", "WEBP": "image/webp"}

app = FastAPI(title="Syntelle image processor", version="0.1.0")

@app.middleware("http")
async def service_auth(request: Request, call_next):
    if os.environ.get("SYNTELLE_DEPLOYMENT_MODE") == "cloud" and request.url.path != "/healthz":
        token = os.environ.get("SYNTELLE_PROCESSOR_TOKEN", "")
        if len(token) < 32 or not hmac.compare_digest(request.headers.get("Authorization", ""), "Bearer " + token):
            return JSONResponse({"error": "Service authorization required"}, status_code=403)
    return await call_next(request)


class ImageResult(BaseModel):
    format: Literal["JPEG", "PNG", "WEBP"]
    mime_type: str
    width: int
    height: int
    size_bytes: int


class ImageError(Exception):
    def __init__(self, status: int, code: str, message: str):
        self.status = status
        self.code = code
        self.message = message


@app.exception_handler(ImageError)
async def image_error_handler(_: Request, exc: ImageError) -> JSONResponse:
    return JSONResponse(
        status_code=exc.status,
        content={"error": {"code": exc.code, "message": exc.message}},
        headers={"Cache-Control": "no-store"},
    )


@app.exception_handler(RequestValidationError)
async def request_error_handler(_: Request, exc: RequestValidationError) -> JSONResponse:
    return JSONResponse(
        status_code=400,
        content={"error": {"code": "invalid_request", "message": "Upload one multipart image named image."}},
    )


@app.get("/healthz")
def health() -> dict[str, str]:
    return {"status": "ok"}


def inspect_image(data: bytes) -> ImageResult:
    if len(data) > MAX_IMAGE_BYTES:
        raise ImageError(413, "file_too_large", "Choose an image no larger than 5,000,000 bytes.")
    if not data:
        raise ImageError(400, "empty_file", "The selected file is empty.")
    try:
        with Image.open(BytesIO(data)) as image:
            image_format = image.format
            if image_format not in MIME_TYPES:
                raise ImageError(415, "unsupported_format", "Choose a JPEG, PNG, or WebP image.")
            image.verify()
        # verify() checks structure; a fresh decoder must also load all pixel data.
        with Image.open(BytesIO(data)) as image:
            image.load()
            ImageOps.exif_transpose(image, in_place=True)
            width, height = image.size
    except Image.DecompressionBombError as exc:
        raise ImageError(422, "image_dimensions_too_large", "Choose an image with smaller dimensions.") from exc
    except UnidentifiedImageError as exc:
        recognizable = data.startswith((b"\xff\xd8", b"\x89PNG\r\n\x1a\n")) or (
            data.startswith(b"RIFF") and data[8:12] == b"WEBP"
        )
        if recognizable:
            raise ImageError(422, "corrupt_image", "This image is damaged or incomplete.") from exc
        raise ImageError(415, "unsupported_format", "Choose a JPEG, PNG, or WebP image.") from exc
    except (OSError, SyntaxError, ValueError) as exc:
        raise ImageError(422, "corrupt_image", "This image is damaged or incomplete.") from exc
    return ImageResult(
        format=image_format,
        mime_type=MIME_TYPES[image_format],
        width=width,
        height=height,
        size_bytes=len(data),
    )


@app.post("/internal/v1/images/validate")
def validate_image(image: Annotated[UploadFile, File()]) -> ImageResult:
    try:
        return inspect_image(image.file.read(MAX_IMAGE_BYTES + 1))
    finally:
        image.file.close()
