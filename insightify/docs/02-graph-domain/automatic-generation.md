# Automatic FlowFold Graph Generation

## Decision

Projectを選択しただけではAIを起動しない。保存済みGraphがあれば即座に復元し、なければempty stateに
`Generate with Codex` または `Generate with Antigravity` を表示する。ユーザーの明示操作で、現在選択している
providerへ一度だけ初期Graph生成を依頼する。

AIが決めるのはNode、包含関係、Edge、summary、evidenceという意味構造である。座標はAIへ生成させず、同じ
Graphなら同じ結果になるdeterministic layoutをDesktop側で適用する。これにより再生成時の不要な位置揺れと、
model固有のレイアウト差を避ける。

## Pipeline

1. Desktop main processが選択Projectをopaque idから解決する。
2. Git管理対象と未追跡ファイルの一覧から、安全化したbounded snapshotを作る。
3. secret候補、dependency、binary、巨大ファイル、symlinkを除外し、snapshot hashを計算する。
4. 選択providerをJSON Schema付きで起動する。
5. CodexとAntigravity CLIは空の一時directoryから、prompt内のsnapshotだけを解析する。Codex turnはread-onlyかつnetwork disabledとする。
6. provider出力をZodで再検証し、参照切れ、重複id、self edgeを拒否する。
7. 検証済みGraph、provider、snapshot hash、生成時刻をDesktop SQLiteへupsertする。
8. rendererへtyped eventを送り、root NodeをFlowFold Canvasへ配置する。

初回生成はrootごとに子Nodeも作る。子を持たないPortalへEnterした場合は、同じpipelineをscope expansion modeで
実行する。providerは新規Node/Edgeだけのappend-only patchを返し、main processは新規Nodeが指定Roomのdescendantで
あり、新規Edgeの片端が新規Nodeであることを検証してから現在Graphへmergeする。

## Current MVP Contract

- Node: `id`, `title`, `summary`, `kind`, `parentId`, `evidence`
- Edge: `source`, `target`, `label`
- Graph全体のNode/Edge数に固定上限を置かない。rootはoverviewで3–5個を推奨
- Canvasには現在ScopeのNodeを表示し、Portalからchild ScopeへEnterできる
- empty PortalへEnterすると選択中providerがそのRoomを自動展開する
- Graph生成と通常のanchored conversationは別runとして扱う
- 再生成は現在のGraphを、検証と永続化が成功した時点で原子的に置換する

## Failure Behavior

snapshot、provider起動、schema validation、SQLite保存のどこかが失敗した場合、既存Graphは維持する。生成runを
failedとしてUIへ返し、未検証のJSONをGraphとして表示・保存しない。cancel時も既存Graphを変更しない。

## Next Steps

- 生成前snapshot previewと含有ファイルの除外操作
- 再生成結果を直接置換せずChangeSetとしてdiff表示
- evidenceからArtifact Linkへの昇格
- 生成Graphと手動編集GraphのChangeSet diff
- snapshot hashによるstale表示と差分再解析
