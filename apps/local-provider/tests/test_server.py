from __future__ import annotations

import asyncio
import unittest
from types import SimpleNamespace

from connectrpc.code import Code
from connectrpc.errors import ConnectError

from synthify.localprovider.backend import (
    BackendCapabilities,
    GenerationResult,
    ModelInfo,
)
from synthify.localprovider.server import LocalProviderService
from synthify.localprovider.v1 import provider_pb2 as provider


class ControlledBackend:
    def __init__(self, *, failure: Exception | None = None) -> None:
        self.failure = failure
        self.started = asyncio.Event()
        self.operation_cancelled = asyncio.Event()
        self.cancelled_generation_ids: list[str] = []

    async def capabilities(self) -> BackendCapabilities:
        return BackendCapabilities(
            server_version="test",
            default_model_id="antigravity:test-model",
            models=(ModelInfo(id="antigravity:test-model"),),
        )

    async def generate_text(
        self,
        generation_id: str,
        model_id: str,
        system_prompt: str,
        user_prompt: str,
    ) -> GenerationResult:
        del generation_id, model_id, system_prompt, user_prompt
        self.started.set()
        if self.failure is not None:
            raise self.failure
        try:
            await asyncio.Event().wait()
        except asyncio.CancelledError:
            self.operation_cancelled.set()
            raise
        raise AssertionError("unreachable")

    async def generate_structured(
        self,
        generation_id: str,
        model_id: str,
        system_prompt: str,
        user_prompt: str,
        json_schema: bytes,
    ) -> GenerationResult:
        del generation_id, model_id, system_prompt, user_prompt, json_schema
        raise AssertionError("not used")

    async def cancel(self, generation_id: str) -> bool:
        self.cancelled_generation_ids.append(generation_id)
        return True

    async def close(self) -> None:
        return None


def text_request(generation_id: str) -> provider.GenerateTextRequest:
    return provider.GenerateTextRequest(
        generation_id=generation_id,
        model_id="antigravity:test-model",
        user_prompt="test",
    )


class LocalProviderServiceTest(unittest.IsolatedAsyncioTestCase):
    async def test_watchdog_cancels_backend_after_bounded_timeout(self) -> None:
        backend = ControlledBackend()
        service = LocalProviderService(backend, max_generation_seconds=0.02)

        with self.assertRaises(ConnectError) as raised:
            await service.generate_text(
                text_request("generation-timeout"),
                SimpleNamespace(timeout_ms=None),
            )

        self.assertEqual(raised.exception.code, Code.DEADLINE_EXCEEDED)
        self.assertTrue(backend.operation_cancelled.is_set())
        self.assertEqual(backend.cancelled_generation_ids, ["generation-timeout"])

    async def test_request_task_cancellation_cancels_backend(self) -> None:
        backend = ControlledBackend()
        service = LocalProviderService(backend, max_generation_seconds=10)
        task = asyncio.create_task(
            service.generate_text(
                text_request("generation-disconnect"),
                SimpleNamespace(timeout_ms=None),
            )
        )
        await asyncio.wait_for(backend.started.wait(), timeout=1)

        task.cancel()
        with self.assertRaises(asyncio.CancelledError):
            await task

        self.assertTrue(backend.operation_cancelled.is_set())
        self.assertEqual(backend.cancelled_generation_ids, ["generation-disconnect"])

    async def test_unexpected_backend_error_is_redacted_and_cancelled(self) -> None:
        backend = ControlledBackend(failure=RuntimeError("secret provider payload"))
        service = LocalProviderService(backend, max_generation_seconds=1)

        with self.assertRaises(ConnectError) as raised:
            await service.generate_text(
                text_request("generation-error"),
                SimpleNamespace(timeout_ms=None),
            )

        self.assertEqual(raised.exception.code, Code.INTERNAL)
        self.assertNotIn("secret provider payload", raised.exception.message)
        self.assertEqual(backend.cancelled_generation_ids, ["generation-error"])


if __name__ == "__main__":
    unittest.main()
