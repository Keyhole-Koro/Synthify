-- Documents & files
--   documents            : workspace 内のドキュメントメタ
--   document_files       : 物理ファイル (path, mime, size)
--   upload_reservations  : 署名付き URL 発行時の予約レコード
--   document_chunks      : チャンク分割 + 埋め込みベクタ

CREATE TABLE IF NOT EXISTS documents (
  document_id TEXT PRIMARY KEY,
  workspace_id TEXT NOT NULL REFERENCES workspaces(workspace_id) ON DELETE CASCADE,
  uploaded_by TEXT NOT NULL,
  filename TEXT NOT NULL,
  mime_type TEXT NOT NULL,
  file_size BIGINT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_documents_workspace_created_at ON documents(workspace_id, created_at DESC);

CREATE TABLE IF NOT EXISTS document_files (
  file_id     TEXT PRIMARY KEY,
  document_id TEXT NOT NULL REFERENCES documents(document_id) ON DELETE CASCADE,
  path        TEXT NOT NULL,
  mime_type   TEXT NOT NULL,
  file_size   BIGINT NOT NULL DEFAULT 0,
  created_at  TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_document_files_doc_id ON document_files(document_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_document_files_doc_path ON document_files(document_id, path);

CREATE TABLE IF NOT EXISTS upload_reservations (
  reservation_id TEXT PRIMARY KEY,
  account_id TEXT NOT NULL REFERENCES accounts(account_id) ON DELETE CASCADE,
  workspace_id TEXT NOT NULL REFERENCES workspaces(workspace_id) ON DELETE CASCADE,
  document_id TEXT NOT NULL REFERENCES documents(document_id) ON DELETE CASCADE,
  expected_size_bytes BIGINT NOT NULL,
  actual_size_bytes BIGINT NOT NULL DEFAULT 0,
  status TEXT NOT NULL,
  expires_at TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ NOT NULL,
  confirmed_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_upload_reservations_document_id ON upload_reservations(document_id);
CREATE INDEX IF NOT EXISTS idx_upload_reservations_account_status ON upload_reservations(account_id, status, expires_at);

CREATE TABLE IF NOT EXISTS document_chunks (
  chunk_id TEXT PRIMARY KEY,
  document_id TEXT NOT NULL REFERENCES documents(document_id) ON DELETE CASCADE,
  file_id TEXT REFERENCES document_files(file_id) ON DELETE CASCADE,
  heading TEXT NOT NULL DEFAULT '',
  text TEXT NOT NULL,
  source_page INTEGER,
  embedding VECTOR(768)
);

CREATE INDEX IF NOT EXISTS idx_document_chunks_document_id ON document_chunks(document_id);
CREATE INDEX IF NOT EXISTS idx_document_chunks_file_id ON document_chunks(file_id);
-- CockroachDB vector index syntax (Experimental in some versions, but GIN is not supported for VECTOR)
-- CREATE INDEX IF NOT EXISTS idx_document_chunks_embedding ON document_chunks USING GIN (embedding);
