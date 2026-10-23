"""Generate small synthetic test images; no portraits or reference assets are used."""

from pathlib import Path

from PIL import Image

destination = Path(__file__).resolve().parent
image = Image.new("RGB", (32, 48), "#0F766E")
for image_format, extension in [("PNG", "png"), ("JPEG", "jpg"), ("WEBP", "webp"), ("GIF", "gif")]:
    image.save(destination / f"image.{extension}", format=image_format)
exif = Image.Exif()
exif[274] = 6
image.save(destination / "rotated.jpg", format="JPEG", exif=exif)
(destination / "corrupt.png").write_bytes(b"\x89PNG\r\n\x1a\n")
