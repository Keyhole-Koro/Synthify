CREATE TABLE IF NOT EXISTS item_source_refs (
  source_ref_id TEXT PRIMARY KEY,
  item_id TEXT NOT NULL REFERENCES tree_items(id) ON DELETE CASCADE,
  source_type TEXT NOT NULL CHECK (source_type IN ('github', 'web', 'local_file', 'chat', 'document')),
  url TEXT NOT NULL DEFAULT '',
  repo TEXT NOT NULL DEFAULT '',
  ref TEXT NOT NULL DEFAULT '',
  path TEXT NOT NULL DEFAULT '',
  line_start INTEGER,
  line_end INTEGER,
  kind TEXT NOT NULL DEFAULT '',
  external_id TEXT NOT NULL DEFAULT '',
  title TEXT NOT NULL DEFAULT '',
  confidence DOUBLE PRECISION,
  snapshot_ref TEXT NOT NULL DEFAULT '',
  content_hash TEXT NOT NULL DEFAULT '',
  metadata_json TEXT NOT NULL DEFAULT '{}',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_item_source_refs_item_id ON item_source_refs(item_id);
CREATE INDEX IF NOT EXISTS idx_item_source_refs_type ON item_source_refs(source_type);
