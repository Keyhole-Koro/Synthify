INSERT INTO workspaces (
  workspace_id, name, owner_id, plan, storage_used_bytes, storage_quota_bytes, max_file_size_bytes, max_uploads_per_day, created_at
) VALUES (
  'ws_demo', 'Synthify Demo', 'user_demo', 'pro', 2097152, 53687091200, 524288000, 200, NOW()
) ON CONFLICT (workspace_id) DO NOTHING;

INSERT INTO workspace_members (workspace_id, user_id, email, role, is_dev, invited_at, invited_by) VALUES
  ('ws_demo', 'user_demo', 'demo@synthify.dev', 'owner', TRUE, NOW(), 'user_demo'),
  ('ws_demo', 'user_alice', 'alice@synthify.dev', 'editor', FALSE, NOW(), 'user_demo'),
  ('ws_demo', 'user_bob', 'bob@synthify.dev', 'viewer', FALSE, NOW(), 'user_demo')
ON CONFLICT (workspace_id, user_id) DO NOTHING;

INSERT INTO documents (
  document_id, workspace_id, uploaded_by, filename, mime_type, file_size, status, extraction_depth, node_count, current_stage, error_message, created_at, updated_at
) VALUES
  ('doc_sales', 'ws_demo', 'user_demo', 'sales_strategy.pdf', 'application/pdf', 524288, 'completed', 'full', 4, '', '', NOW(), NOW()),
  ('doc_market', 'ws_demo', 'user_alice', 'market_analysis.md', 'text/markdown', 131072, 'processing', '', 0, 'pass1_extraction', '', NOW(), NOW()),
  ('doc_data', 'ws_demo', 'user_demo', 'customer_data.csv', 'text/csv', 65536, 'failed', '', 0, '', 'Gemini API timeout after 3 retries (pass1_extraction)', NOW(), NOW())
ON CONFLICT (document_id) DO NOTHING;

INSERT INTO nodes (
  node_id, document_id, label, level, category, entity_type, description, summary_html, created_by, created_at
) VALUES
  ('nd_root', 'doc_sales', '販売戦略', 0, 'concept', '', '当期における販売拡大の最上位方針', '<p>主要施策として <a data-paper-id="nd_tel">テレアポ施策</a> と <a data-paper-id="nd_sns">SNS施策</a> を採用する。</p>', 'user_demo', NOW()),
  ('nd_tel', 'doc_sales', 'テレアポ施策', 1, 'concept', '', '月次100件を目標とした架電施策', '<p>月次100件を目標とした架電施策。<a data-paper-id="nd_cv">CV率 3.2%</a> を基準に改善する。</p>', 'user_demo', NOW()),
  ('nd_sns', 'doc_sales', 'SNS施策', 1, 'concept', '', 'SNSを活用したブランド認知向上施策', '<p>SNS を活用したブランド認知向上施策。<a data-paper-id="nd_cv">CV率 3.2%</a> と比較して ROI を評価する。</p>', 'user_demo', NOW()),
  ('nd_cv', 'doc_sales', 'CV率 3.2%', 2, 'entity', 'metric', 'テレアポの成約率。前期比 +0.8pp の改善', '<p>テレアポの成約率を示す指標。</p>', 'user_demo', NOW())
ON CONFLICT (node_id) DO NOTHING;

INSERT INTO edges (
  edge_id, document_id, source_node_id, target_node_id, edge_type, description, created_at
) VALUES
  ('ed_01', 'doc_sales', 'nd_root', 'nd_tel', 'hierarchical', '', NOW()),
  ('ed_02', 'doc_sales', 'nd_root', 'nd_sns', 'hierarchical', '', NOW()),
  ('ed_03', 'doc_sales', 'nd_tel', 'nd_cv', 'hierarchical', '', NOW()),
  ('ed_04', 'doc_sales', 'nd_cv', 'nd_sns', 'measured_by', 'CV率はROI比較の入力データ', NOW())
ON CONFLICT (edge_id) DO NOTHING;
