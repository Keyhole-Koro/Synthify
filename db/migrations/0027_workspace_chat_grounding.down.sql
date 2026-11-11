DROP INDEX IF EXISTS idx_workspace_chat_message_sources_item_id;

-- NOT NULL に戻す前に、item 由来の出典 (document_id IS NULL) を落とす。
-- 残したまま制約を戻すと migration が失敗する。
DELETE FROM workspace_chat_message_sources WHERE document_id IS NULL;

ALTER TABLE workspace_chat_message_sources
  ALTER COLUMN document_id SET NOT NULL;

ALTER TABLE workspace_chat_messages
  DROP COLUMN IF EXISTS grounded;
