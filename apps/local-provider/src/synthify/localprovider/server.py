from __future__ import annotations

import asyncio
import hmac
import json
from collections.abc import Awaitable, Callable
from typing import Any

from connectrpc.code import Code
from connectrpc.errors import ConnectError, ErrorDetail
from connectrpc.request import RequestContext
from protobuf.wkt import Any as ConnectAny
from protovalidate import ValidationError, Validator

from synthify.localprovider.backend import Backend, BackendError, GenerationResult
from synthify.localprovider.v1 import provider_pb2 as provider
from synthify.localprovider.v1.provider_connect import (
    LocalProviderServiceASGIApplication,
)


MAX_REQUEST_BYTES = 18 << 20


def provider_error(
    code: Code,
    reason: int,
    *,
    turn_started: bool = False,
    retry_after_ms: int = 0,
) -> ConnectError:
    detail = provider.LocalProviderErrorDetail(
        reason=reason,
        turn_started=turn_started,
        retry_after_ms=retry_after_ms,
    )
    wire_detail = ConnectAny(
        type_url=f"type.googleapis.com/{detail.DESCRIPTOR.full_name}",
        value=detail.SerializeToString(),
    )
    return ConnectError(
        code,
        "local provider request failed",
        details=(ErrorDetail(wire_detail),),
    )


class BearerAuthInterceptor:
    def __init__(self, token: str) -> None:
        self._expected = f"Bearer {token}"

    async def intercept_unary(
        self,
        call_next: Callable[[Any, RequestContext], Awaitable[Any]],
        request: Any,
        ctx: RequestContext,
    ) -> Any:
        supplied = ctx.request_headers.get("authorization", "")
        if not hmac.compare_digest(supplied, self._expected):
            raise provider_error(
                Code.UNAUTHENTICATED,
                provider.LocalProviderErrorDetail.REASON_AUTHENTICATION_REQUIRED,
            )
        return await call_next(request, ctx)


class ContractValidationInterceptor:
    def __init__(self) -> None:
        self._validator = Validator()

    async def intercept_unary(
        self,
        call_next: Callable[[Any, RequestContext], Awaitable[Any]],
        request: Any,
        ctx: RequestContext,
    ) -> Any:
        try:
            self._validator.validate(request)
        except ValidationError as error:
            raise ConnectError(Code.INVALID_ARGUMENT, "invalid request") from error

        response = await call_next(request, ctx)
        try:
            self._validator.validate(response)
        except ValidationError as error:
            raise provider_error(
                Code.INTERNAL,
                provider.LocalProviderErrorDetail.REASON_INTERNAL,
                turn_started=True,
            ) from error
        return response


class LocalProviderService:
    def __init__(self, backend: Backend, max_generation_seconds: float) -> None:
        self._backend = backend
        self._max_generation_seconds = max_generation_seconds

    async def check(
        self, request: provider.CheckRequest, ctx: RequestContext
    ) -> provider.CheckResponse:
        del request, ctx
        await self._backend.capabilities()
        return provider.CheckResponse(status=provider.CheckResponse.STATUS_READY)

    async def get_capabilities(
        self, request: provider.GetCapabilitiesRequest, ctx: RequestContext
    ) -> provider.GetCapabilitiesResponse:
        del request, ctx
        capabilities = await self._backend.capabilities()
        return provider.GetCapabilitiesResponse(
            server_version=capabilities.server_version,
            default_model_id=capabilities.default_model_id,
            models=[
                provider.ModelCapability(
                    id=model.id,
                    supports_structured=model.supports_structured,
                )
                for model in capabilities.models
            ],
        )

    async def generate_text(
        self, request: provider.GenerateTextRequest, ctx: RequestContext
    ) -> provider.GenerateTextResponse:
        if request.source_files:
            raise ConnectError(
                Code.UNIMPLEMENTED,
                "source files require a shared job directory",
            )
        result = await self._run_generation(
            request.generation_id,
            ctx,
            self._backend.generate_text(
                request.generation_id,
                request.model_id,
                request.system_prompt,
                request.user_prompt,
            ),
        )
        if not isinstance(result.content, str):
            raise provider_error(
                Code.INTERNAL,
                provider.LocalProviderErrorDetail.REASON_INTERNAL,
                turn_started=True,
            )
        return provider.GenerateTextResponse(
            text=result.content,
            usage=_usage(result),
        )

    async def generate_structured(
        self, request: provider.GenerateStructuredRequest, ctx: RequestContext
    ) -> provider.GenerateStructuredResponse:
        if request.source_files:
            raise ConnectError(
                Code.UNIMPLEMENTED,
                "source files require a shared job directory",
            )
        try:
            json.loads(request.json_schema)
        except (UnicodeDecodeError, json.JSONDecodeError) as error:
            raise ConnectError(Code.INVALID_ARGUMENT, "invalid JSON schema") from error

        result = await self._run_generation(
            request.generation_id,
            ctx,
            self._backend.generate_structured(
                request.generation_id,
                request.model_id,
                request.system_prompt,
                request.user_prompt,
                request.json_schema,
            ),
        )
        if not isinstance(result.content, bytes):
            raise provider_error(
                Code.INTERNAL,
                provider.LocalProviderErrorDetail.REASON_INTERNAL,
                turn_started=True,
            )
        return provider.GenerateStructuredResponse(
            json_payload=result.content,
            usage=_usage(result),
        )

    async def cancel_generation(
        self, request: provider.CancelGenerationRequest, ctx: RequestContext
    ) -> provider.CancelGenerationResponse:
        del ctx
        return provider.CancelGenerationResponse(
            found=await self._backend.cancel(request.generation_id)
        )

    async def _run_generation(
        self,
        generation_id: str,
        ctx: RequestContext,
        operation: Awaitable[GenerationResult],
    ) -> GenerationResult:
        timeout = self._max_generation_seconds
        if ctx.timeout_ms is not None:
            timeout = min(timeout, max(ctx.timeout_ms / 1000, 0.001))
        try:
            return await asyncio.wait_for(operation, timeout=timeout)
        except asyncio.CancelledError:
            await self._backend.cancel(generation_id)
            raise
        except TimeoutError as error:
            await self._backend.cancel(generation_id)
            raise provider_error(
                Code.DEADLINE_EXCEEDED,
                provider.LocalProviderErrorDetail.REASON_INTERNAL,
                turn_started=True,
            ) from error
        except BackendError as error:
            raise provider_error(
                error.code,
                error.reason,
                turn_started=error.turn_started,
                retry_after_ms=error.retry_after_ms,
            ) from error
        except Exception as error:
            await self._backend.cancel(generation_id)
            raise provider_error(
                Code.INTERNAL,
                provider.LocalProviderErrorDetail.REASON_INTERNAL,
                turn_started=True,
            ) from error


def create_application(
    backend: Backend,
    token: str,
    *,
    max_generation_seconds: float,
) -> LocalProviderServiceASGIApplication:
    return LocalProviderServiceASGIApplication(
        LocalProviderService(backend, max_generation_seconds),
        interceptors=(
            BearerAuthInterceptor(token),
            ContractValidationInterceptor(),
        ),
        read_max_bytes=MAX_REQUEST_BYTES,
    )


def _usage(result: GenerationResult) -> provider.Usage:
    return provider.Usage(
        model=result.model,
        input_tokens=result.input_tokens,
        output_tokens=result.output_tokens,
    )
