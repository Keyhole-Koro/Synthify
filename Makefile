.PHONY: proto-lint proto-build proto-generate proto-format

proto-lint:
	buf lint

proto-build:
	buf build

proto-generate:
	buf generate

proto-format:
	buf format -w
