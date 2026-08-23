from __future__ import annotations

import argparse
import asyncio
import ipaddress
import os
import secrets
import stat
import tempfile
from pathlib import Path

import uvicorn

from synthify.localprovider.antigravity_cli import AntigravityCLIBackend
from synthify.localprovider.server import create_application


def _default_token_file() -> Path:
    state_directory = Path(
        os.environ.get("XDG_STATE_HOME", Path.home() / ".local" / "state")
    )
    return state_directory / "synthify" / "local-provider" / "token"


def _read_token(path: str) -> str:
    token_path = Path(path)
    try:
        path_info = token_path.lstat()
    except OSError as error:
        raise ValueError("token file is unavailable") from error
    if stat.S_ISLNK(path_info.st_mode):
        raise ValueError("token file must be a regular non-symlink file")

    flags = os.O_RDONLY
    if hasattr(os, "O_NOFOLLOW"):
        flags |= os.O_NOFOLLOW
    descriptor = -1
    try:
        descriptor = os.open(token_path, flags)
        file_info = os.fstat(descriptor)
        if not stat.S_ISREG(file_info.st_mode):
            raise ValueError("token file must be a regular non-symlink file")
        if (path_info.st_dev, path_info.st_ino) != (file_info.st_dev, file_info.st_ino):
            raise ValueError("token file changed while it was being opened")
        if os.name == "posix" and file_info.st_mode & 0o077:
            raise ValueError("token file permissions must be owner-only")
        with os.fdopen(descriptor, "rb") as token_file:
            descriptor = -1
            token = token_file.read(4097).decode("ascii")
    except (OSError, UnicodeError) as error:
        raise ValueError("token file is unavailable") from error
    finally:
        if descriptor >= 0:
            os.close(descriptor)
    if token.endswith("\n"):
        token = token[:-1]
        if token.endswith("\r"):
            token = token[:-1]
    if (
        len(token) < 32
        or len(token) > 4096
        or any(character <= " " or character > "~" for character in token)
    ):
        raise ValueError("token file contains an invalid token")
    return token


def _ensure_token_file(path: Path) -> None:
    if os.path.lexists(path):
        _read_token(str(path))
        return
    try:
        path.parent.mkdir(mode=0o700, parents=True, exist_ok=True)
        directory_info = path.parent.stat()
    except OSError as error:
        raise ValueError("token directory is unavailable") from error
    if not stat.S_ISDIR(directory_info.st_mode):
        raise ValueError("token directory is unavailable")
    if os.name == "posix" and directory_info.st_mode & 0o077:
        raise ValueError("token directory permissions must be owner-only")

    descriptor = -1
    try:
        descriptor = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
        with os.fdopen(descriptor, "wb") as token_file:
            descriptor = -1
            token_file.write(secrets.token_urlsafe(32).encode("ascii"))
    except FileExistsError:
        _read_token(str(path))
    except OSError as error:
        raise ValueError("token file is unavailable") from error
    finally:
        if descriptor >= 0:
            os.close(descriptor)
    _read_token(str(path))


def _configure_worker_environment(
    environment_file: Path,
    token_file: Path,
    *,
    endpoint: str,
    worker_token_file: str,
) -> None:
    if not worker_token_file.startswith("/") or any(
        character.isspace() for character in worker_token_file
    ):
        raise ValueError(
            "worker token file must be an absolute path without whitespace"
        )
    _ensure_token_file(token_file)
    values = {
        "DEPLOYMENT_MODE": "self-hosted",
        "LLM_PROVIDER": "antigravity",
        "LOCAL_PROVIDER_ENDPOINT": endpoint,
        "LOCAL_PROVIDER_TOKEN_FILE": worker_token_file,
        "LOCAL_PROVIDER_TOKEN_HOST_DIR": str(token_file.parent),
    }
    try:
        lines = environment_file.read_text(encoding="utf-8").splitlines()
    except FileNotFoundError:
        lines = []
    except OSError as error:
        raise ValueError("worker environment file is unavailable") from error

    retained = []
    for line in lines:
        key, separator, _ = line.partition("=")
        if separator and key.strip() in values:
            continue
        retained.append(line)
    if retained and retained[-1]:
        retained.append("")
    retained.extend(f"{key}={value}" for key, value in values.items())
    contents = "\n".join(retained) + "\n"

    try:
        environment_file.parent.mkdir(parents=True, exist_ok=True)
        existing_mode = (
            stat.S_IMODE(environment_file.stat().st_mode)
            if environment_file.exists()
            else 0o600
        )
        descriptor, temporary_path = tempfile.mkstemp(
            prefix=f".{environment_file.name}.",
            dir=environment_file.parent,
        )
        with os.fdopen(descriptor, "w", encoding="utf-8") as output:
            output.write(contents)
        os.chmod(temporary_path, existing_mode)
        os.replace(temporary_path, environment_file)
    except OSError as error:
        raise ValueError("worker environment file is unavailable") from error


async def _serve(args: argparse.Namespace) -> None:
    token = _read_token(args.token_file)
    backend = await AntigravityCLIBackend.create(
        args.agy_path,
        args.default_model,
        generation_timeout_seconds=args.generation_timeout,
        max_concurrent_generations=args.max_concurrent_generations,
    )
    application = create_application(
        backend,
        token,
        max_generation_seconds=args.generation_timeout,
    )
    config = uvicorn.Config(
        application,
        host=args.host,
        port=args.port,
        log_level="warning",
        access_log=False,
    )
    server = uvicorn.Server(config)
    try:
        await server.serve()
    finally:
        await backend.close()


def main() -> None:
    parser = argparse.ArgumentParser(prog="synthify-local-provider")
    parser.add_argument("--host", default="127.0.0.1")
    parser.add_argument("--port", type=int, default=7777)
    parser.add_argument(
        "--token-file",
        help="owner-only token file; defaults to the user local-state directory",
    )
    parser.add_argument("--agy-path", default="agy")
    parser.add_argument("--default-model", required=True)
    parser.add_argument("--generation-timeout", type=float, default=300)
    parser.add_argument("--max-concurrent-generations", type=int, default=2)
    parser.add_argument(
        "--configure-worker-env",
        type=Path,
        metavar="PATH",
        help="upsert self-hosted Worker settings into a Compose .env file before serving",
    )
    parser.add_argument(
        "--worker-token-file",
        help="absolute path used by the Worker container; defaults to /run/synthify-local-provider/<token name>",
    )
    args = parser.parse_args()

    try:
        address = ipaddress.ip_address(args.host)
    except ValueError as error:
        parser.error(str(error))
    if not address.is_loopback:
        parser.error("--host must be a loopback address")
    if not 1 <= args.port <= 65535:
        parser.error("--port must be between 1 and 65535")
    if args.generation_timeout <= 0 or args.generation_timeout > 3600:
        parser.error("--generation-timeout must be between 0 and 3600 seconds")
    if not 1 <= args.max_concurrent_generations <= 16:
        parser.error("--max-concurrent-generations must be between 1 and 16")

    token_file = Path(args.token_file) if args.token_file else _default_token_file()
    args.token_file = str(token_file)
    if args.configure_worker_env is not None:
        endpoint_host = f"[{args.host}]" if address.version == 6 else args.host
        worker_token_file = args.worker_token_file or (
            "/run/synthify-local-provider/" + token_file.name
        )
        try:
            _configure_worker_environment(
                args.configure_worker_env,
                token_file,
                endpoint=f"http://{endpoint_host}:{args.port}",
                worker_token_file=worker_token_file,
            )
        except ValueError as error:
            parser.error(str(error))
        print(f"configured Worker environment: {args.configure_worker_env}")

    try:
        asyncio.run(_serve(args))
    except (OSError, ValueError) as error:
        parser.error(str(error))


if __name__ == "__main__":
    main()
