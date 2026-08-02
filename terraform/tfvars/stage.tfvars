project_id   = "synthify-stage-491705"
region       = "asia-northeast1"
environment  = "stage"
web_base_url = "https://stage.synthify.keyhole.work"

# monitor ダッシュボードのカスタムドメイン (scheme なしのホスト名)。
# 空 => ドメインマッピングを作らず、Cloud Run の run.app URL を使う。
#
# 一時的に空にしている。stage.monitor.synthify.keyhole.work の所有権が GCP 側で
# 未検証で、DomainMapping の作成が "Caller is not authorized to administer the
# domain" で失敗し、デプロイ全体がそこで止まっていた。ダッシュボード自体は
# run.app URL で動くので、検証を待つ間デプロイを止める理由がない。
#
# 戻す手順: Search Console で keyhole.work の所有権を検証し、デプロイに使う
# サービスアカウントを所有者に追加してから、下の値を復帰させて再デプロイする。
# その後 `terraform output monitor_dns_records` が返す CNAME を Cloudflare に
# 追加する (プロキシは DNS only にすること)。
monitor_domain = ""
# monitor_domain = "stage.monitor.synthify.keyhole.work"

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
