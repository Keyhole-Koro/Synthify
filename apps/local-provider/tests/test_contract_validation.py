from __future__ import annotations

import unittest

from protovalidate import ValidationError, Validator

from synthify.localprovider.v1.provider_pb2 import (
    GenerateStructuredRequest,
    LocalProviderErrorDetail,
)


class ContractValidationTest(unittest.TestCase):
    def setUp(self) -> None:
        self.validator = Validator()

    @staticmethod
    def valid_request() -> GenerateStructuredRequest:
        return GenerateStructuredRequest(
            generation_id="01K3ABCDEF0123456789ABCDEF",
            model_id="antigravity:claude-sonnet",
            system_prompt="Return structured content.",
            json_schema=b"{}",
        )

    def test_valid_structured_request(self) -> None:
        self.validator.validate(self.valid_request())

    def test_generation_id_is_required(self) -> None:
        request = self.valid_request()
        request.generation_id = ""
        with self.assertRaises(ValidationError):
            self.validator.validate(request)

    def test_model_id_rejects_whitespace(self) -> None:
        request = self.valid_request()
        request.model_id = "antigravity: claude"
        with self.assertRaises(ValidationError):
            self.validator.validate(request)

    def test_schema_is_required(self) -> None:
        request = self.valid_request()
        request.json_schema = b""
        with self.assertRaises(ValidationError):
            self.validator.validate(request)

    def test_retry_delay_is_bounded(self) -> None:
        self.validator.validate(
            LocalProviderErrorDetail(
                reason=LocalProviderErrorDetail.REASON_RATE_LIMITED,
                retry_after_ms=60_000,
            )
        )
        with self.assertRaises(ValidationError):
            self.validator.validate(
                LocalProviderErrorDetail(
                    reason=LocalProviderErrorDetail.REASON_RATE_LIMITED,
                    retry_after_ms=60_001,
                )
            )


if __name__ == "__main__":
    unittest.main()
