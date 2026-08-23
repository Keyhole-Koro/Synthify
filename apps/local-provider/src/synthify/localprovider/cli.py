from __future__ import annotations

import argparse
import asyncio
import ipaddress
import os
import stat
from pathlib import Path

import uvicorn

from synthify.localprovider.antigravity_cli import AntigravityCLIBackend
from synthify.localprovider.server import create_application


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
            token = token_file.read(4097).decode("ascii").strip()
    except (OSError, UnicodeError) as error:
        raise ValueError("token file is unavailable") from error
    finally:
        if descriptor >= 0:
            os.close(descriptor)
    if (
        len(token) < 32
        or len(token) > 4096
        or any(character <= " " or character > "~" for character in token)
    ):
        raise ValueError("token file contains an invalid token")
    return token


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
    parser.add_argument("--token-file", required=True)
    parser.add_argument("--agy-path", default="agy")
    parser.add_argument("--default-model", required=True)
    parser.add_argument("--generation-timeout", type=float, default=300)
    parser.add_argument("--max-concurrent-generations", type=int, default=2)
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

    try:
        asyncio.run(_serve(args))
    except (OSError, ValueError) as error:
        parser.error(str(error))


if __name__ == "__main__":
    main()
