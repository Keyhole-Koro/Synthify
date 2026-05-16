.PHONY: proto-lint proto-build proto-generate proto-format \
	infra-stage-up infra-stage-down infra-stage-plan db-stage-reset \
	infra-prod-up infra-prod-down infra-prod-plan db-prod-reset \
	tfstate-stage tfstate-prod

proto-lint:
	buf lint

proto-build:
	buf build

proto-generate:
	buf generate

proto-format:
	buf format -w

# --- State buckets (run once per env before terraform init) ---
# Usage: make tfstate-stage PROJECT_ID=my-stage-proj [REGION=asia-northeast1]
tfstate-stage:
	bash scripts/bootstrap-tfstate.sh stage $(PROJECT_ID) $(REGION)

tfstate-prod:
	bash scripts/bootstrap-tfstate.sh prod $(PROJECT_ID) $(REGION)

# --- Stage ---
infra-stage-up:
	bash scripts/manage-env.sh stage up

infra-stage-down:
	bash scripts/manage-env.sh stage down

infra-stage-plan:
	bash scripts/manage-env.sh stage plan

db-stage-reset:
	bash scripts/manage-env.sh stage reset-db

# --- Prod ---
infra-prod-up:
	bash scripts/manage-env.sh prod up

infra-prod-down:
	bash scripts/manage-env.sh prod down

infra-prod-plan:
	bash scripts/manage-env.sh prod plan

db-prod-reset:
	bash scripts/manage-env.sh prod reset-db
