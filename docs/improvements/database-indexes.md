# Database Index Improvements

## Missing Indexes on Foreign Keys

PostgreSQL does not automatically create indexes on foreign key columns. While many foreign keys in this project have explicit indexes, some are currently missing, which can lead to full table scans during reverse lookups (finding children by parent ID).

### 1. `documents.workspace_id`
- **Reason:** Frequently used to list all documents within a workspace.
- **Impact:** As the number of documents grows, listing documents in the UI will become slower.
- **Recommendation:** `CREATE INDEX idx_documents_workspace_id ON documents(workspace_id);`

### 2. `account_users.user_id`
- **Reason:** Used to find which accounts/workspaces a specific user belongs to.
- **Impact:** Slow login/session initialization when the `account_users` table becomes large. Note: The current primary key is `(account_id, user_id)`, which only optimizes searches starting with `account_id`.
- **Recommendation:** `CREATE INDEX idx_account_users_user_id ON account_users(user_id);`

### 3. `document_processing_jobs.document_id` / `workspace_id`
- **Reason:** Used to show job history for a specific document or workspace.
- **Recommendation:** 
  - `CREATE INDEX idx_document_processing_jobs_document_id ON document_processing_jobs(document_id);`
  - `CREATE INDEX idx_document_processing_jobs_workspace_id ON document_processing_jobs(workspace_id);`

## Vector Search Performance
The current `idx_document_chunks_embedding` uses `HNSW`. This is appropriate for the current scale, but should be monitored as the number of chunks reaches hundreds of thousands.
