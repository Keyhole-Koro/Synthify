.PHONY: proto-lint proto-build proto-generate proto-format

proto-lint:
	./scripts/proto.sh lint

proto-build:
	./scripts/proto.sh build

proto-generate:
	./scripts/proto.sh generate

proto-format:
	./scripts/proto.sh format
