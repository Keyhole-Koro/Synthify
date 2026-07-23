project_id   = "synthify-491705"
region       = "asia-northeast1"
environment  = "prod"
web_base_url = "https://synthify.keyhole.work"

# monitor ダッシュボードのカスタムドメイン (scheme なしのホスト名)。
monitor_domain = "monitor.synthify.keyhole.work"

# CI/WIF SA that runs terraform apply (GCP_WIF_SA_EMAIL). Needs actAs on
# the api/worker runtime SAs to attach them to Cloud Run.
deployer_principal = "serviceAccount:github-deploy@synthify-491705.iam.gserviceaccount.com"
