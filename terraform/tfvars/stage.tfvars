project_id   = "synthify-stage-491705"
region       = "asia-northeast1"
environment  = "stage"
web_base_url = "https://stage.synthify.keyhole.work"

# Lock stage to a single account. CD overrides this with the GitHub
# Environment variable ALLOWED_USER_EMAILS when set; this value is the
# fallback so a manual `make infra-stage-up` is locked down too.
allowed_user_emails = "korokororin47@gmail.com"
