"""Read the exact Cloudflare zone IDs needed by the explicitly invoked rollout."""

import json
from urllib.error import URLError
from urllib.parse import urlencode
from urllib.request import Request, urlopen


def zone_identity(token, name="syntelle.online"):
    query = urlencode({"name": name, "status": "active", "per_page": 5})
    request = Request(
        f"https://api.cloudflare.com/client/v4/zones?{query}",
        headers={"Authorization": f"Bearer {token}", "Accept": "application/json"},
    )
    try:
        with urlopen(request, timeout=15) as response:
            payload = json.loads(response.read())
    except (OSError, URLError, json.JSONDecodeError):
        raise SystemExit("Cloudflare zone lookup failed; check the token and network.") from None
    zones = payload.get("result", []) if payload.get("success") else []
    exact = [zone for zone in zones if zone.get("name") == name]
    if len(exact) != 1:
        raise SystemExit("Cloudflare must return exactly one active syntelle.online zone.")
    zone = exact[0]
    account_id = zone.get("account", {}).get("id", "")
    zone_id = zone.get("id", "")
    if not account_id or not zone_id:
        raise SystemExit("Cloudflare did not return the account and zone IDs.")
    return account_id, zone_id
