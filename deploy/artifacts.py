"""Fetch a hash-pinned private ZIP only during an explicit CI deployment.

The signed URL is an environment secret. No URL or archive contents are logged.
"""

import hashlib
import os
from pathlib import Path, PurePosixPath
import re
import stat
import tempfile
from urllib.parse import urlsplit
from urllib.request import HTTPRedirectHandler, build_opener
import zipfile

LIMIT = 2 * 1024**3


class NoRedirect(HTTPRedirectHandler):
    def redirect_request(self, *args, **kwargs):
        raise ValueError("Private artifact redirects are disabled.")


def extract_bundle(archive, destination, expected_hash):
    if not re.fullmatch(r"[a-f0-9]{64}", expected_hash):
        raise ValueError("Provide the private archive SHA-256.")
    with open(archive, "rb") as source:
        actual = hashlib.file_digest(source, "sha256").hexdigest()
    if actual != expected_hash:
        raise ValueError("Private artifact hash mismatch.")
    root = Path(destination).resolve()
    if root.exists():
        raise ValueError("Private artifact destination must be new.")
    with zipfile.ZipFile(archive) as bundle:
        members = bundle.infolist()
        if sum(entry.file_size for entry in members) > LIMIT or len(members) > 20000:
            raise ValueError("Private artifact exceeds the extraction bound.")
        names = set()
        for entry in members:
            path = PurePosixPath(entry.filename)
            if (
                not entry.filename
                or path.is_absolute()
                or ".." in path.parts
                or "\\" in entry.orig_filename
                or ":" in entry.filename
                or stat.S_ISLNK(entry.external_attr >> 16)
                or path.as_posix() in names
            ):
                raise ValueError("Unsafe private artifact member.")
            names.add(path.as_posix())
        root.mkdir(parents=True, mode=0o700)
        for entry in members:
            target = root.joinpath(*PurePosixPath(entry.filename).parts)
            if entry.is_dir():
                target.mkdir(parents=True, exist_ok=True, mode=0o700)
            else:
                target.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
                with bundle.open(entry) as source, target.open("xb") as output:
                    while chunk := source.read(1024 * 1024):
                        output.write(chunk)
                target.chmod(0o600)


def main():
    url = os.environ["BUNDLE_URL"]
    parsed = urlsplit(url)
    if (
        parsed.scheme != "https"
        or not parsed.hostname
        or not parsed.hostname.endswith(".blob.core.windows.net")
        or parsed.username
        or parsed.port
    ):
        raise SystemExit("Private artifact must use an HTTPS Azure Blob URL.")
    # Keep temporary bytes and extraction outside source/evidence directories.
    private = Path(".deploy-private")
    private.mkdir(mode=0o700, exist_ok=True)
    with tempfile.TemporaryDirectory(dir=private) as temporary:
        archive = Path(temporary) / "bundle.zip"
        try:
            with (
                build_opener(NoRedirect()).open(url, timeout=60) as source,
                archive.open("xb") as output,
            ):
                total = 0
                while chunk := source.read(1024 * 1024):
                    total += len(chunk)
                    if total > LIMIT:
                        raise ValueError("Private artifact exceeds the download bound.")
                    output.write(chunk)
        except Exception:
            # urllib errors can contain signed URLs; never print their details.
            raise SystemExit("Private artifact download failed.") from None
        extract_bundle(archive, private / "release", os.environ["BUNDLE_SHA256"])


if __name__ == "__main__":
    main()
