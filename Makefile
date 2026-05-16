.PHONY: proto-lint proto-build proto-generate proto-format infra-stage-up infra-stage-down infra-stage-plan db-stage-reset

proto-lint:
	buf lint

proto-build:
	buf build

proto-generate:
	buf generate

proto-format:
	buf format -w

infra-stage-up:
	bash scripts/manage-stage.sh up

infra-stage-down:
	bash scripts/manage-stage.sh down

infra-stage-plan:
	bash scripts/manage-stage.sh plan

db-stage-reset:
	bash scripts/manage-stage.sh reset-db
