ALTER TABLE job_capabilities
  DROP COLUMN IF EXISTS max_transform_creations,
  DROP COLUMN IF EXISTS max_transform_runs;
