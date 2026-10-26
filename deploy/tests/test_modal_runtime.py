"""Import and invoke the launcher with an offline Modal/FastAPI fixture."""

import asyncio
import importlib
import os
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
    local = False
    spawn = None
    lookups = []

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

    class Function:
        @staticmethod
        def from_name(app_name, function_name):
            OfflineModal.lookups.append((app_name, function_name))
            return types.SimpleNamespace(
                spawn=types.SimpleNamespace(aio=OfflineModal.spawn)
            )

    @staticmethod
    def fastapi_endpoint(**k):
        return lambda fn: fn

    @classmethod
    def is_local(cls):
        return cls.local


class Response:
    def __init__(self, data, status_code):
        self.status_code = status_code


class ModalRuntimeTests(unittest.TestCase):
    def test_renderer_module_imports_without_fastapi(self):
        module_names = (
            "deploy.modal_render",
            "deploy.modal_shared",
        )
        for name in module_names:
            sys.modules.pop(name, None)
        with patch.dict(sys.modules, {"modal": OfflineModal, "fastapi": None}):
            importlib.import_module("deploy.modal_render")
        for name in module_names:
            sys.modules.pop(name, None)

    def test_import_and_authenticated_spawn(self):
        class Request:
            def __init__(self, token):
                self.headers = {"Authorization": token}

            async def json(self):
                return {"allocation_id": "allocation_fixture"}

        module_names = (
            "deploy.modal_app",
            "deploy.modal_launcher",
            "deploy.modal_render",
            "deploy.modal_shared",
        )
        for name in module_names:
            sys.modules.pop(name, None)
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
                    "SYNTELLE_MODAL_LAUNCH_TOKEN": "secret",
                },
            ),
        ):
            calls = []

            async def spawn(identifier):
                calls.append(identifier)

            OfflineModal.spawn = spawn
            OfflineModal.lookups = []
            module = importlib.import_module("deploy.modal_app")
            self.assertEqual(
                asyncio.run(module.launch(Request("wrong"))).status_code, 403
            )
            self.assertEqual(calls, [])
            self.assertEqual(OfflineModal.lookups, [])
            self.assertEqual(
                asyncio.run(module.launch(Request("Bearer secret"))).status_code, 202
            )
            self.assertEqual(calls, ["allocation_fixture"])
            self.assertEqual(
                OfflineModal.lookups, [("syntelle-renderer", "render")]
            )
            self.assertEqual(module.render.configuration["max_containers"], 16)
            with patch.object(module.render.__globals__["subprocess"], "run") as native:
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
                        "--video-height",
                        "720",
                    ],
                )
                self.assertEqual(
                    arguments.kwargs["env"]["SYNTELLE_VIDEO_ENCODER"], "nvenc"
                )
                self.assertEqual(
                    arguments.kwargs["env"]["SYNTELLE_GPU_PRECISION"], "fp32"
                )
                self.assertEqual(
                    arguments.kwargs["env"]["SYNTELLE_RENDER_ALLOCATION_ID"],
                    "allocation_fixture",
                )
            with patch.object(
                module.render.__globals__["subprocess"],
                "run",
                side_effect=subprocess.CalledProcessError(1, "offline-native"),
            ) as native:
                with self.assertRaises(subprocess.CalledProcessError):
                    module.render("allocation_fixture")
                self.assertEqual(native.call_count, 1)
        for name in module_names:
            sys.modules.pop(name, None)
