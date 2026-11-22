# サービスアカウントと IAM（手で apply する）

環境（stage・prod）ごとに、次のものを持つ。CD は apply しない。デプロイ用 SA にはできないようにしてある。

- 実行用 SA 5 つ（`synthify-{api,worker,eval,eval-scheduler,monitor}-<env>`）と、そのプロジェクトのロール（Firestore、Vertex AI、Cloud Tasks）、API の自分自身への付与（actAs、トークンの発行）
- API が Firebase Auth の利用者を読む・消すだけのカスタムロール `synthifyApiFirebaseAuthUsers`（`firebaseauth.users.get` は招待の確認、`firebaseauth.users.delete` は退会）とその付与
- デプロイ用 SA（`github-deploy`）とそのプロジェクトのロール、実行用 SA への actAs
- デプロイ用 SA から外すロール: `resourcemanager.projectIamAdmin`、`iam.serviceAccountAdmin`、プロジェクト全体の `iam.serviceAccountTokenCreator`（`google_project_iam_member_remove`）
- API から外すロール: CD が前に付けた `firebaseauth.admin`（#75）と `firebaseauth.viewer`（#55）。上のカスタムロールに替わる。admin では、ログインの設定を変えたり、どの利用者でも作ったり書き換えたりできた
- GitHub Actions からデプロイ用 SA になる道（Workload Identity のプール `github-actions` とプロバイダ `github`）。条件は「このリポジトリの、環境と同じ名前の GitHub environment の、同じ名前のブランチのジョブ」
- デプロイ用 SA 自身の IAM ポリシー（上のプロバイダだけが使える。全体を書き換えるので、手で足した付与は消える）

バケットと secret への付与（uploads のバケットの読み書きと削除、secret の読み取り）は持たない。デプロイ用 SA は `storage.admin` と `secretmanager.admin` で、CD が作るバケットと secret の IAM を変えられるので、CD（`services/platform`）に残している。

## なぜ手で apply するのか

デプロイ用 SA は、ワークフローを動かせる人なら誰でもなれる。前は `projectIamAdmin` などを持っていて、自分に好きなロールを付けられた。つまり、ワークフローを動かせればプロジェクトを丸ごと取れた。

IAM を変える力をデプロイ用 SA から外したので、IAM はここに移し、プロジェクトのオーナーが一度だけ apply する。CD（`deploy-backend.yml`）は `terraform/environments` しか apply せず、そこに IAM の resource を書くと CI（`backend.yml`）が落ちる。

## 前提

- 自分のアカウントがプロジェクトのオーナー（`projectIamAdmin`、`serviceAccountAdmin`、`iam.workloadIdentityPoolAdmin`、`serviceUsageAdmin` があれば足りる）。
- Terraform 1.7 以上。CI と CD は 1.9.8。
- `gcloud auth application-default login` 済み。

## 今の stage・prod に入れる順番

環境ごとに、stage、次に prod の順で行う。1 と 2 の順を逆にしても壊れはしないが、その環境のデプロイが 1 を終えるまで失敗する（古いコードのデプロイが、外したロールを付け直そうとして 403 になる）。

1. **この変更をその環境にデプロイする**（stage なら main を `stage` に push）。
   - CD の Terraform は 1.9.8 になる。`removed` ブロックで、実行用 SA と IAM を CD の state から外す。消しはしない。
   - デプロイのログで、Pass 1（`Terraform apply (platform only)`）の plan を見る。実行用 SA と IAM の resource がすべて `will no longer be managed by Terraform, but will not be destroyed` で、`0 to destroy` であること。今の main をすべてデプロイした環境なら 36 件。前のデプロイが古ければ少ない（#67 のロール 3 件、#55 と #75 の Firebase Auth の付与が 1 件ずつ、まだ無い）。
   - この時点では、デプロイ用 SA の権限はまだ変わっていない。
2. **このディレクトリを apply する**（自分のアカウントで）。

   ```bash
   cd terraform/manual/iam
   ENV=stage                      # prod なら prod
   PROJECT=synthify-stage-491705  # prod なら synthify-491705
   # state はその環境の state バケットの、CD とは別の prefix に置く
   terraform init -reconfigure \
     -backend-config="bucket=${PROJECT}-tfstate-${ENV}" \
     -backend-config="prefix=synthify/${ENV}-iam"
   terraform plan  -var-file="${ENV}.tfvars"
   terraform apply -var-file="${ENV}.tfvars"
   ```

   plan で見るもの:
   - import が 2 件（プールとプロバイダ）。プロバイダは `attribute_condition` だけが変わる。
   - SA は「作成」と出るが、既にあるものを取り込むだけ（`create_ignore_already_exists`）。
   - `google_project_iam_member_remove.deploy` が 3 件。ここで 3 つのロールが外れる。
   - カスタムロール `google_project_iam_custom_role.api_firebase_auth_users` と、その API への付与が 1 件ずつ。そのあと `google_project_iam_member_remove.api` が 2 件で、API から `firebaseauth.admin` と `firebaseauth.viewer` が外れる（付いていない環境では何もしない）。
   - `google_service_account_iam_policy.deploy` が、デプロイ用 SA のポリシーを `workloadIdentityUser` の 1 つだけにする。
   - destroy が 0 件。
   - CD のデプロイ用 SA はこの state バケットにも書ける。plan に覚えの無いものがあれば apply しない。

   IAM の変更が効くまで数分かかることがある。apply の直後に招待や退会が Firebase Auth の権限のエラーで失敗したら、数分おいてやり直す。
3. **その環境のデプロイを、もう一度流す**（`stage` に push するか、Actions で直前の実行を Re-run）。backend と frontend の両方が通ることを見る。外した権限が要る操作があれば、ここで 403 になる。
4. 確かめる。

   ```bash
   # デプロイ用 SA のロール。13 個で、projectIamAdmin・serviceAccountAdmin・serviceAccountTokenCreator が無い
   gcloud projects get-iam-policy "$PROJECT" --flatten="bindings[].members" \
     --filter="bindings.members:github-deploy@" --format="value(bindings.role)"
   # プロバイダの条件
   gcloud iam workload-identity-pools providers describe github \
     --workload-identity-pool=github-actions --location=global --project "$PROJECT" \
     --format="value(attributeCondition)"
   # デプロイ用 SA 自身のポリシー。workloadIdentityUser だけ
   gcloud iam service-accounts get-iam-policy "github-deploy@${PROJECT}.iam.gserviceaccount.com" --project "$PROJECT"
   # API のロール。datastore.user、cloudtasks.enqueuer と projects/<project>/roles/synthifyApiFirebaseAuthUsers。roles/firebaseauth.* は無い
   gcloud projects get-iam-policy "$PROJECT" --flatten="bindings[].members" \
     --filter="bindings.members:synthify-api-${ENV}@" --format="value(bindings.role)"
   # カスタムロールの権限。firebaseauth.users.delete と firebaseauth.users.get だけ
   gcloud iam roles describe synthifyApiFirebaseAuthUsers --project "$PROJECT" --format="value(includedPermissions)"
   ```

CD 以外の管理の操作を知らせるアラート（`admin change outside CD`）を入れていれば、2 の操作で鳴る。自分の作業なので、記録して閉じる。

## 変わること

- デプロイ用 SA になれるのは、`stage` ブランチから environment `stage` で動くジョブ（stage のプロジェクト）と、`prod` ブランチから environment `prod` で動くジョブ（prod のプロジェクト）だけ。
- `workflow_dispatch` でほかのブランチからデプロイすることはできなくなる（GCP の認証で落ちる）。手で流すときは、ブランチに `stage` か `prod` を選ぶ。
- デプロイ用 SA は IAM を変えられない。CD の stack に IAM を足すときは、ここに足して手で apply する。
- API は Firebase Auth の利用者を読むことと消すことしかできない。ログインの設定を変えたり、利用者を作ったり書き換えたりはできない。

## 戻すとき・困ったとき

- デプロイが 403 で落ちたら、エラーの権限を見て、それを含むロールを `variables.tf` の `deploy_project_roles` に足して apply する。外した 3 つは足さない（precondition で止まる）。
- API が Firebase Auth の権限（`firebaseauth.users.*` など）で 403 になったら、その権限をカスタムロール `api_firebase_auth_users` の `permissions` に足して apply する。`firebaseauth.admin` と `firebaseauth.viewer` は API のロールに戻さない（precondition で止まる）。
- GCP の認証（Workload Identity）で落ちたら、ジョブのブランチと environment を見る。条件を一時的に戻すなら `main.tf` の `attribute_condition` を変えて apply する。急ぐなら次のコマンドで戻せるが、次の apply で条件はまた変わる。

  ```bash
  gcloud iam workload-identity-pools providers update-oidc github \
    --workload-identity-pool=github-actions --location=global --project "$PROJECT" \
    --attribute-condition="assertion.repository == 'Keyhole-Koro/Synthify'"
  ```

## 新しい環境

1. 先に state バケットを作る（`scripts/bootstrap-tfstate.sh <env> <project>`）。
2. このディレクトリに `<env>.tfvars` を作り、`adopt_existing_workload_identity = false` も入れて apply する（プールとプロバイダは取り込まずに作る）。
3. 出力の `workload_identity_provider` を GitHub の environment secret `GCP_WIF_PROVIDER` に、`deploy_service_account_email` を `GCP_WIF_SA_EMAIL` に入れる。`terraform/tfvars/<env>.tfvars` の `deployer_principal` も合わせる。
4. それから CD を流す（[terraform/README.md](../../README.md) の「初回セットアップ」）。
