-- Workspace chat: workspace 内の完了済みドキュメント / ナレッジツリーを対象にした
-- read-only の質問応答。
--
--   workspace_chat_conversations  : 会話単位
--   workspace_chat_messages       : user / assistant のメッセージ履歴
--   workspace_chat_message_sources: assistant メッセージが引用した根拠
--
-- 設計は docs/architecture/workspace-chat-design.md を参照。
-- 会話へのアクセス権は workspace へのアクセス権と同一とする。created_by は
-- 監査用であって可視性の判定には使わない (private 会話が必要になった時点で
-- 明示的な visibility カラムを migration で追加する)。

CREATE TABLE IF NOT EXISTS workspace_chat_conversations (
  conversation_id TEXT PRIMARY KEY,
  workspace_id    TEXT NOT NULL REFERENCES workspaces(workspace_id) ON DELETE CASCADE,
  created_by      TEXT NOT NULL,
  title           TEXT NOT NULL DEFAULT '',
  created_at      TIMESTAMPTZ NOT NULL,
  updated_at      TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_workspace_chat_conversations_workspace_updated_at
  ON workspace_chat_conversations(workspace_id, updated_at DESC);

CREATE TABLE IF NOT EXISTS workspace_chat_messages (
  message_id      TEXT PRIMARY KEY,
  conversation_id TEXT NOT NULL REFERENCES workspace_chat_conversations(conversation_id) ON DELETE CASCADE,
  role            TEXT NOT NULL CHECK (role IN ('user', 'assistant')),
  content         TEXT NOT NULL,
  -- 'gemini' 固定で書き始めるが、後から local provider を足したときに
  -- 「どのモデルが答えたか」を遡って監査できるようメッセージ単位で持つ。
  model_selection TEXT NOT NULL DEFAULT 'gemini',
  -- 生成失敗 / キャンセルを行の欠落ではなく状態として表現する。
  -- 'complete' | 'failed' | 'cancelled'。user メッセージは常に 'complete'。
  status          TEXT NOT NULL DEFAULT 'complete'
                  CHECK (status IN ('complete', 'failed', 'cancelled')),
  error_code      TEXT NOT NULL DEFAULT '',
  -- retrieval の再現用の内部記録。API レスポンスには出さない。
  -- source id / retrieval strategy / 実効モデルのみを入れ、資格情報や
  -- 隠しプロンプト本文は入れない。
  retrieval_snapshot_json JSONB NOT NULL DEFAULT '{}',
  created_at      TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_workspace_chat_messages_conversation_created_at
  ON workspace_chat_messages(conversation_id, created_at);

CREATE TABLE IF NOT EXISTS workspace_chat_message_sources (
  message_id  TEXT NOT NULL REFERENCES workspace_chat_messages(message_id) ON DELETE CASCADE,
  ordinal     INTEGER NOT NULL,
  document_id TEXT NOT NULL REFERENCES documents(document_id) ON DELETE CASCADE,
  chunk_id    TEXT NOT NULL DEFAULT '',
  item_id     TEXT NOT NULL DEFAULT '',
  label       TEXT NOT NULL,
  PRIMARY KEY (message_id, ordinal)
);

CREATE INDEX IF NOT EXISTS idx_workspace_chat_message_sources_document_id
  ON workspace_chat_message_sources(document_id);
