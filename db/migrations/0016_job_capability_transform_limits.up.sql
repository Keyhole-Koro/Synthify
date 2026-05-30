-- job_capabilities gains transform-specific caps so create_transform invocations
-- (and dynamic-tool executions) are bounded per job, matching the cost/recursion
-- guard the JobCapability domain type already documents. Until now these limits
-- existed only as struct fields with no column, so they were silently 0
-- (= unlimited) and never enforced. Default 0 preserves "unlimited" for rows
-- created before the values are wired.
ALTER TABLE job_capabilities
  ADD COLUMN IF NOT EXISTS max_transform_creations INTEGER NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS max_transform_runs INTEGER NOT NULL DEFAULT 0;
