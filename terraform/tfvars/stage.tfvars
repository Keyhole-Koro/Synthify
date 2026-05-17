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

# Stripe usage-based price IDs (non-secret identifiers; safe in plaintext and
# also exposed to the frontend). The env var names say PRO_ for historical
# reasons only — the Pro plan was removed and the app registers these as
# BillingPlanUsageBased (see apps/api/internal/infrastructure/stripe/provider.go).
# At least one currency is required or the API exits(1) on startup.
stripe_pro_price_id_jpy = "price_1TWoCK2YahrFM7WjfShNKk0s"
stripe_pro_price_id_usd = "price_1TWoCK2YahrFM7WjyDlIeh3l"
# stripe_default_currency defaults to "jpy"; uncomment to change.
# stripe_default_currency = "jpy"
