from __future__ import annotations

import asyncio
import json
import os
import re
import shutil
import signal
import tempfile
from pathlib import Path
from typing import Any

from connectrpc.code import Code
from jsonschema import SchemaError as JSONSchemaSchemaError
from jsonschema import ValidationError as JSONSchemaValidationError
from jsonschema.validators import validator_for

from synthify.localprovider.backend import (
    BackendCapabilities,
    BackendError,
    GenerationResult,
    ModelInfo,
)
from synthify.localprovider.v1 import provider_pb2 as provider


MINIMUM_AGY_VERSION = (1, 1, 15)
MAX_CLI_OUTPUT_BYTES = 18 << 20
DIAGNOSTIC_READ_CHUNK_BYTES = 64 << 10


class AntigravityCLIBackend:
    def __init__(
        self,
        executable: str,
        capabilities: BackendCapabilities,
        *,
        generation_timeout_seconds: float,
        max_concurrent_generations: int,
    ) -> None:
        self._executable = executable
        self._capabilities = capabilities
        self._generation_timeout_seconds = generation_timeout_seconds
        self._generation_slots = asyncio.BoundedSemaphore(max_concurrent_generations)
        self._active: dict[str, asyncio.subprocess.Process] = {}
        self._lock = asyncio.Lock()

    @classmethod
    async def create(
        cls,
        executable: str,
        default_model: str,
        *,
        generation_timeout_seconds: float,
        max_concurrent_generations: int = 2,
    ) -> AntigravityCLIBackend:
        if generation_timeout_seconds <= 0 or generation_timeout_seconds > 3600:
            raise ValueError("generation timeout must be between 0 and 3600 seconds")
        if not 1 <= max_concurrent_generations <= 16:
            raise ValueError("maximum concurrent generations must be between 1 and 16")
        executable_path = shutil.which(executable)
        if executable_path is None:
            raise ValueError("Antigravity CLI executable was not found")
        version = await _read_version(executable_path)
        if version < MINIMUM_AGY_VERSION:
            required = ".".join(str(part) for part in MINIMUM_AGY_VERSION)
            raise ValueError(f"Antigravity CLI {required} or newer is required")

        model_slugs = await _read_models(executable_path)
        if default_model not in model_slugs:
            raise ValueError("default Antigravity model is not advertised by the CLI")
        canonical_models = tuple(
            ModelInfo(id=f"antigravity:{slug}") for slug in model_slugs
        )
        return cls(
            executable_path,
            BackendCapabilities(
                server_version="antigravity-cli/" + ".".join(map(str, version)),
                default_model_id=f"antigravity:{default_model}",
                models=canonical_models,
            ),
            generation_timeout_seconds=generation_timeout_seconds,
            max_concurrent_generations=max_concurrent_generations,
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
        result = await self._execute(
            generation_id,
            model_id,
            _compose_prompt(system_prompt, user_prompt),
        )
        response = result.get("response")
        if not isinstance(response, str) or not response:
            raise BackendError.internal(turn_started=True)
        return _generation_result(model_id, response, result)

    async def generate_structured(
        self,
        generation_id: str,
        model_id: str,
        system_prompt: str,
        user_prompt: str,
        json_schema: bytes,
    ) -> GenerationResult:
        try:
            schema = json.loads(json_schema)
        except (UnicodeDecodeError, json.JSONDecodeError) as error:
            raise BackendError(
                Code.INVALID_ARGUMENT,
                provider.LocalProviderErrorDetail.REASON_INVALID_STRUCTURED_OUTPUT,
            ) from error
        try:
            validator_class = validator_for(schema)
            validator_class.check_schema(schema)
        except (JSONSchemaSchemaError, TypeError) as error:
            raise BackendError(
                Code.INVALID_ARGUMENT,
                provider.LocalProviderErrorDetail.REASON_INVALID_STRUCTURED_OUTPUT,
            ) from error

        result = await self._execute(
            generation_id,
            model_id,
            _compose_prompt(system_prompt, user_prompt),
            schema_bytes=json_schema,
        )

        structured = result.get("structured_output")
        if structured is None:
            response = result.get("response")
            if not isinstance(response, str):
                raise BackendError.internal(turn_started=True)
            try:
                structured = json.loads(response)
            except json.JSONDecodeError as error:
                raise BackendError(
                    Code.INTERNAL,
                    provider.LocalProviderErrorDetail.REASON_INVALID_STRUCTURED_OUTPUT,
                    turn_started=True,
                ) from error
        try:
            validator_class(schema).validate(structured)
        except JSONSchemaValidationError as error:
            raise BackendError(
                Code.INTERNAL,
                provider.LocalProviderErrorDetail.REASON_INVALID_STRUCTURED_OUTPUT,
                turn_started=True,
            ) from error
        encoded = json.dumps(
            structured, ensure_ascii=False, separators=(",", ":")
        ).encode()
        return _generation_result(model_id, encoded, result)

    async def cancel(self, generation_id: str) -> bool:
        async with self._lock:
            process = self._active.get(generation_id)
        if process is None or process.returncode is not None:
            return False
        await _stop_process(process)
        return True

    async def close(self) -> None:
        async with self._lock:
            processes = list(self._active.values())
        await asyncio.gather(*(_stop_process(process) for process in processes))

    async def _execute(
        self,
        generation_id: str,
        model_id: str,
        prompt: str,
        *,
        schema_bytes: bytes | None = None,
    ) -> dict[str, Any]:
        model_slug = _model_slug(model_id, self._capabilities)
        arguments = [
            self._executable,
            "--input-format",
            "stream-json",
            "--output-format",
            "stream-json",
            "--disable-slash-commands",
            "--sandbox",
            "--mode",
            "plan",
            "--model",
            model_slug,
            "--print-timeout",
            f"{max(1, int(self._generation_timeout_seconds))}s",
        ]
        input_message = (
            json.dumps(
                {"event": "user", "message": {"content": prompt}},
                ensure_ascii=False,
                separators=(",", ":"),
            ).encode()
            + b"\n"
        )

        async with self._generation_slots:
            with tempfile.TemporaryDirectory(
                prefix="synthify-provider-job-"
            ) as directory:
                if schema_bytes is not None:
                    schema_path = Path(directory, "schema.json")
                    schema_path.write_bytes(schema_bytes)
                    schema_path.chmod(0o400)
                    arguments.extend(("--json-schema", str(schema_path)))
                process = await asyncio.create_subprocess_exec(
                    *arguments,
                    cwd=directory,
                    stdin=asyncio.subprocess.PIPE,
                    stdout=asyncio.subprocess.PIPE,
                    stderr=asyncio.subprocess.PIPE,
                    start_new_session=(os.name == "posix"),
                    limit=MAX_CLI_OUTPUT_BYTES + 1,
                )
                async with self._lock:
                    duplicate = generation_id in self._active
                    if not duplicate:
                        self._active[generation_id] = process
                if duplicate:
                    await _stop_process(process)
                    raise BackendError.internal()
                try:
                    return await asyncio.wait_for(
                        _read_result(process, input_message),
                        timeout=self._generation_timeout_seconds,
                    )
                except TimeoutError as error:
                    await _stop_process(process)
                    raise BackendError(
                        Code.DEADLINE_EXCEEDED,
                        provider.LocalProviderErrorDetail.REASON_INTERNAL,
                        turn_started=True,
                    ) from error
                except asyncio.CancelledError:
                    await _stop_process(process)
                    raise
                finally:
                    async with self._lock:
                        if self._active.get(generation_id) is process:
                            self._active.pop(generation_id, None)


async def _read_version(executable: str) -> tuple[int, int, int]:
    output = await _run_probe(executable, "--version")
    match = re.fullmatch(r"\s*(\d+)\.(\d+)\.(\d+)(?:[-+].*)?\s*", output)
    if match is None:
        raise ValueError("Antigravity CLI returned an invalid version")
    return tuple(int(part) for part in match.groups())


async def _read_models(executable: str) -> tuple[str, ...]:
    output = await _run_probe(executable, "--output-format", "json", "models")
    try:
        envelope = json.loads(output)
    except json.JSONDecodeError as error:
        raise ValueError("Antigravity CLI returned invalid model metadata") from error
    if (
        not isinstance(envelope, dict)
        or envelope.get("status") != "SUCCESS"
        or not isinstance(envelope.get("response"), str)
    ):
        raise ValueError("Antigravity CLI model discovery failed")
    models: list[str] = []
    for line in envelope["response"].splitlines():
        slug, separator, _ = line.partition("\t")
        if separator and re.fullmatch(r"[a-z0-9][a-z0-9-]{0,126}", slug):
            models.append(slug)
    if not models or len(models) != len(set(models)):
        raise ValueError("Antigravity CLI returned invalid model metadata")
    return tuple(models)


async def _run_probe(executable: str, *arguments: str) -> str:
    process = await asyncio.create_subprocess_exec(
        executable,
        *arguments,
        stdin=asyncio.subprocess.DEVNULL,
        stdout=asyncio.subprocess.PIPE,
        stderr=asyncio.subprocess.PIPE,
        start_new_session=(os.name == "posix"),
        limit=1 << 20,
    )
    try:
        stdout, _ = await asyncio.wait_for(process.communicate(), timeout=15)
    except TimeoutError as error:
        await _stop_process(process)
        raise ValueError("Antigravity CLI probe timed out") from error
    if process.returncode != 0 or len(stdout) > 1 << 20:
        raise ValueError("Antigravity CLI probe failed")
    return stdout.decode("utf-8")


async def _read_result(
    process: asyncio.subprocess.Process, input_message: bytes
) -> dict[str, Any]:
    if process.stdin is None or process.stdout is None or process.stderr is None:
        raise BackendError.internal()
    diagnostics = asyncio.create_task(_drain_diagnostics(process.stderr))
    total_bytes = 0
    final_result: dict[str, Any] | None = None
    saw_turn = False
    try:
        process.stdin.write(input_message)
        await process.stdin.drain()
        process.stdin.close()
        while line := await process.stdout.readline():
            total_bytes += len(line)
            if total_bytes > MAX_CLI_OUTPUT_BYTES:
                raise BackendError.internal(turn_started=saw_turn)
            try:
                event = json.loads(line)
            except json.JSONDecodeError as error:
                raise BackendError.internal(turn_started=saw_turn) from error
            if not isinstance(event, dict):
                raise BackendError.internal(turn_started=saw_turn)
            if event.get("event") == "step_update":
                step = event.get("step_update")
                saw_turn = saw_turn or (
                    isinstance(step, dict) and step.get("step_type") == "user_input"
                )
            if event.get("event") == "result" and isinstance(event.get("result"), dict):
                final_result = event["result"]
        return_code = await process.wait()
    except asyncio.CancelledError:
        await _stop_process(process)
        raise
    except BackendError:
        await _stop_process(process)
        raise
    except (OSError, ValueError, asyncio.LimitOverrunError) as error:
        await _stop_process(process)
        raise BackendError.internal(turn_started=saw_turn) from error
    finally:
        await diagnostics

    if final_result is None:
        raise BackendError.internal(turn_started=saw_turn)
    status = final_result.get("status")
    if status != "SUCCESS" or return_code != 0:
        raise _backend_error(final_result, saw_turn)
    return final_result


async def _drain_diagnostics(stream: asyncio.StreamReader) -> None:
    while await stream.read(DIAGNOSTIC_READ_CHUNK_BYTES):
        pass


def _backend_error(result: dict[str, Any], saw_turn: bool) -> BackendError:
    status = result.get("status")
    if status in ("CANCELED", "INTERRUPTED"):
        return BackendError(
            Code.CANCELED,
            provider.LocalProviderErrorDetail.REASON_INTERNAL,
            turn_started=saw_turn,
        )
    raw_error = result.get("error")
    classification = raw_error.lower() if isinstance(raw_error, str) else ""
    if (
        "authentication required" in classification
        or "unauthenticated" in classification
    ):
        return BackendError(
            Code.UNAUTHENTICATED,
            provider.LocalProviderErrorDetail.REASON_AUTHENTICATION_REQUIRED,
            turn_started=saw_turn,
        )
    if "quota" in classification or "out of credits" in classification:
        return BackendError(
            Code.RESOURCE_EXHAUSTED,
            provider.LocalProviderErrorDetail.REASON_PROVIDER_QUOTA_EXHAUSTED,
            turn_started=saw_turn,
        )
    if "rate limit" in classification or "resource_exhausted" in classification:
        return BackendError(
            Code.RESOURCE_EXHAUSTED,
            provider.LocalProviderErrorDetail.REASON_RATE_LIMITED,
            turn_started=saw_turn,
        )
    return BackendError.internal(turn_started=saw_turn)


async def _stop_process(process: asyncio.subprocess.Process) -> None:
    if process.returncode is not None:
        return
    try:
        if os.name == "posix":
            os.killpg(process.pid, signal.SIGTERM)
        else:
            process.terminate()
    except ProcessLookupError:
        return
    try:
        await asyncio.wait_for(process.wait(), timeout=2)
        return
    except TimeoutError:
        pass
    try:
        if os.name == "posix":
            os.killpg(process.pid, signal.SIGKILL)
        else:
            process.kill()
    except ProcessLookupError:
        return
    await process.wait()


def _model_slug(model_id: str, capabilities: BackendCapabilities) -> str:
    if model_id not in {model.id for model in capabilities.models}:
        raise BackendError(
            Code.INVALID_ARGUMENT,
            provider.LocalProviderErrorDetail.REASON_INTERNAL,
        )
    return model_id.removeprefix("antigravity:")


def _compose_prompt(system_prompt: str, user_prompt: str) -> str:
    sections: list[str] = []
    if system_prompt:
        sections.append("System instructions:\n" + system_prompt)
    if user_prompt:
        sections.append("User request:\n" + user_prompt)
    return "\n\n".join(sections) or "Produce the requested output."


def _generation_result(
    model_id: str, content: str | bytes, result: dict[str, Any]
) -> GenerationResult:
    usage = result.get("usage")
    if not isinstance(usage, dict):
        raise BackendError.internal(turn_started=True)
    input_tokens = usage.get("input_tokens")
    output_tokens = usage.get("output_tokens")
    if (
        not isinstance(input_tokens, int)
        or isinstance(input_tokens, bool)
        or input_tokens < 0
        or not isinstance(output_tokens, int)
        or isinstance(output_tokens, bool)
        or output_tokens < 0
    ):
        raise BackendError.internal(turn_started=True)
    return GenerationResult(
        content=content,
        model=model_id,
        input_tokens=input_tokens,
        output_tokens=output_tokens,
    )
