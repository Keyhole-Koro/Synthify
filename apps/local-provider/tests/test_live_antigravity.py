from __future__ import annotations

import json
import os
import unittest

from synthify.localprovider.antigravity_cli import AntigravityCLIBackend


LIVE_ENABLED = os.environ.get("SYNTHIFY_LOCAL_PROVIDER_LIVE") == "1"
LIVE_MODEL = os.environ.get("SYNTHIFY_LOCAL_PROVIDER_LIVE_MODEL", "")


@unittest.skipUnless(
    LIVE_ENABLED and LIVE_MODEL,
    "set SYNTHIFY_LOCAL_PROVIDER_LIVE=1 and SYNTHIFY_LOCAL_PROVIDER_LIVE_MODEL",
)
class LiveAntigravityTest(unittest.IsolatedAsyncioTestCase):
    async def asyncSetUp(self) -> None:
        self.backend = await AntigravityCLIBackend.create(
            os.environ.get("SYNTHIFY_LOCAL_PROVIDER_AGY_PATH", "agy"),
            LIVE_MODEL,
            generation_timeout_seconds=300,
            max_concurrent_generations=1,
        )

    async def asyncTearDown(self) -> None:
        await self.backend.close()

    async def test_text_and_structured_generation(self) -> None:
        text = await self.backend.generate_text(
            "live-text-smoke",
            f"antigravity:{LIVE_MODEL}",
            "This is a local provider smoke test. Do not inspect or modify files.",
            "Reply with a short confirmation.",
        )
        self.assertIsInstance(text.content, str)
        self.assertTrue(text.content)
        self.assertEqual(text.model, f"antigravity:{LIVE_MODEL}")

        structured = await self.backend.generate_structured(
            "live-structured-smoke",
            f"antigravity:{LIVE_MODEL}",
            "This is a local provider smoke test. Do not inspect or modify files.",
            "Return an object whose ok field is true.",
            b'{"type":"object","properties":{"ok":{"const":true}},"required":["ok"],"additionalProperties":false}',
        )
        self.assertEqual(json.loads(structured.content), {"ok": True})
        self.assertEqual(structured.model, f"antigravity:{LIVE_MODEL}")


if __name__ == "__main__":
    unittest.main()
