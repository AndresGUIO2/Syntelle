import base64
import re
from time import perf_counter
from hashlib import sha256
from pathlib import Path

from .faces import PreparationError

DEFAULT_MODEL = "gemini-nano-banana-2.1"
VISEMES = ("A", "EI", "O", "U", "MBP", "SZ", "FV", "CHSHR")
REQUEST_SETTINGS = {"type": "image", "mime_type": "image/jpeg", "aspect_ratio": "3:4", "image_size": "1K"}


def build_a_prompt(path: Path) -> str:
    return build_prompt(path, "A")


def build_prompt(path: Path, viseme: str) -> str:
    if viseme not in VISEMES:
        raise PreparationError("Unknown viseme identifier.")
    source = path.read_text(encoding="utf-8")
    descriptions = re.split(r"\n\s*\n", source.split("[VISEME_DESCRIPTION]", 1)[1].split("---- GENERAL PROMPT", 1)[0].strip())
    if len(descriptions) != len(VISEMES):
        raise PreparationError("The supplied viseme inventory changed; check the original source.")
    description = descriptions[VISEMES.index(viseme)].strip()
    general = source.split("---- GENERAL PROMPT", 1)[1].split("---- medio PARPADEO", 1)[0].strip()
    if "Edit only the mouth area" not in general:
        raise PreparationError("The supplied prompt layout changed; check the original source.")
    # Preserve the source file and its wording; only fill the supplied placeholder.
    return general.replace("[VISEME_DESCRIPTION]", description)


def generate_a(base_bytes: bytes, prompt: str, api_key: str, model: str, mime_type: str = "image/jpeg", *, supporting_images: tuple[tuple[bytes, str], ...] = ()) -> tuple[bytes, dict]:
    import httpx

    if not api_key.strip():
        raise PreparationError("Set GEMINI_API_KEY in the local .env file before generating.")
    payload = {
        "model": model,
        "input": [
            {"type": "text", "text": prompt},
            {"type": "image", "mime_type": mime_type, "data": base64.b64encode(base_bytes).decode("ascii")},
        ],
        "response_format": REQUEST_SETTINGS.copy(),
        "store": False,
    }
    for data, image_mime in supporting_images:
        payload["input"].append({"type": "image", "mime_type": image_mime, "data": base64.b64encode(data).decode("ascii")})
    started = perf_counter()
    try:
        # REST avoids SDK-internal retries; exactly one POST with no redirects.
        with httpx.Client(timeout=180.0, transport=httpx.HTTPTransport(retries=0), follow_redirects=False) as client:
            response = client.post(
                "https://generativelanguage.googleapis.com/v1beta/interactions",
                headers={"x-goog-api-key": api_key}, json=payload,
            )
    except httpx.RequestError:
        raise PreparationError("Gemini connection failed or timed out. No automatic retry was made; check the local run record before retrying.") from None
    try:
        body = response.json()
    except ValueError:
        raise PreparationError(f"Gemini returned an invalid response (HTTP {response.status_code}).") from None
    if not response.is_success:
        message = str(body.get("error", {}).get("message", "Request rejected.")).replace(api_key, "[redacted]")[:400]
        raise PreparationError(f"Gemini rejected the request (HTTP {response.status_code}). {message} No automatic retry was made.")
    images = [body["output_image"]] if body.get("output_image") else [
        item for step in body.get("steps", []) if step.get("type") == "model_output"
        for item in step.get("content", []) if item.get("type") == "image" and item.get("data")
    ]
    if len(images) != 1:
        raise PreparationError("Gemini returned no image. No automatic retry was made.")
    image = images[0]
    try:
        data = base64.b64decode(image["data"], validate=True)
    except (ValueError, TypeError):
        raise PreparationError("Gemini returned invalid image data.") from None
    if not data or len(data) > 20_000_000:
        raise PreparationError("Gemini returned an empty or oversized image.")
    return data, {
        "model": model, "resolution": "1K", "provider_mime_type": image.get("mime_type", "image/jpeg"),
        "interaction_id": body.get("id"), "usage": body.get("usage", {}),
        "duration_seconds": round(perf_counter() - started, 3), "prompt_sha256": sha256(prompt.encode()).hexdigest(),
        "request_settings": {"response_format": REQUEST_SETTINGS.copy(), "store": False, "retries": 0, "timeout_seconds": 180,
                             "supporting_images": [{"sha256": sha256(data).hexdigest(), "mime_type": mime} for data, mime in supporting_images]},
    }
