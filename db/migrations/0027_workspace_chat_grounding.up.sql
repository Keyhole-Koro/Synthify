-- Workspace chat がドキュメント無しでも答えられるようにする。
--
-- 元の v1 は「処理済みドキュメントが無ければ回答不能」だったが、実際には
-- 資料を入れる前から使いたいという要求がある。ナレッジツリーの item も
-- 出典として使い、それも無ければモデルの一般知識で答える。
--
-- grounded は「この回答が workspace 内の出典に基づいているか」。false の
-- 回答は UI で明示する。回答本文から後追いで判定できない情報なので、
-- 行に持たせる。既存行は出典検証を通った回答なので true を既定にする。
ALTER TABLE workspace_chat_messages
  ADD COLUMN IF NOT EXISTS grounded BOOLEAN NOT NULL DEFAULT TRUE;

-- item を出典にできるようにする。chunk_id と同じく、サーバーが検証した
-- 候補集合の中の id しか入らない。
--
-- document_id は元々 NOT NULL + documents への FK だった。ツリー item だけを
-- 出典にする回答には対応する document が無く、空文字は FK 違反になるので
-- NULL を許す。FK は NULL を素通しするため、document 由来の出典に対する
-- 参照整合性はそのまま残る。
ALTER TABLE workspace_chat_message_sources
  ALTER COLUMN document_id DROP NOT NULL;

CREATE INDEX IF NOT EXISTS idx_workspace_chat_message_sources_item_id
  ON workspace_chat_message_sources(item_id);
