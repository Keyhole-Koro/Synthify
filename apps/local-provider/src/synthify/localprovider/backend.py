from __future__ import annotations

from dataclasses import dataclass
from typing import Protocol

from connectrpc.code import Code

from synthify.localprovider.v1 import provider_pb2 as provider


@dataclass(frozen=True)
class ModelInfo:
    id: str
    supports_structured: bool = True


@dataclass(frozen=True)
class BackendCapabilities:
    server_version: str
    default_model_id: str
    models: tuple[ModelInfo, ...]


@dataclass(frozen=True)
class GenerationResult:
    content: str | bytes
    model: str
    input_tokens: int
    output_tokens: int


class BackendError(Exception):
    def __init__(
        self,
        code: Code,
        reason: int,
        *,
        turn_started: bool = False,
        retry_after_ms: int = 0,
    ) -> None:
        super().__init__("local provider backend request failed")
        self.code = code
        self.reason = reason
        self.turn_started = turn_started
        self.retry_after_ms = retry_after_ms

    @classmethod
    def internal(cls, *, turn_started: bool = False) -> BackendError:
        return cls(
            Code.INTERNAL,
            provider.LocalProviderErrorDetail.REASON_INTERNAL,
            turn_started=turn_started,
        )


class Backend(Protocol):
    async def capabilities(self) -> BackendCapabilities: ...

    async def generate_text(
        self,
        generation_id: str,
        model_id: str,
        system_prompt: str,
        user_prompt: str,
    ) -> GenerationResult: ...

    async def generate_structured(
        self,
        generation_id: str,
        model_id: str,
        system_prompt: str,
        user_prompt: str,
        json_schema: bytes,
    ) -> GenerationResult: ...

    async def cancel(self, generation_id: str) -> bool: ...

    async def close(self) -> None: ...
