from __future__ import annotations

import os
import tempfile
import unittest
from pathlib import Path

from synthify.localprovider.cli import _read_token


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


if __name__ == "__main__":
    unittest.main()
