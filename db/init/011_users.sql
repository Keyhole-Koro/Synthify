-- Users table for centralized user info
CREATE TABLE IF NOT EXISTS users (
  user_id TEXT PRIMARY KEY,
  email TEXT NOT NULL,
  display_name TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL,
  last_login_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL
);

-- Optional: Add FK to account_users if desired, but might require migrating existing data.
-- For now, we'll keep them loosely coupled by user_id string to avoid migration pain
-- unless we are sure we can do it.
