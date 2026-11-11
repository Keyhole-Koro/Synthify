.PHONY: proto-lint proto-build proto-generate proto-generate-local-provider proto-format \
	infra-stage-up infra-stage-down infra-stage-plan db-stage-reset \
	infra-prod-up infra-prod-down infra-prod-plan db-prod-reset \
	tfstate-stage tfstate-prod \
	logs-stage-worker logs-stage-api logs-stage-job logs-stage-trace \
	logs-prod-worker logs-prod-api logs-prod-job logs-prod-trace

proto-lint:
	buf lint

proto-build:
	buf build

proto-generate:
	buf generate

proto-generate-local-provider:
	buf generate --template buf.gen.local-provider.yaml

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

# --- Cloud Logging shortcuts ----------------------------------------------
# Pull recent errors / per-job traces out of Cloud Logging without remembering
# the gcloud incantation. Optional overrides:
#   SINCE='10 minutes ago'   how far back to look (date -d compatible)
#   LIMIT=50                 max entries to return
#   JOB_ID=<ulid>            for logs-*-job: filter by jsonPayload.job_id
#   TRACE_ID=<hex>           for logs-*-trace: filter by trace=projects/.../traces/<TRACE_ID>
#
# Example: make logs-stage-worker SINCE='30 minutes ago' LIMIT=100
# Example: make logs-stage-job JOB_ID=01KSPNQR670W96MNC0J8ST250K
SINCE ?= 1 hour ago
LIMIT ?= 30
STAGE_PROJECT ?= synthify-stage-491705
PROD_PROJECT  ?= synthify-prod
LOG_FORMAT_ERR = value(timestamp,resource.labels.service_name,severity,jsonPayload.msg,jsonPayload.error)
LOG_FORMAT_ALL = value(timestamp,resource.labels.service_name,jsonPayload.level,jsonPayload.msg,jsonPayload.error)

define gcloud_logs_errors
gcloud logging read \
  'resource.labels.service_name="$(1)" AND severity>=ERROR \
   AND timestamp>="'"$$(date -u -d '$(SINCE)' +%Y-%m-%dT%H:%M:%SZ)"'"' \
  --project=$(2) --limit=$(LIMIT) \
  --format='$(LOG_FORMAT_ERR)' --order=desc
endef

define gcloud_logs_by_job
@if [ -z "$(JOB_ID)" ]; then echo "JOB_ID=<ulid> is required" >&2; exit 2; fi
gcloud logging read \
  'jsonPayload.job_id="$(JOB_ID)" \
   AND timestamp>="'"$$(date -u -d '$(SINCE)' +%Y-%m-%dT%H:%M:%SZ)"'"' \
  --project=$(1) --limit=$(LIMIT) \
  --format='$(LOG_FORMAT_ALL)' --order=asc
endef

define gcloud_logs_by_trace
@if [ -z "$(TRACE_ID)" ]; then echo "TRACE_ID=<hex> is required" >&2; exit 2; fi
gcloud logging read \
  'trace="projects/$(1)/traces/$(TRACE_ID)"' \
  --project=$(1) --limit=$(LIMIT) \
  --format='$(LOG_FORMAT_ALL)' --order=asc
endef

logs-stage-worker:
	$(call gcloud_logs_errors,synthify-worker-stage,$(STAGE_PROJECT))

logs-stage-api:
	$(call gcloud_logs_errors,synthify-api-stage,$(STAGE_PROJECT))

logs-stage-job:
	$(call gcloud_logs_by_job,$(STAGE_PROJECT))

logs-stage-trace:
	$(call gcloud_logs_by_trace,$(STAGE_PROJECT))

logs-prod-worker:
	$(call gcloud_logs_errors,synthify-worker-prod,$(PROD_PROJECT))

logs-prod-api:
	$(call gcloud_logs_errors,synthify-api-prod,$(PROD_PROJECT))

logs-prod-job:
	$(call gcloud_logs_by_job,$(PROD_PROJECT))

logs-prod-trace:
	$(call gcloud_logs_by_trace,$(PROD_PROJECT))
