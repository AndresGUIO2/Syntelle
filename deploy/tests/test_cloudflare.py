import json
import unittest
from unittest.mock import patch

from deploy.cloudflare import zone_identity


class CloudflareZoneTests(unittest.TestCase):
    @patch("deploy.cloudflare.urlopen")
    def test_resolves_only_the_exact_active_zone(self, urlopen):
        class Response:
            def __enter__(self):
                return self

            def __exit__(self, *_):
                return False

            def read(self):
                return json.dumps(
                    {
                        "success": True,
                        "result": [
                            {
                                "name": "syntelle.online",
                                "id": "zone-fixture",
                                "account": {"id": "account-fixture"},
                            }
                        ],
                    }
                ).encode()

        urlopen.return_value = Response()
        self.assertEqual(
            zone_identity("fixture-token"), ("account-fixture", "zone-fixture")
        )
        request = urlopen.call_args.args[0]
        self.assertEqual(request.get_header("Authorization"), "Bearer fixture-token")
        self.assertIn("name=syntelle.online", request.full_url)

    @patch("deploy.cloudflare.urlopen")
    def test_rejects_ambiguous_zone_results(self, urlopen):
        class Response:
            def __enter__(self):
                return self

            def __exit__(self, *_):
                return False

            def read(self):
                return json.dumps(
                    {
                        "success": True,
                        "result": [
                            {"name": "syntelle.online", "id": "one", "account": {"id": "a"}},
                            {"name": "syntelle.online", "id": "two", "account": {"id": "b"}},
                        ],
                    }
                ).encode()

        urlopen.return_value = Response()
        with self.assertRaises(SystemExit):
            zone_identity("fixture-token")
