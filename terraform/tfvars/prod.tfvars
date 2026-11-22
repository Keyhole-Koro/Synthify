project_id   = "synthify-491705"
region       = "asia-northeast1"
environment  = "prod"
web_base_url = "https://synthify.keyhole.work"

# monitor ダッシュボードのカスタムドメイン (scheme なしのホスト名)。
# 空 => ドメインマッピングを作らず、Cloud Run の run.app URL を使う。
#
# stage と同じ理由で一時的に空にしている (詳細は stage.tfvars)。prod はまだ
# monitor をデプロイしていないが、ドメイン所有権の検証は keyhole.work 単位なので
# 未検証のまま流せば同じ地点で止まる。先に踏まないようにしておく。
monitor_domain = ""
# monitor_domain = "monitor.synthify.keyhole.work"

# CI/WIF SA that runs terraform apply (GCP_WIF_SA_EMAIL). Needs actAs on
# the api/worker runtime SAs to attach them to Cloud Run.
deployer_principal = "serviceAccount:github-deploy@synthify-491705.iam.gserviceaccount.com"

# New documents and paid plans are closed: the API refuses uploads, document
# processing and Stripe Checkout, the web says so, and the daily LLM eval is
# paused. Trees are made on users' own machines with the agent skill now.
# Existing trees stay readable, shareable and deletable. To reopen, set false
# and deploy (docs/runbooks/README.md).
intake_closed = true
