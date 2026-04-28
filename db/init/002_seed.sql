-- Seed data for development
INSERT INTO accounts (account_id, name, plan, created_at, updated_at)
VALUES ('acc_seed_1', 'Demo Account', 'pro', NOW(), NOW())
ON CONFLICT (account_id) DO NOTHING;

-- Link a default user ID to this account
INSERT INTO account_users (account_id, user_id, role, joined_at)
VALUES ('acc_seed_1', 'user_seed_1', 'admin', NOW())
ON CONFLICT (account_id, user_id) DO NOTHING;

INSERT INTO workspaces (workspace_id, account_id, name, created_at, updated_at)
VALUES 
  ('ws_seed_1', 'acc_seed_1', 'Research & Development', NOW(), NOW()),
  ('ws_seed_2', 'acc_seed_1', 'Archived Project X', NOW(), NOW())
ON CONFLICT (workspace_id) DO NOTHING;

-- Add a hierarchical tree for Workspace 1
INSERT INTO tree_items (id, workspace_id, parent_id, label, level, description, summary_html, created_by, created_at, updated_at)
VALUES 
  ('root_1', 'ws_seed_1', NULL, 'Master Blueprint', 0, 'Project main architecture', '<h1>Main</h1>', 'system', NOW(), NOW()),
  ('node_1_1', 'ws_seed_1', 'root_1', 'Frontend Architecture', 1, 'React/Next.js setup', '<p>Details about frontend...</p>', 'system', NOW(), NOW()),
  ('node_1_2', 'ws_seed_1', 'root_1', 'Backend Services', 1, 'Go microservices', '<p>Details about backend...</p>', 'system', NOW(), NOW()),
  ('node_1_1_1', 'ws_seed_1', 'node_1_1', 'Atomic Design', 2, 'Component pattern', '<span>Atomic...</span>', 'system', NOW(), NOW()),
  ('node_1_2_1', 'ws_seed_1', 'node_1_2', 'PostgreSQL Schema', 2, 'DB Design', '<span>Schema...</span>', 'system', NOW(), NOW())
ON CONFLICT (id) DO NOTHING;

INSERT INTO documents (document_id, workspace_id, uploaded_by, filename, mime_type, file_size, created_at)
VALUES 
  ('doc_seed_1', 'ws_seed_1', 'system', 'annual_report_2024.pdf', 'application/pdf', 1024000, NOW() - INTERVAL '1 hour'),
  ('doc_seed_2', 'ws_seed_1', 'system', 'technical_spec_v2.docx', 'application/vnd.openxmlformats-officedocument.wordprocessingml.document', 512000, NOW() - INTERVAL '30 minutes'),
  ('doc_seed_3', 'ws_seed_1', 'system', 'contract_legal_v1.pdf', 'application/pdf', 2048000, NOW() - INTERVAL '2 hours'),
  ('doc_seed_4', 'ws_seed_2', 'system', 'old_spec.pdf', 'application/pdf', 500000, NOW() - INTERVAL '10 days')
ON CONFLICT (document_id) DO NOTHING;

INSERT INTO document_processing_jobs (
  job_id, document_id, workspace_id, job_type, status, current_stage, created_at, updated_at
) VALUES 
  ('job-audit-demo-1', 'doc_seed_1', 'ws_seed_1', 'process_document', 'succeeded', 'completed', NOW() - INTERVAL '1 hour', NOW() - INTERVAL '55 minutes'),
  ('job-audit-demo-2', 'doc_seed_2', 'ws_seed_1', 'process_document', 'running', 'semantic_chunking', NOW() - INTERVAL '30 minutes', NOW() - INTERVAL '30 minutes'),
  ('job-audit-demo-3', 'doc_seed_3', 'ws_seed_1', 'process_document', 'failed', 'error', NOW() - INTERVAL '2 hours', NOW() - INTERVAL '1 hour 50 minutes')
ON CONFLICT (job_id) DO NOTHING;

-- Seed logs for the first job
INSERT INTO job_mutation_logs (
  mutation_id, job_id, workspace_id, target_type, target_id, mutation_type, before_json, after_json, provenance_json, created_at
) VALUES 
  ('m1', 'job-audit-demo-1', 'ws_seed_1', 'agent', 'Orchestrator', 'start', '{}', '{}', '{}', NOW() - INTERVAL '10 minutes'),
  ('m2', 'job-audit-demo-1', 'ws_seed_1', 'tool_call', 'manage_job_checklist', 'execute', '{"action": "add", "description": "Analyze document structure"}', '{"status": "ok", "task_id": "T1"}', '{"duration_ms": 150}', NOW() - INTERVAL '9 minutes'),
  ('m3', 'job-audit-demo-1', 'ws_seed_1', 'tool_call', 'extract_text', 'execute', '{"file_uri": "gs://bucket/doc.pdf"}', '{"raw_text": "Extracted content..."}', '{"duration_ms": 1200}', NOW() - INTERVAL '8 minutes'),
  ('m4', 'job-audit-demo-1', 'ws_seed_1', 'tool_call', 'semantic_chunking', 'execute', '{"raw_text": "..."}', '{"chunks": [{"index":0, "text": "..."}]}', '{"duration_ms": 2500}', NOW() - INTERVAL '7 minutes'),
  ('m5', 'job-audit-demo-1', 'ws_seed_1', 'tool_call', 'goal_driven_synthesis', 'execute', '{"document_brief": "Master blueprint..."}', '{"items": [{"label": "Concept A", "level": 0}]}', '{"duration_ms": 5000}', NOW() - INTERVAL '5 minutes'),
  ('m6', 'job-audit-demo-1', 'ws_seed_1', 'tool_call', 'quality_critique', 'execute', '{"target_data": "..."}', '{"valid": true, "issues": []}', '{"duration_ms": 3200}', NOW() - INTERVAL '2 minutes'),
  ('m7', 'job-audit-demo-1', 'ws_seed_1', 'tool_call', 'persist_knowledge_tree', 'execute', '{"items": []}', '{"success": true}', '{"duration_ms": 450}', NOW() - INTERVAL '1 minute')
ON CONFLICT (mutation_id) DO NOTHING;
