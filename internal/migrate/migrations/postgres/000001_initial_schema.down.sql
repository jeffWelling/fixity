-- Drop triggers
DROP TRIGGER IF EXISTS update_config_updated_at ON config;
DROP TRIGGER IF EXISTS update_webhooks_updated_at ON webhooks;
DROP TRIGGER IF EXISTS update_files_updated_at ON files;
DROP TRIGGER IF EXISTS update_storage_targets_updated_at ON storage_targets;

-- Drop function
DROP FUNCTION IF EXISTS update_updated_at_column();

-- Drop tables in reverse order (respecting foreign key constraints)
DROP TABLE IF EXISTS config;
DROP TABLE IF EXISTS webhook_deliveries;
DROP TABLE IF EXISTS webhooks;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS scan_checkpoints;
DROP TABLE IF EXISTS change_events;
DROP TABLE IF EXISTS scans;
DROP TABLE IF EXISTS files;
DROP TABLE IF EXISTS storage_targets;
