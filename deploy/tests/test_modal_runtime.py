"""Import and invoke the launcher with an offline Modal/FastAPI fixture."""

import asyncio
import importlib.util
import os
from pathlib import Path
import sys
import subprocess
import types
import unittest
from unittest.mock import patch


class Chain:
    def __getattr__(self, name):
        return lambda *a, **k: self


class OfflineModal:
    calls = []

    class Image:
        @staticmethod
        def from_registry(*a, **k):
            return Chain()

        @staticmethod
        def debian_slim(*a, **k):
            return Chain()

    class Secret:
        @staticmethod
        def from_name(*a, **k):
            return None

    class App:
        def __init__(self, *a):
            pass

        def function(self, **configuration):
            def decorate(fn):
                fn.configuration = configuration
                return fn

            return decorate

    @staticmethod
    def fastapi_endpoint(**k):
        return lambda fn: fn


class Response:
    def __init__(self, data, status_code):
        self.status_code = status_code


class ModalRuntimeTests(unittest.TestCase):
    def test_import_and_authenticated_spawn(self):
        class Request:
            def __init__(self, token):
                self.headers = {"Authorization": token}

            async def json(self):
                return {"allocation_id": "allocation_fixture"}

        module = types.ModuleType("offline")
        path = Path(__file__).resolve().parents[1] / "modal_app.py"
        with (
            patch.dict(
                sys.modules,
                {
                    "modal": OfflineModal,
                    "fastapi": types.SimpleNamespace(Request=Request),
                    "fastapi.responses": types.SimpleNamespace(JSONResponse=Response),
                },
            ),
            patch.dict(
                os.environ,
                {
                    "SYNTELLE_RENDER_IMAGE": "fixture@sha256:abc",
                    "SYNTELLE_RENDER_ASSETS": ".",
                    "SYNTELLE_MODAL_LAUNCH_TOKEN": "secret",
                },
            ),
        ):
            exec(compile(path.read_text(), str(path), "exec"), module.__dict__)
            calls = []

            async def spawn(identifier):
                calls.append(identifier)

            module.render.spawn = types.SimpleNamespace(aio=spawn)
            self.assertEqual(
                asyncio.run(module.launch(Request("wrong"))).status_code, 403
            )
            self.assertEqual(calls, [])
            self.assertEqual(
                asyncio.run(module.launch(Request("Bearer secret"))).status_code, 202
            )
            self.assertEqual(calls, ["allocation_fixture"])
            self.assertEqual(module.render.configuration["max_containers"], 1)
            with patch.object(module.subprocess, "run") as native:
                module.render("allocation_fixture")
                arguments = native.call_args
                self.assertEqual(
                    arguments.args[0],
                    [
                        "/usr/local/bin/syntelle-render-worker",
                        "--assets",
                        "/assets",
                        "--capacity",
                        "2",
                    ],
                )
                self.assertEqual(
                    arguments.kwargs["env"]["SYNTELLE_VIDEO_ENCODER"], "nvenc"
                )
                self.assertEqual(
                    arguments.kwargs["env"]["SYNTELLE_RENDER_ALLOCATION_ID"],
                    "allocation_fixture",
                )
            with patch.object(
                module.subprocess,
                "run",
                side_effect=subprocess.CalledProcessError(1, "offline-native"),
            ) as native:
                with self.assertRaises(subprocess.CalledProcessError):
                    module.render("allocation_fixture")
                self.assertEqual(native.call_count, 1)
