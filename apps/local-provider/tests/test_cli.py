from __future__ import annotations

import os
import stat
import tempfile
import unittest
from pathlib import Path

from synthify.localprovider.cli import _configure_worker_environment, _read_token


class ReadTokenTest(unittest.TestCase):
    def setUp(self) -> None:
        self.temporary_directory = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary_directory.cleanup)
        self.directory = Path(self.temporary_directory.name)

    def write_token(self, content: bytes, mode: int = 0o600) -> Path:
        path = self.directory / "provider-token"
        path.write_bytes(content)
        path.chmod(mode)
        return path

    def test_reads_owner_only_ascii_token(self) -> None:
        path = self.write_token(b"a" * 32 + b"\n")
        self.assertEqual(_read_token(str(path)), "a" * 32)

    @unittest.skipUnless(os.name == "posix", "POSIX permission bits required")
    def test_rejects_group_or_world_permissions(self) -> None:
        path = self.write_token(b"a" * 32, mode=0o640)
        with self.assertRaisesRegex(ValueError, "owner-only"):
            _read_token(str(path))

    def test_rejects_symlink(self) -> None:
        target = self.write_token(b"a" * 32)
        link = self.directory / "token-link"
        link.symlink_to(target)
        with self.assertRaisesRegex(ValueError, "non-symlink"):
            _read_token(str(link))

    def test_rejects_invalid_lengths_and_non_ascii(self) -> None:
        invalid_contents = (
            b"short",
            b"a" * 4097,
            "é".encode(),
            b" " + b"a" * 32,
            b"a" * 32 + b"\n\n",
        )
        for index, content in enumerate(invalid_contents):
            with self.subTest(content_length=len(content)):
                path = self.directory / f"invalid-{index}"
                path.write_bytes(content)
                path.chmod(0o600)
                with self.assertRaises(ValueError):
                    _read_token(str(path))

    def test_errors_do_not_disclose_path_or_token(self) -> None:
        secret = "never-echo-this-secret-token-1234"
        path = self.directory / secret
        with self.assertRaises(ValueError) as raised:
            _read_token(str(path))
        message = str(raised.exception)
        self.assertNotIn(secret, message)
        self.assertNotIn(str(self.directory), message)

    def test_configure_worker_environment_creates_token_and_upserts_keys(self) -> None:
        environment_file = self.directory / ".env"
        environment_file.write_text("GEMINI_MODEL=keep\nLLM_PROVIDER=gemini\n")
        environment_file.chmod(0o600)
        token_file = self.directory / "state" / "local-provider" / "token"

        _configure_worker_environment(
            environment_file,
            token_file,
            endpoint="http://127.0.0.1:7777",
            worker_token_file="/run/synthify-local-provider/token",
        )

        contents = environment_file.read_text()
        self.assertIn("GEMINI_MODEL=keep\n", contents)
        self.assertEqual(contents.count("LLM_PROVIDER="), 1)
        self.assertIn("DEPLOYMENT_MODE=self-hosted\n", contents)
        self.assertIn("LLM_PROVIDER=antigravity\n", contents)
        self.assertIn("LOCAL_PROVIDER_ENDPOINT=http://127.0.0.1:7777\n", contents)
        self.assertIn(
            "LOCAL_PROVIDER_TOKEN_FILE=/run/synthify-local-provider/token\n",
            contents,
        )
        self.assertIn(f"LOCAL_PROVIDER_TOKEN_HOST_DIR={token_file.parent}\n", contents)
        self.assertEqual(len(_read_token(str(token_file))), 43)
        self.assertEqual(stat.S_IMODE(token_file.stat().st_mode), 0o600)

        _configure_worker_environment(
            environment_file,
            token_file,
            endpoint="http://127.0.0.1:7777",
            worker_token_file="/run/synthify-local-provider/token",
        )
        self.assertEqual(contents, environment_file.read_text())

    def test_configure_worker_environment_rejects_unsafe_container_path(self) -> None:
        with self.assertRaisesRegex(ValueError, "absolute path"):
            _configure_worker_environment(
                self.directory / ".env",
                self.directory / "state" / "token",
                endpoint="http://127.0.0.1:7777",
                worker_token_file="relative token",
            )


if __name__ == "__main__":
    unittest.main()
