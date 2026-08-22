from __future__ import annotations

import argparse
import hmac
import json
import threading
from collections.abc import Callable
from socketserver import ThreadingMixIn
from typing import Any, BinaryIO
from wsgiref.simple_server import WSGIRequestHandler, WSGIServer, make_server

from connectrpc.code import Code
from connectrpc.errors import ConnectError, ErrorDetail
from connectrpc.request import RequestContext
from protobuf.wkt import Any as ConnectAny
from protovalidate import ValidationError, Validator

from synthify.localprovider.v1 import provider_pb2 as provider
from synthify.localprovider.v1.provider_connect import (
    LocalProviderServiceWSGIApplication,
)


MODEL_ID = "antigravity:test-model"


def _provider_error(
    code: Code,
    message: str,
    detail: provider.LocalProviderErrorDetail,
) -> ConnectError:
    """Bridge a google.protobuf detail into Connect Python's wire container."""
    wire_detail = ConnectAny(
        type_url=f"type.googleapis.com/{detail.DESCRIPTOR.full_name}",
        value=detail.SerializeToString(),
    )
    return ConnectError(code, message, details=(ErrorDetail(wire_detail),))


class BearerAuthInterceptor:
    def __init__(self, token: str) -> None:
        self._expected = f"Bearer {token}"

    def intercept_unary_sync(
        self,
        call_next: Callable[[Any, RequestContext], Any],
        request: Any,
        ctx: RequestContext,
    ) -> Any:
        supplied = ctx.request_headers.get("authorization", "")
        if not hmac.compare_digest(supplied, self._expected):
            raise _provider_error(
                Code.UNAUTHENTICATED,
                "local provider authentication failed",
                provider.LocalProviderErrorDetail(
                    reason=provider.LocalProviderErrorDetail.REASON_AUTHENTICATION_REQUIRED,
                ),
            )
        return call_next(request, ctx)


class ContractValidationInterceptor:
    def __init__(self) -> None:
        self._validator = Validator()

    def intercept_unary_sync(
        self,
        call_next: Callable[[Any, RequestContext], Any],
        request: Any,
        ctx: RequestContext,
    ) -> Any:
        try:
            self._validator.validate(request)
        except ValidationError as error:
            raise ConnectError(Code.INVALID_ARGUMENT, "invalid request") from error

        response = call_next(request, ctx)
        try:
            self._validator.validate(response)
        except ValidationError as error:
            raise _provider_error(
                Code.INTERNAL,
                "invalid provider response",
                provider.LocalProviderErrorDetail(
                    reason=provider.LocalProviderErrorDetail.REASON_INTERNAL,
                    turn_started=True,
                ),
            ) from error
        return response


class DeterministicLocalProvider:
    def __init__(self) -> None:
        self._lock = threading.Lock()
        self._active: dict[str, threading.Event] = {}

    def check(
        self, request: provider.CheckRequest, ctx: RequestContext
    ) -> provider.CheckResponse:
        del request, ctx
        return provider.CheckResponse(status=provider.CheckResponse.STATUS_READY)

    def get_capabilities(
        self, request: provider.GetCapabilitiesRequest, ctx: RequestContext
    ) -> provider.GetCapabilitiesResponse:
        del request, ctx
        return provider.GetCapabilitiesResponse(
            server_version="cross-language-test",
            default_model_id=MODEL_ID,
            models=[
                provider.ModelCapability(id=MODEL_ID, supports_structured=True),
            ],
        )

    def generate_text(
        self, request: provider.GenerateTextRequest, ctx: RequestContext
    ) -> provider.GenerateTextResponse:
        del ctx
        if request.user_prompt == "rate-limit":
            raise _provider_error(
                Code.RESOURCE_EXHAUSTED,
                "provider rate limit",
                provider.LocalProviderErrorDetail(
                    reason=provider.LocalProviderErrorDetail.REASON_RATE_LIMITED,
                    turn_started=False,
                    retry_after_ms=250,
                ),
            )

        if request.user_prompt == "wait-for-cancel":
            cancelled = threading.Event()
            with self._lock:
                self._active[request.generation_id] = cancelled
            print(f"STARTED {request.generation_id}", flush=True)
            if not cancelled.wait(timeout=10):
                with self._lock:
                    self._active.pop(request.generation_id, None)
                raise _provider_error(
                    Code.DEADLINE_EXCEEDED,
                    "test generation was not cancelled",
                    provider.LocalProviderErrorDetail(
                        reason=provider.LocalProviderErrorDetail.REASON_INTERNAL,
                        turn_started=True,
                    ),
                )
            raise ConnectError(Code.CANCELED, "generation cancelled")

        return provider.GenerateTextResponse(
            text=f"text:{request.user_prompt}",
            usage=provider.Usage(
                model=MODEL_ID,
                input_tokens=3,
                output_tokens=2,
            ),
        )

    def generate_structured(
        self, request: provider.GenerateStructuredRequest, ctx: RequestContext
    ) -> provider.GenerateStructuredResponse:
        del ctx
        json.loads(request.json_schema)
        return provider.GenerateStructuredResponse(
            json_payload=b'{"answer":"ok"}',
            usage=provider.Usage(
                model=MODEL_ID,
                input_tokens=5,
                output_tokens=3,
            ),
        )

    def cancel_generation(
        self, request: provider.CancelGenerationRequest, ctx: RequestContext
    ) -> provider.CancelGenerationResponse:
        del ctx
        with self._lock:
            cancelled = self._active.pop(request.generation_id, None)
        if cancelled is None:
            return provider.CancelGenerationResponse(found=False)
        cancelled.set()
        print(f"CANCELLED {request.generation_id}", flush=True)
        return provider.CancelGenerationResponse(found=True)


class ThreadingWSGIServer(ThreadingMixIn, WSGIServer):
    daemon_threads = True


class QuietRequestHandler(WSGIRequestHandler):
    def log_message(self, format: str, *args: Any) -> None:
        del format, args


class ContentLengthReader:
    """Keep WSGI reads within the declared body instead of the live socket."""

    def __init__(self, stream: BinaryIO, length: int) -> None:
        self._stream = stream
        self._remaining = length

    def read(self, size: int = -1) -> bytes:
        if self._remaining == 0:
            return b""
        if size < 0 or size > self._remaining:
            size = self._remaining
        contents = self._stream.read(size)
        self._remaining -= len(contents)
        return contents


class ContentLengthBoundApplication:
    def __init__(self, application: LocalProviderServiceWSGIApplication) -> None:
        self._application = application

    def __call__(self, environ: dict[str, Any], start_response: Callable) -> Any:
        length = int(environ.get("CONTENT_LENGTH") or 0)
        environ["wsgi.input"] = ContentLengthReader(environ["wsgi.input"], length)
        return self._application(environ, start_response)


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--host", default="127.0.0.1")
    parser.add_argument("--port", type=int, default=0)
    parser.add_argument("--token-file", required=True)
    args = parser.parse_args()

    with open(args.token_file, encoding="utf-8") as token_file:
        token = token_file.read().strip()

    application = LocalProviderServiceWSGIApplication(
        DeterministicLocalProvider(),
        interceptors=(
            BearerAuthInterceptor(token),
            ContractValidationInterceptor(),
        ),
    )
    server = make_server(
        args.host,
        args.port,
        ContentLengthBoundApplication(application),
        server_class=ThreadingWSGIServer,
        handler_class=QuietRequestHandler,
    )
    print(f"READY {server.server_port}", flush=True)
    server.serve_forever()


if __name__ == "__main__":
    main()
