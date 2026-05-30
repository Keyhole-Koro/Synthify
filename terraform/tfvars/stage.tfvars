project_id   = "synthify-stage-491705"
region       = "asia-northeast1"
environment  = "stage"
web_base_url = "https://stage.synthify.keyhole.work"

# Lock stage to a single account. CD overrides this with the GitHub
# Environment variable ALLOWED_USER_EMAILS when set; this value is the
# fallback so a manual `make infra-stage-up` is locked down too.
allowed_user_emails = "korokororin47@gmail.com"

# CI/WIF SA that runs terraform apply (GCP_WIF_SA_EMAIL). Needs actAs on
# the api/worker runtime SAs to attach them to Cloud Run.
deployer_principal = "serviceAccount:github-deploy@synthify-stage-491705.iam.gserviceaccount.com"

# Debug: log raw LLM payloads and raw agent responses (per-turn text +
# functionCall presence) to diagnose stuck jobs. Revert to "false" once the
# investigation is done — response bodies end up in logs while this is on.
log_llm_payload = "true"
