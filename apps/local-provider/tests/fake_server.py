from __future__ import annotations

import argparse
import asyncio
import json
import socket

import uvicorn
from connectrpc.code import Code

from synthify.localprovider.backend import (
    BackendCapabilities,
    BackendError,
    GenerationResult,
    ModelInfo,
)
from synthify.localprovider.server import create_application
from synthify.localprovider.v1 import provider_pb2 as provider


MODEL_ID = "antigravity:test-model"


class DeterministicBackend:
    def __init__(self) -> None:
        self._active: dict[str, asyncio.Event] = {}
        self._lock = asyncio.Lock()
        self._capabilities = BackendCapabilities(
            server_version="cross-language-test",
            default_model_id=MODEL_ID,
            models=(ModelInfo(id=MODEL_ID),),
        )

    async def capabilities(self) -> BackendCapabilities:
        return self._capabilities

    async def generate_text(
        self,
        generation_id: str,
        model_id: str,
        system_prompt: str,
        user_prompt: str,
    ) -> GenerationResult:
        del system_prompt
        if user_prompt == "rate-limit":
            raise BackendError(
                Code.RESOURCE_EXHAUSTED,
                provider.LocalProviderErrorDetail.REASON_RATE_LIMITED,
                retry_after_ms=250,
            )
        if user_prompt == "wait-for-cancel":
            cancelled = asyncio.Event()
            async with self._lock:
                self._active[generation_id] = cancelled
            print(f"STARTED {generation_id}", flush=True)
            try:
                await asyncio.wait_for(cancelled.wait(), timeout=10)
            except TimeoutError as error:
                raise BackendError.internal(turn_started=True) from error
            raise BackendError(
                Code.CANCELED,
                provider.LocalProviderErrorDetail.REASON_INTERNAL,
                turn_started=True,
            )
        return GenerationResult(
            content=f"text:{user_prompt}",
            model=model_id,
            input_tokens=3,
            output_tokens=2,
        )

    async def generate_structured(
        self,
        generation_id: str,
        model_id: str,
        system_prompt: str,
        user_prompt: str,
        json_schema: bytes,
    ) -> GenerationResult:
        del generation_id, system_prompt, user_prompt
        json.loads(json_schema)
        return GenerationResult(
            content=b'{"answer":"ok"}',
            model=model_id,
            input_tokens=5,
            output_tokens=3,
        )

    async def cancel(self, generation_id: str) -> bool:
        async with self._lock:
            cancelled = self._active.pop(generation_id, None)
        if cancelled is None:
            return False
        cancelled.set()
        print(f"CANCELLED {generation_id}", flush=True)
        return True

    async def close(self) -> None:
        async with self._lock:
            active = list(self._active.values())
            self._active.clear()
        for cancelled in active:
            cancelled.set()


async def serve(args: argparse.Namespace, token: str) -> None:
    backend = DeterministicBackend()
    application = create_application(
        backend,
        token,
        max_generation_seconds=10,
    )
    listener = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    listener.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
    listener.bind((args.host, args.port))
    listener.listen(socket.SOMAXCONN)
    print(f"READY {listener.getsockname()[1]}", flush=True)
    config = uvicorn.Config(
        application,
        log_level="error",
        access_log=False,
    )
    server = uvicorn.Server(config)
    try:
        await server.serve(sockets=[listener])
    finally:
        await backend.close()


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--host", default="127.0.0.1")
    parser.add_argument("--port", type=int, default=0)
    parser.add_argument("--token-file", required=True)
    args = parser.parse_args()

    with open(args.token_file, encoding="utf-8") as token_file:
        token = token_file.read().strip()
    asyncio.run(serve(args, token))


if __name__ == "__main__":
    main()
