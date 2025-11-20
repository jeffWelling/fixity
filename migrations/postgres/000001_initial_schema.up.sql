-- Enable extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Storage targets table
CREATE TABLE storage_targets (
    id                              BIGSERIAL PRIMARY KEY,
    name                            TEXT NOT NULL UNIQUE,
    type                            TEXT NOT NULL CHECK (type IN ('local', 'nfs', 'smb')),
    path                            TEXT NOT NULL,
    server                          TEXT,
    share                           TEXT,
    credentials_ref                 TEXT,
    enabled                         BOOLEAN NOT NULL DEFAULT TRUE,
    scan_schedule                   TEXT,
    parallel_workers                INT NOT NULL DEFAULT 1 CHECK (parallel_workers > 0),
    random_sample_percent           DOUBLE PRECISION NOT NULL DEFAULT 1.0 CHECK (random_sample_percent >= 0 AND random_sample_percent <= 100),
    checksum_algorithm              TEXT NOT NULL DEFAULT 'md5' CHECK (checksum_algorithm IN ('md5', 'sha256', 'blake3')),
    checkpoint_interval             INT NOT NULL DEFAULT 1000 CHECK (checkpoint_interval > 0),
    batch_size                      INT NOT NULL DEFAULT 1000 CHECK (batch_size > 0),
    large_change_threshold_count    INT CHECK (large_change_threshold_count IS NULL OR large_change_threshold_count > 0),
    large_change_threshold_percent  DOUBLE PRECISION CHECK (large_change_threshold_percent IS NULL OR large_change_threshold_percent > 0),
    large_change_threshold_bytes    BIGINT CHECK (large_change_threshold_bytes IS NULL OR large_change_threshold_bytes > 0),
    created_at                      TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at                      TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_targets_enabled ON storage_targets(enabled) WHERE enabled = TRUE;

-- Files table
CREATE TABLE files (
    id                  BIGSERIAL PRIMARY KEY,
    storage_target_id   BIGINT NOT NULL REFERENCES storage_targets(id) ON DELETE CASCADE,
    path                TEXT NOT NULL,
    size                BIGINT NOT NULL CHECK (size >= 0),
    first_seen          TIMESTAMP WITH TIME ZONE NOT NULL,
    last_seen           TIMESTAMP WITH TIME ZONE NOT NULL,
    current_checksum    TEXT,
    checksum_type       TEXT CHECK (checksum_type IN ('md5', 'sha256', 'blake3')),
    last_checksummed_at TIMESTAMP WITH TIME ZONE,
    deleted_at          TIMESTAMP WITH TIME ZONE,
    created_at          TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),

    UNIQUE(storage_target_id, path),
    CHECK ((current_checksum IS NULL AND checksum_type IS NULL) OR (current_checksum IS NOT NULL AND checksum_type IS NOT NULL))
);

CREATE INDEX idx_files_target_path ON files(storage_target_id, path);
CREATE INDEX idx_files_checksum ON files(current_checksum) WHERE current_checksum IS NOT NULL;
CREATE INDEX idx_files_last_seen ON files(last_seen);
CREATE INDEX idx_files_last_checksummed ON files(last_checksummed_at) WHERE last_checksummed_at IS NOT NULL;
CREATE INDEX idx_files_deleted ON files(deleted_at) WHERE deleted_at IS NOT NULL;
CREATE INDEX idx_files_active ON files(storage_target_id, deleted_at) WHERE deleted_at IS NULL;

-- Scans table
CREATE TABLE scans (
    id                  BIGSERIAL PRIMARY KEY,
    storage_target_id   BIGINT NOT NULL REFERENCES storage_targets(id) ON DELETE CASCADE,
    status              TEXT NOT NULL CHECK (status IN ('running', 'completed', 'failed', 'partial')),
    started_at          TIMESTAMP WITH TIME ZONE NOT NULL,
    completed_at        TIMESTAMP WITH TIME ZONE,
    files_scanned       BIGINT NOT NULL DEFAULT 0 CHECK (files_scanned >= 0),
    files_added         BIGINT NOT NULL DEFAULT 0 CHECK (files_added >= 0),
    files_deleted       BIGINT NOT NULL DEFAULT 0 CHECK (files_deleted >= 0),
    files_modified      BIGINT NOT NULL DEFAULT 0 CHECK (files_modified >= 0),
    files_verified      BIGINT NOT NULL DEFAULT 0 CHECK (files_verified >= 0),
    errors_count        INT NOT NULL DEFAULT 0 CHECK (errors_count >= 0),
    error_messages      TEXT[],
    is_large_change     BOOLEAN NOT NULL DEFAULT FALSE,
    resumed_from        BIGINT REFERENCES scans(id),
    created_at          TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),

    CHECK ((status = 'running' AND completed_at IS NULL) OR (status IN ('completed', 'failed', 'partial') AND completed_at IS NOT NULL))
);

CREATE INDEX idx_scans_target ON scans(storage_target_id);
CREATE INDEX idx_scans_started ON scans(started_at DESC);
CREATE INDEX idx_scans_status ON scans(status);
CREATE INDEX idx_scans_large_change ON scans(is_large_change) WHERE is_large_change = TRUE;

-- Change events table
CREATE TABLE change_events (
    id              BIGSERIAL PRIMARY KEY,
    scan_id         BIGINT NOT NULL REFERENCES scans(id) ON DELETE CASCADE,
    file_id         BIGINT NOT NULL REFERENCES files(id) ON DELETE CASCADE,
    event_type      TEXT NOT NULL CHECK (event_type IN ('added', 'deleted', 'modified', 'verified')),
    detected_at     TIMESTAMP WITH TIME ZONE NOT NULL,
    old_checksum    TEXT,
    new_checksum    TEXT,
    old_size        BIGINT CHECK (old_size IS NULL OR old_size >= 0),
    new_size        BIGINT CHECK (new_size IS NULL OR new_size >= 0),
    created_at      TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_changes_scan ON change_events(scan_id);
CREATE INDEX idx_changes_file ON change_events(file_id);
CREATE INDEX idx_changes_type ON change_events(event_type);
CREATE INDEX idx_changes_detected ON change_events(detected_at DESC);

-- Scan checkpoints table
CREATE TABLE scan_checkpoints (
    scan_id             BIGINT PRIMARY KEY REFERENCES scans(id) ON DELETE CASCADE,
    last_processed_path TEXT NOT NULL,
    files_processed     BIGINT NOT NULL CHECK (files_processed >= 0),
    checkpoint_at       TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- Users table
CREATE TABLE users (
    id              BIGSERIAL PRIMARY KEY,
    username        TEXT NOT NULL UNIQUE,
    password_hash   TEXT NOT NULL,
    email           TEXT,
    is_admin        BOOLEAN NOT NULL DEFAULT FALSE,
    created_at      TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    last_login      TIMESTAMP WITH TIME ZONE,

    CHECK (char_length(username) >= 3 AND char_length(username) <= 50)
);

-- Sessions table
CREATE TABLE sessions (
    token       TEXT PRIMARY KEY,
    user_id     BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at  TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at  TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_sessions_user ON sessions(user_id);
CREATE INDEX idx_sessions_expires ON sessions(expires_at);

-- Webhooks table
CREATE TABLE webhooks (
    id                  BIGSERIAL PRIMARY KEY,
    name                TEXT NOT NULL,
    url                 TEXT NOT NULL,
    enabled             BOOLEAN NOT NULL DEFAULT TRUE,
    event_includes      TEXT[],
    event_excludes      TEXT[],
    retry_attempts      INT NOT NULL DEFAULT 3 CHECK (retry_attempts >= 0),
    retry_backoff_sec   INT NOT NULL DEFAULT 60 CHECK (retry_backoff_sec > 0),
    timeout_sec         INT NOT NULL DEFAULT 30 CHECK (timeout_sec > 0),
    created_at          TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- Webhook deliveries table
CREATE TABLE webhook_deliveries (
    id                  BIGSERIAL PRIMARY KEY,
    webhook_id          BIGINT NOT NULL REFERENCES webhooks(id) ON DELETE CASCADE,
    event_type          TEXT NOT NULL,
    payload             JSONB NOT NULL,
    status              TEXT NOT NULL CHECK (status IN ('pending', 'delivered', 'failed')),
    attempt             INT NOT NULL DEFAULT 0 CHECK (attempt >= 0),
    last_attempt_at     TIMESTAMP WITH TIME ZONE,
    next_retry_at       TIMESTAMP WITH TIME ZONE,
    delivered_at        TIMESTAMP WITH TIME ZONE,
    error_message       TEXT,
    created_at          TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_webhook_deliveries_status ON webhook_deliveries(status);
CREATE INDEX idx_webhook_deliveries_retry ON webhook_deliveries(next_retry_at)
    WHERE status = 'pending' AND next_retry_at IS NOT NULL;

-- Config table
CREATE TABLE config (
    key         TEXT PRIMARY KEY,
    value       TEXT NOT NULL,
    description TEXT,
    updated_at  TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_by  BIGINT REFERENCES users(id)
);

-- Insert default config values
INSERT INTO config (key, value, description) VALUES
    ('retention_days', '3650', 'Data retention period in days (default: 10 years)'),
    ('session_timeout_minutes', '60', 'Session timeout in minutes'),
    ('bcrypt_cost', '12', 'Bcrypt work factor for password hashing'),
    ('default_checksum_algorithm', 'md5', 'Default checksum algorithm for new storage targets');

-- Function to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Triggers for updated_at
CREATE TRIGGER update_storage_targets_updated_at BEFORE UPDATE ON storage_targets
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_files_updated_at BEFORE UPDATE ON files
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_webhooks_updated_at BEFORE UPDATE ON webhooks
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_config_updated_at BEFORE UPDATE ON config
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
