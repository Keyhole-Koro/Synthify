# Interactive Canvas MVP

## Implemented Interaction Contract

Desktop Canvasは生成済みGraphを表示するだけでなく、次の編集とnavigationを提供する。

- clickでNodeを選択し、右ThreadをそのNodeへanchorする
- dragでScope-local座標を変更し、pointer release時にSQLiteへ保存する
- double clickまたは`Enter`でNodeをPortalとして開く。画面切り替えではなく、Portal位置を原点とする
  200msのdive animationで子Roomへ潜り、`Back`では同じ原点へ縮んで戻る
- breadcrumbまたは`Back`で任意の親Scopeへ戻る。祖先Roomは背後に折り紙状のstackとして残る
- `Peek`でRoomを切り替えず、summary、child、evidence、connectionを確認する
- zoom controlsでstructure、flow、implementationのsemantic levelを切り替える
- AIによるNode生成、Nodeの編集、再帰削除
- 同一Room内Edgeの作成、編集、削除

## State and Persistence

`FlowGraph`はNode、包含関係、Edgeという意味状態を持つ。`GraphLayout`はNode idごとのScope-localな`x/y`を持ち、
意味状態から分離する。rendererは編集後のdocument全体をtyped IPCへ送り、main processがZodで再検証してから
Project SQLiteへupsertする。

Node追加のprimary pathはAI generationである。子を持たないPortalへEnterすると、現在選択中providerが対象Roomだけを
自動展開する。既存NodeとEdgeはgeneration中にlockし、完全保持できない出力を拒否する。新規Nodeから別Scopeの
既存Nodeへ向かうBoundary Edgeは許可するが、既存Node同士だけの無関係なEdge追加は拒否する。

Node削除は対象Nodeとdescendant、その集合へ接続するEdge、layout entryを同じdocument mutationで除去する。
Edgeは現在Roomに表示されているNode間だけをeditor候補にする。Graph全体のNode/Edge数に固定上限は置かず、
Room単位のpatch generationと表示によって再帰的に拡張する。

## Anchored Agent Run

選択Nodeがある場合、Agent promptの先頭にProject、Graph、Room path、Node kind、summary、evidenceを付加する。
未選択時は現在Roomをanchorにする。Graph自動生成runとanchored conversation runは別runであり、Node選択の変更時に
表示transcriptを切り替える。

## Flow-oriented Layout

`layoutFlowNodes`はNode数から行列を決めず、Edgeのtopological rankで列を決める。indegree 0のNodeを起点に
各targetを一列右へ押し出し、同一rankのNodeを縦へ並べる。循環部分はtopological startを持たないため、
非循環部分の後ろへまとめて置く。

Roomは自然なpitch (`COLUMN_PITCH`, `ROW_PITCH`) でstageを構成し、そのstageをframeへfitさせる。密なRoomは
縮小されてsemantic levelが下がり、短いflowは拡大されて情報量が上がる。Portalの保存sizeは変わらない。

## Scope Projection and Boundary Ports

`projectFlowToScope`は現在Roomの外にあるEdge端点を、そのRoomに見えている祖先Portalへ写す。これにより
子孫同士のEdgeは対応するPortal間のbundled edgeとして表示され、`count`で本数を示す。

片端が現在Roomの外にあるEdgeは`scopeBoundaryPorts`がboundary portへ変換する。Room Node自身が端点である
Edgeは、Room内でflowを開始/終了するNodeへ割り当てる。boundary portはframe座標のchipとしてflowの上の余白に
置き、cardを跨がない経路で対象Portalへ接続する。Roomへ入っても入力元、出力先、前後関係は失われない。

## Portal Fold

Portalは子Scopeのlive canvasをmountしない。`buildPortalPreview`が最大5 Nodeのminiatureとその内部Edgeを
snapshotとして返し、cardの中に折り畳まれた紙として描く。表示しきれない子は`+n`で示し、childCountと
descendantCountをfooterに出す。Interaction Spec 9.6と24.2に従い、L3でも再帰的な展開は行わない。

## Semantic Zoom

semantic levelはPortalのprojected width (`PORTAL_CARD_WIDTH * 適用scale`) から決め、cardの保存sizeは変えない。
境界の上下12%をhysteresis bandとする。

- `structure`: title、kind、status、接続済みport、折り畳み件数
- `flow`: summary、Portal fold内のminiature flow、edge label、boundary port名、Peek/Enter/edit action
- `implementation`: 先頭evidence path、fold内の子Node名

zoomはGraph mutationではなくview stateであるため、Graph Revisionを作らない。manual Node positionはlayout stateとして
永続化する。

## Room Density

1 Roomの直接の子は最大`FLOWFOLD_ROOM_MAX_NODES` (7)。生成promptで4–7を要求し、超過した出力は
`balanceFlowGraphScopes`が末尾を継続Roomへ畳んで階層を深くする。Graph全体のNode数に上限は置かない。

## Renderer Preview Harness

`apps/desktop/preview`はstubしたpreload bridgeへ実際の`App`を接続する。`bun --cwd apps/desktop preview`で
agent CLIとElectronなしにCanvasを操作でき、boundary port、Portal fold、dive animationの回帰確認に使う。
