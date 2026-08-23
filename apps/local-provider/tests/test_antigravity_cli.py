from __future__ import annotations

import asyncio
import json
import os
import stat
import tempfile
import textwrap
import unittest
from pathlib import Path

from connectrpc.code import Code

from synthify.localprovider.antigravity_cli import AntigravityCLIBackend
from synthify.localprovider.backend import BackendError


FAKE_AGY = r"""
#!/usr/bin/env python3
import json
import os
import sys
import time

if "--version" in sys.argv:
    print(os.environ.get("FAKE_AGY_VERSION", "1.1.18"))
    raise SystemExit(0)

if "models" in sys.argv:
    print(json.dumps({
        "status": "SUCCESS",
        "response": "test-model\tTest Model\nother-model\tOther Model\n",
        "usage": {},
    }))
    raise SystemExit(0)

message = json.loads(sys.stdin.readline())
prompt = message["message"]["content"]
capture_path = os.environ.get("FAKE_AGY_CAPTURE")
if capture_path:
    with open(capture_path, "w", encoding="utf-8") as capture:
        json.dump({"argv": sys.argv[1:], "prompt": prompt, "cwd": os.getcwd()}, capture)

print(json.dumps({"event": "init", "init": {"cwd": os.getcwd()}}), flush=True)
print(json.dumps({
    "event": "step_update",
    "step_update": {"step_type": "user_input", "state": "DONE"},
}), flush=True)

if "wait-for-cancel" in prompt:
    time.sleep(30)

if "malformed-stream" in prompt:
    print("{", flush=True)
    time.sleep(30)

if "quota-error" in prompt:
    print(json.dumps({
        "event": "result",
        "result": {
            "status": "ERROR",
            "response": "",
            "error": "Individual quota exhausted",
            "usage": {"input_tokens": 0, "output_tokens": 0},
        },
    }), flush=True)
    raise SystemExit(1)

result = {
    "status": "SUCCESS",
    "response": "generated text",
    "usage": {"input_tokens": 7, "output_tokens": 3},
}
if "--json-schema" in sys.argv:
    result["response"] = "{\"answer\":\"ok\"}"
    result["structured_output"] = {"answer": "ok"}
    if "invalid-structured" in prompt:
        result["structured_output"] = {"answer": 42}
print(json.dumps({"event": "result", "result": result}), flush=True)
"""


class AntigravityCLIBackendTest(unittest.IsolatedAsyncioTestCase):
    def setUp(self) -> None:
        self.temporary_directory = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary_directory.cleanup)
        self.directory = Path(self.temporary_directory.name)
        self.executable = self.directory / "agy"
        self.executable.write_text(textwrap.dedent(FAKE_AGY).lstrip())
        self.executable.chmod(self.executable.stat().st_mode | stat.S_IXUSR)
        self.capture = self.directory / "capture.json"
        self.previous_capture = os.environ.get("FAKE_AGY_CAPTURE")
        os.environ["FAKE_AGY_CAPTURE"] = str(self.capture)
        self.addCleanup(self._restore_environment)

    def _restore_environment(self) -> None:
        if self.previous_capture is None:
            os.environ.pop("FAKE_AGY_CAPTURE", None)
        else:
            os.environ["FAKE_AGY_CAPTURE"] = self.previous_capture

    async def backend(self) -> AntigravityCLIBackend:
        return await AntigravityCLIBackend.create(
            str(self.executable),
            "test-model",
            generation_timeout_seconds=5,
        )

    async def test_discovers_models_and_keeps_prompt_out_of_argv(self) -> None:
        backend = await self.backend()
        capabilities = await backend.capabilities()
        self.assertEqual(capabilities.default_model_id, "antigravity:test-model")
        self.assertEqual(
            [model.id for model in capabilities.models],
            ["antigravity:test-model", "antigravity:other-model"],
        )

        result = await backend.generate_text(
            "generation-text",
            "antigravity:test-model",
            "system secret",
            "user secret",
        )
        self.assertEqual(result.content, "generated text")
        self.assertEqual((result.input_tokens, result.output_tokens), (7, 3))
        capture = json.loads(self.capture.read_text())
        self.assertNotIn("system secret", " ".join(capture["argv"]))
        self.assertNotIn("user secret", " ".join(capture["argv"]))
        self.assertIn("System instructions:\nsystem secret", capture["prompt"])
        self.assertIn("User request:\nuser secret", capture["prompt"])
        self.assertNotEqual(capture["cwd"], os.getcwd())

    async def test_structured_output_is_validated(self) -> None:
        backend = await self.backend()
        result = await backend.generate_structured(
            "generation-structured",
            "antigravity:test-model",
            "system",
            "user",
            b'{"type":"object","properties":{"answer":{"type":"string"}},"required":["answer"]}',
        )
        self.assertEqual(result.content, b'{"answer":"ok"}')

    async def test_rejects_invalid_schema_before_starting_generation(self) -> None:
        backend = await self.backend()
        for schema in (b'{"type":"not-a-json-schema-type"}', b"1"):
            with self.subTest(schema=schema), self.assertRaises(BackendError) as raised:
                await backend.generate_structured(
                    "generation-invalid-schema",
                    "antigravity:test-model",
                    "",
                    "must-not-run",
                    schema,
                )
            self.assertEqual(raised.exception.code, Code.INVALID_ARGUMENT)
        self.assertFalse(self.capture.exists())

    async def test_rejects_structured_output_that_misses_the_schema(self) -> None:
        backend = await self.backend()
        with self.assertRaises(BackendError) as raised:
            await backend.generate_structured(
                "generation-invalid-output",
                "antigravity:test-model",
                "",
                "invalid-structured",
                b'{"type":"object","properties":{"answer":{"type":"string"}},"required":["answer"]}',
            )
        self.assertEqual(raised.exception.code, Code.INTERNAL)
        self.assertTrue(raised.exception.turn_started)

    async def test_quota_error_is_typed_without_raw_message(self) -> None:
        backend = await self.backend()
        with self.assertRaises(BackendError) as raised:
            await backend.generate_text(
                "generation-quota",
                "antigravity:test-model",
                "",
                "quota-error",
            )
        self.assertEqual(raised.exception.code, Code.RESOURCE_EXHAUSTED)
        self.assertNotIn("quota", str(raised.exception).lower())

    async def test_cancel_stops_the_matching_process(self) -> None:
        backend = await self.backend()
        task = asyncio.create_task(
            backend.generate_text(
                "generation-cancel",
                "antigravity:test-model",
                "",
                "wait-for-cancel",
            )
        )
        for _ in range(100):
            if await backend.cancel("generation-cancel"):
                break
            await asyncio.sleep(0.01)
        else:
            self.fail("generation process did not become active")
        with self.assertRaises(BackendError):
            await asyncio.wait_for(task, timeout=2)
        self.assertFalse(await backend.cancel("generation-cancel"))

    async def test_timeout_stops_the_process(self) -> None:
        backend = await AntigravityCLIBackend.create(
            str(self.executable),
            "test-model",
            generation_timeout_seconds=0.1,
        )
        with self.assertRaises(BackendError) as raised:
            await asyncio.wait_for(
                backend.generate_text(
                    "generation-timeout",
                    "antigravity:test-model",
                    "",
                    "wait-for-cancel",
                ),
                timeout=2,
            )
        self.assertEqual(raised.exception.code, Code.DEADLINE_EXCEEDED)
        self.assertFalse(await backend.cancel("generation-timeout"))

    async def test_malformed_stream_stops_the_process(self) -> None:
        backend = await self.backend()
        with self.assertRaises(BackendError):
            await asyncio.wait_for(
                backend.generate_text(
                    "generation-malformed",
                    "antigravity:test-model",
                    "",
                    "malformed-stream",
                ),
                timeout=2,
            )
        self.assertFalse(await backend.cancel("generation-malformed"))

    async def test_concurrency_limit_queues_later_generation(self) -> None:
        backend = await AntigravityCLIBackend.create(
            str(self.executable),
            "test-model",
            generation_timeout_seconds=5,
            max_concurrent_generations=1,
        )
        first = asyncio.create_task(
            backend.generate_text(
                "generation-first",
                "antigravity:test-model",
                "",
                "wait-for-cancel",
            )
        )
        for _ in range(100):
            if self.capture.exists():
                break
            await asyncio.sleep(0.01)
        else:
            self.fail("first generation process did not start")

        second = asyncio.create_task(
            backend.generate_text(
                "generation-second",
                "antigravity:test-model",
                "",
                "ordinary-request",
            )
        )
        await asyncio.sleep(0.05)
        self.assertFalse(second.done())
        self.assertFalse(await backend.cancel("generation-second"))
        self.assertTrue(await backend.cancel("generation-first"))
        with self.assertRaises(BackendError):
            await first
        self.assertEqual((await second).content, "generated text")

    async def test_rejects_old_cli_version(self) -> None:
        previous = os.environ.get("FAKE_AGY_VERSION")
        os.environ["FAKE_AGY_VERSION"] = "1.1.14"
        try:
            with self.assertRaisesRegex(ValueError, "1.1.15"):
                await self.backend()
        finally:
            if previous is None:
                os.environ.pop("FAKE_AGY_VERSION", None)
            else:
                os.environ["FAKE_AGY_VERSION"] = previous


if __name__ == "__main__":
    unittest.main()
