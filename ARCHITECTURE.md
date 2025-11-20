# Fixity - System Architecture

## Overview

Fixity is a file integrity monitoring system built in Go with a PostgreSQL backend, designed for deployment in Kubernetes. The architecture emphasizes modularity, testability, and operational robustness.

## System Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                         Web Browser                              │
│                    (User Interface)                              │
└────────────────────────┬────────────────────────────────────────┘
                         │ HTTPS (Ingress + cert-manager)
                         ▼
┌─────────────────────────────────────────────────────────────────┐
│                    Kubernetes Ingress                            │
│                    (Traefik + TLS)                               │
└────────────────────────┬────────────────────────────────────────┘
                         │ HTTP
                         ▼
┌─────────────────────────────────────────────────────────────────┐
│                  Fixity Application Pod                          │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │                   HTTP Server                             │   │
│  │  ┌──────────┬─────────────┬─────────────┬─────────────┐  │   │
│  │  │ Web UI   │ REST API    │ Health      │ Metrics     │  │   │
│  │  │ Handlers │ Handlers    │ /healthz    │ /metrics    │  │   │
│  │  │          │             │ /readyz     │             │  │   │
│  │  └──────────┴─────────────┴─────────────┴─────────────┘  │   │
│  └───────────────────────┬──────────────────────────────────┘   │
│                          │                                       │
│  ┌───────────────────────┼──────────────────────────────────┐   │
│  │              Application Core                             │   │
│  │  ┌─────────────────────────────────────────────────────┐ │   │
│  │  │ Scan Coordinator                                     │ │   │
│  │  │  - Schedule management                               │ │   │
│  │  │  - Scan queue                                        │ │   │
│  │  │  - Progress tracking                                 │ │   │
│  │  └────────────┬────────────────────────────────────────┘ │   │
│  │               │                                           │   │
│  │  ┌────────────┼────────────────────────────────────────┐ │   │
│  │  │ Scanner Engine                                       │ │   │
│  │  │  ┌──────────────┬───────────────┬─────────────────┐ │ │   │
│  │  │  │ File Walker  │ Checksum      │ Change Detector │ │ │   │
│  │  │  │              │ Computer      │                 │ │ │   │
│  │  │  └──────────────┴───────────────┴─────────────────┘ │ │   │
│  │  │  ┌──────────────────────────────────────────────── │ │   │
│  │  │  │ Random Sampler (weighted by verification age)   │ │   │
│  │  │  └──────────────────────────────────────────────── │ │   │
│  │  └──────────────────────────────────────────────────── │   │
│  │               │                                           │   │
│  │  ┌────────────┼────────────────────────────────────────┐ │   │
│  │  │ Storage Backend Abstraction                         │ │   │
│  │  │  ┌──────────┬──────────┬──────────┬──────────────┐ │ │   │
│  │  │  │ Local FS │ NFS      │ SMB/CIFS │ Future: S3   │ │ │   │
│  │  │  └──────────┴──────────┴──────────┴──────────────┘ │ │   │
│  │  └──────────────────────────────────────────────────── │   │
│  │               │                                           │   │
│  │  ┌────────────┼────────────────────────────────────────┐ │   │
│  │  │ Alert Engine                                         │ │   │
│  │  │  - Webhook dispatcher                                │ │   │
│  │  │  - Event filtering                                   │ │   │
│  │  │  - Retry logic                                       │ │   │
│  │  └──────────────────────────────────────────────────── │   │
│  │               │                                           │   │
│  │  ┌────────────┴────────────────────────────────────────┐ │   │
│  │  │ Database Layer                                       │ │   │
│  │  │  - Repository pattern                                │ │   │
│  │  │  - Query builders                                    │ │   │
│  │  │  - Transaction management                            │ │   │
│  │  └──────────────────────────────────────────────────── │   │
│  └──────────────────────┬────────────────────────────────────   │
└─────────────────────────┼────────────────────────────────────────┘
                          │ SQL
                          ▼
┌─────────────────────────────────────────────────────────────────┐
│              PostgreSQL Database (on Ra)                         │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │ Tables: files, scans, change_events, storage_targets,    │   │
│  │         users, webhooks, config, scan_checkpoints         │   │
│  └──────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
           │                    │                    │
           ▼                    ▼                    ▼
    ┌──────────┐         ┌──────────┐        ┌──────────┐
    │  Samba   │         │   NFS    │        │  Local   │
    │  Share   │         │  Mount   │        │   Path   │
    │(Synology)│         │  (Ceph)  │        │          │
    └──────────┘         └──────────┘        └──────────┘
```

## Component Design

### 1. HTTP Server Layer

**Responsibilities:**
- Serve web UI (HTML templates or static frontend)
- Handle REST API requests
- Manage authentication and sessions
- Provide health check endpoints
- Export Prometheus metrics

**Key Components:**
```go
type Server struct {
    router     *chi.Mux
    db         *Database
    scanner    *ScanCoordinator
    config     *ConfigManager
    auth       *AuthManager
    metrics    *MetricsCollector
}

// Endpoints
// UI
GET  /                     - Dashboard
GET  /scans                - Scan history
GET  /scans/{id}           - Scan details
GET  /files                - File search/browse
GET  /files/{id}           - File history
GET  /config               - Configuration UI
POST /config/targets       - Add storage target
POST /scans/trigger        - Manual scan trigger

// API
GET  /api/v1/scans         - List scans
GET  /api/v1/files         - List/search files
GET  /api/v1/changes       - List change events
POST /api/v1/webhooks/test - Test webhook

// System
GET  /healthz              - Liveness probe
GET  /readyz               - Readiness probe
GET  /metrics              - Prometheus metrics
```

### 2. Scan Coordinator

**Responsibilities:**
- Manage scan schedules (cron-based)
- Queue scans for execution
- Track scan progress and state
- Coordinate scan resumption after interruption
- Ensure only one scan per target runs at a time

**Key Components:**
```go
type ScanCoordinator struct {
    db             *Database
    scanner        *ScanEngine
    scheduler      *cron.Cron
    activeScans    map[int64]*ScanState  // target_id -> scan state
    webhooks       *WebhookDispatcher
    mu             sync.RWMutex
}

type ScanState struct {
    ScanID         int64
    TargetID       int64
    Status         ScanStatus  // running, paused, completed, failed
    StartTime      time.Time
    FilesScanned   int64
    FilesTotal     int64      // estimate
    LastCheckpoint string     // last processed path
    Errors         []ScanError
}

// Methods
func (sc *ScanCoordinator) ScheduleScan(target *StorageTarget) error
func (sc *ScanCoordinator) TriggerScan(targetID int64) error
func (sc *ScanCoordinator) PauseScan(targetID int64) error
func (sc *ScanCoordinator) ResumeScan(scanID int64) error
func (sc *ScanCoordinator) GetActiveScanStatus(targetID int64) *ScanState
```

### 3. Scanner Engine

**Responsibilities:**
- Walk filesystem for a storage target
- Compute checksums using configured algorithm
- Detect changes (new, deleted, modified files)
- Implement weighted random sampling
- Handle errors gracefully (continue on file errors)
- Create checkpoints for resumption

**Key Components:**
```go
type ScanEngine struct {
    db             *Database
    checksumWorker *ChecksumWorkerPool
    sampler        *RandomSampler
    config         *ScanConfig
}

type ScanConfig struct {
    TargetID            int64
    ChecksumAlgorithm   string  // "md5", "sha256", "blake3"
    ParallelWorkers     int     // default: 1
    RandomSamplePercent float64 // default: 1.0
    CheckpointInterval  int     // checkpoint every N files
    BatchSize           int     // database batch size
    FileTimeout         time.Duration
}

type FileRecord struct {
    Path            string
    Size            int64
    ModTime         time.Time
    Checksum        string
    ChecksumType    string
    IsNew           bool
    IsDeleted       bool
    IsModified      bool
    PreviousChecksum string
}

// Core methods
func (se *ScanEngine) Scan(ctx context.Context, target *StorageTarget) (*ScanResult, error)
func (se *ScanEngine) scanDirectory(ctx context.Context, backend StorageBackend) ([]*FileRecord, error)
func (se *ScanEngine) detectChanges(ctx context.Context, current []*FileRecord, previous map[string]*FileRecord) []*ChangeEvent
func (se *ScanEngine) selectRandomSample(unchanged []*FileRecord, samplePercent float64) []*FileRecord
```

### 4. Checksum Computer

**Responsibilities:**
- Compute file checksums using specified algorithm
- Stream files in chunks to avoid memory exhaustion
- Handle large files (100GB+) efficiently
- Implement timeout and retry logic
- Support parallel workers

**Key Components:**
```go
type ChecksumWorkerPool struct {
    workers    int
    jobs       chan *ChecksumJob
    results    chan *ChecksumResult
    wg         sync.WaitGroup
}

type ChecksumJob struct {
    Path      string
    Algorithm string
    Timeout   time.Duration
}

type ChecksumResult struct {
    Path      string
    Checksum  string
    Duration  time.Duration
    Error     error
}

// Methods
func NewChecksumWorkerPool(workers int) *ChecksumWorkerPool
func (cwp *ChecksumWorkerPool) Start(ctx context.Context)
func (cwp *ChecksumWorkerPool) Submit(job *ChecksumJob)
func (cwp *ChecksumWorkerPool) Results() <-chan *ChecksumResult
func (cwp *ChecksumWorkerPool) Stop()

// Checksum algorithms
func ComputeMD5(reader io.Reader) (string, error)
func ComputeSHA256(reader io.Reader) (string, error)
func ComputeBLAKE3(reader io.Reader) (string, error)
```

### 5. Random Sampler

**Responsibilities:**
- Select files for random verification
- Implement weighted sampling (prioritize old verification dates)
- Ensure configurable coverage percentage

**Key Components:**
```go
type RandomSampler struct {
    db *Database
}

// Methods
func (rs *RandomSampler) SelectSample(
    ctx context.Context,
    targetID int64,
    unchangedFiles []*FileRecord,
    samplePercent float64,
) ([]*FileRecord, error)

// Weighted sampling query (pseudo-SQL):
// SELECT * FROM files
// WHERE storage_target_id = $1
//   AND last_seen = current_scan_timestamp
//   AND (mtime, size) in unchanged list
// ORDER BY last_checksummed_at ASC NULLS FIRST, random()
// LIMIT (count * samplePercent)
```

### 6. Storage Backend Abstraction

**Responsibilities:**
- Provide unified interface for different storage types
- Handle authentication for SMB
- Detect mount availability
- Stream file contents for checksumming

**Key Components:**
```go
type StorageBackend interface {
    // Check if storage is accessible
    Probe(ctx context.Context) error

    // Walk all files
    Walk(ctx context.Context, fn WalkFunc) error

    // Open file for reading
    Open(ctx context.Context, path string) (io.ReadCloser, error)

    // Get file metadata
    Stat(ctx context.Context, path string) (*FileInfo, error)

    // Close/cleanup
    Close() error
}

type WalkFunc func(path string, info *FileInfo) error

type FileInfo struct {
    Path    string
    Size    int64
    ModTime time.Time
    IsDir   bool
}

// Implementations
type LocalFSBackend struct { rootPath string }
type NFSBackend struct { mountPath string }
type SMBBackend struct {
    server      string
    share       string
    credentials *SMBCredentials
    conn        *smb2.Session
}
```

### 7. Alert Engine (Webhook Dispatcher)

**Responsibilities:**
- Dispatch webhooks for configured events
- Filter events based on webhook configuration
- Implement retry logic with exponential backoff
- Track delivery success/failure

**Key Components:**
```go
type WebhookDispatcher struct {
    db          *Database
    httpClient  *http.Client
    retryQueue  chan *WebhookDelivery
}

type WebhookConfig struct {
    ID              int64
    URL             string
    Enabled         bool
    EventIncludes   []string  // if empty, all events
    EventExcludes   []string  // takes precedence
    RetryAttempts   int
    RetryBackoff    time.Duration
    Timeout         time.Duration
}

type WebhookDelivery struct {
    WebhookID   int64
    EventType   string
    Payload     []byte
    Attempt     int
    MaxAttempts int
    NextRetry   time.Time
}

// Methods
func (wd *WebhookDispatcher) Dispatch(event *Event) error
func (wd *WebhookDispatcher) shouldDispatch(webhook *WebhookConfig, eventType string) bool
func (wd *WebhookDispatcher) deliver(webhook *WebhookConfig, payload []byte) error
func (wd *WebhookDispatcher) retry(delivery *WebhookDelivery) error
```

### 8. Database Layer

**Responsibilities:**
- Provide repository pattern for data access
- Abstract SQL queries
- Manage transactions
- Handle batch operations
- Implement retention policy cleanup

**Key Components:**
```go
type Database struct {
    conn *sql.DB

    Files          *FileRepository
    Scans          *ScanRepository
    Changes        *ChangeEventRepository
    Targets        *StorageTargetRepository
    Users          *UserRepository
    Webhooks       *WebhookRepository
    Config         *ConfigRepository
    Checkpoints    *CheckpointRepository
}

// Example repository
type FileRepository struct {
    db *sql.DB
}

func (fr *FileRepository) GetByID(ctx context.Context, id int64) (*File, error)
func (fr *FileRepository) GetByPath(ctx context.Context, targetID int64, path string) (*File, error)
func (fr *FileRepository) List(ctx context.Context, filters *FileFilters) ([]*File, error)
func (fr *FileRepository) Create(ctx context.Context, file *File) error
func (fr *FileRepository) CreateBatch(ctx context.Context, files []*File) error
func (fr *FileRepository) Update(ctx context.Context, file *File) error
func (fr *FileRepository) Delete(ctx context.Context, id int64) error
func (fr *FileRepository) GetVerificationStatus(ctx context.Context, targetID int64) (*VerificationStats, error)
```

## Database Schema

### Core Tables

#### files
Stores current state of all tracked files.

```sql
CREATE TABLE files (
    id                  BIGSERIAL PRIMARY KEY,
    storage_target_id   BIGINT NOT NULL REFERENCES storage_targets(id),
    path                TEXT NOT NULL,
    size                BIGINT NOT NULL,
    first_seen          TIMESTAMP WITH TIME ZONE NOT NULL,
    last_seen           TIMESTAMP WITH TIME ZONE NOT NULL,
    current_checksum    TEXT,
    checksum_type       TEXT,  -- 'md5', 'sha256', 'blake3'
    last_checksummed_at TIMESTAMP WITH TIME ZONE,
    deleted_at          TIMESTAMP WITH TIME ZONE,  -- NULL if exists, timestamp if deleted
    created_at          TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),

    UNIQUE(storage_target_id, path)
);

CREATE INDEX idx_files_target_path ON files(storage_target_id, path);
CREATE INDEX idx_files_checksum ON files(current_checksum);
CREATE INDEX idx_files_last_seen ON files(last_seen);
CREATE INDEX idx_files_last_checksummed ON files(last_checksummed_at);
CREATE INDEX idx_files_deleted ON files(deleted_at) WHERE deleted_at IS NOT NULL;
```

#### scans
Records each scan execution.

```sql
CREATE TABLE scans (
    id                  BIGSERIAL PRIMARY KEY,
    storage_target_id   BIGINT NOT NULL REFERENCES storage_targets(id),
    status              TEXT NOT NULL,  -- 'running', 'completed', 'failed', 'partial'
    started_at          TIMESTAMP WITH TIME ZONE NOT NULL,
    completed_at        TIMESTAMP WITH TIME ZONE,
    files_scanned       BIGINT DEFAULT 0,
    files_added         BIGINT DEFAULT 0,
    files_deleted       BIGINT DEFAULT 0,
    files_modified      BIGINT DEFAULT 0,
    files_verified      BIGINT DEFAULT 0,
    errors_count        INT DEFAULT 0,
    error_messages      TEXT[],  -- Array of error messages
    is_large_change     BOOLEAN DEFAULT FALSE,
    resumed_from        BIGINT REFERENCES scans(id),  -- If resumed from interrupted scan
    created_at          TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_scans_target ON scans(storage_target_id);
CREATE INDEX idx_scans_started ON scans(started_at DESC);
CREATE INDEX idx_scans_status ON scans(status);
CREATE INDEX idx_scans_large_change ON scans(is_large_change) WHERE is_large_change = TRUE;
```

#### change_events
Records all file lifecycle events.

```sql
CREATE TABLE change_events (
    id                  BIGSERIAL PRIMARY KEY,
    scan_id             BIGINT NOT NULL REFERENCES scans(id),
    file_id             BIGINT NOT NULL REFERENCES files(id),
    event_type          TEXT NOT NULL,  -- 'added', 'deleted', 'modified', 'verified'
    detected_at         TIMESTAMP WITH TIME ZONE NOT NULL,

    -- For modifications
    old_checksum        TEXT,
    new_checksum        TEXT,
    old_size            BIGINT,
    new_size            BIGINT,

    created_at          TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_changes_scan ON change_events(scan_id);
CREATE INDEX idx_changes_file ON change_events(file_id);
CREATE INDEX idx_changes_type ON change_events(event_type);
CREATE INDEX idx_changes_detected ON change_events(detected_at DESC);
```

#### storage_targets
Configuration for monitored storage locations.

```sql
CREATE TABLE storage_targets (
    id                      BIGSERIAL PRIMARY KEY,
    name                    TEXT NOT NULL UNIQUE,
    type                    TEXT NOT NULL,  -- 'local', 'nfs', 'smb'
    path                    TEXT NOT NULL,  -- Local path or mount path

    -- Connection details (for SMB)
    server                  TEXT,
    share                   TEXT,
    credentials_ref         TEXT,  -- Reference to credential location

    -- Scan configuration
    enabled                 BOOLEAN NOT NULL DEFAULT TRUE,
    scan_schedule           TEXT,  -- Cron expression
    parallel_workers        INT NOT NULL DEFAULT 1,
    random_sample_percent   FLOAT NOT NULL DEFAULT 1.0,
    checksum_algorithm      TEXT NOT NULL DEFAULT 'md5',
    checkpoint_interval     INT NOT NULL DEFAULT 1000,
    batch_size              INT NOT NULL DEFAULT 1000,

    -- Thresholds for large change detection
    large_change_threshold_count      INT,
    large_change_threshold_percent    FLOAT,
    large_change_threshold_bytes      BIGINT,

    created_at              TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_targets_enabled ON storage_targets(enabled) WHERE enabled = TRUE;
```

#### scan_checkpoints
Enables scan resumption after interruption.

```sql
CREATE TABLE scan_checkpoints (
    scan_id             BIGINT PRIMARY KEY REFERENCES scans(id),
    last_processed_path TEXT NOT NULL,
    files_processed     BIGINT NOT NULL,
    checkpoint_at       TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);
```

#### users
Authentication and session management.

```sql
CREATE TABLE users (
    id              BIGSERIAL PRIMARY KEY,
    username        TEXT NOT NULL UNIQUE,
    password_hash   TEXT NOT NULL,  -- bcrypt
    email           TEXT,
    is_admin        BOOLEAN NOT NULL DEFAULT FALSE,
    created_at      TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    last_login      TIMESTAMP WITH TIME ZONE
);

CREATE TABLE sessions (
    token           TEXT PRIMARY KEY,
    user_id         BIGINT NOT NULL REFERENCES users(id),
    expires_at      TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at      TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_sessions_user ON sessions(user_id);
CREATE INDEX idx_sessions_expires ON sessions(expires_at);
```

#### webhooks
Webhook configuration and delivery tracking.

```sql
CREATE TABLE webhooks (
    id                  BIGSERIAL PRIMARY KEY,
    name                TEXT NOT NULL,
    url                 TEXT NOT NULL,
    enabled             BOOLEAN NOT NULL DEFAULT TRUE,
    event_includes      TEXT[],  -- Empty = all events
    event_excludes      TEXT[],
    retry_attempts      INT NOT NULL DEFAULT 3,
    retry_backoff_sec   INT NOT NULL DEFAULT 60,
    timeout_sec         INT NOT NULL DEFAULT 30,
    created_at          TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE TABLE webhook_deliveries (
    id                  BIGSERIAL PRIMARY KEY,
    webhook_id          BIGINT NOT NULL REFERENCES webhooks(id),
    event_type          TEXT NOT NULL,
    payload             JSONB NOT NULL,
    status              TEXT NOT NULL,  -- 'pending', 'delivered', 'failed'
    attempt             INT NOT NULL DEFAULT 0,
    last_attempt_at     TIMESTAMP WITH TIME ZONE,
    next_retry_at       TIMESTAMP WITH TIME ZONE,
    delivered_at        TIMESTAMP WITH TIME ZONE,
    error_message       TEXT,
    created_at          TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_webhook_deliveries_status ON webhook_deliveries(status);
CREATE INDEX idx_webhook_deliveries_retry ON webhook_deliveries(next_retry_at)
    WHERE status = 'pending' AND next_retry_at IS NOT NULL;
```

#### config
Application configuration stored in database.

```sql
CREATE TABLE config (
    key         TEXT PRIMARY KEY,
    value       TEXT NOT NULL,
    description TEXT,
    updated_at  TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_by  BIGINT REFERENCES users(id)
);

-- Example config keys:
-- 'retention_days': '3650' (10 years)
-- 'session_timeout_minutes': '60'
-- 'bcrypt_cost': '12'
-- 'default_checksum_algorithm': 'md5'
```

### Retention Policy

Automatic cleanup of old data based on retention configuration:

```sql
-- Delete change events older than retention period
DELETE FROM change_events
WHERE detected_at < NOW() - INTERVAL '1 day' * (
    SELECT value::INT FROM config WHERE key = 'retention_days'
);

-- Keep files table entries even after deletion for historical reference
-- But can optionally purge deleted files older than retention
DELETE FROM files
WHERE deleted_at IS NOT NULL
  AND deleted_at < NOW() - INTERVAL '1 day' * (
    SELECT value::INT FROM config WHERE key = 'retention_days'
);
```

## Data Flow

### Scan Execution Flow

```
1. Scheduler triggers scan (or manual trigger)
   ↓
2. ScanCoordinator creates scan record (status: 'running')
   ↓
3. ScanEngine.Scan() starts:
   a. Probe storage backend (check mount availability)
   b. Walk filesystem, collect file metadata
   c. Load previous scan state from database
   d. Detect changes (new, deleted, modified)
   e. Select random sample of unchanged files
   f. Compute checksums for changed + sampled files
   g. Batch write to database
   h. Create checkpoints periodically
   ↓
4. Change detection:
   - Compare current files with previous scan
   - Files in current but not previous → ADDED
   - Files in previous but not current → DELETED
   - Files in both with different checksum → MODIFIED
   ↓
5. Record changes in change_events table
   ↓
6. Update files table:
   - New files: INSERT
   - Deleted files: UPDATE deleted_at
   - Modified files: UPDATE checksum, last_checksummed_at
   - Unchanged sampled files: UPDATE last_checksummed_at
   - Unchanged non-sampled: UPDATE last_seen only
   ↓
7. Evaluate large change thresholds
   ↓
8. Update scan record (status: 'completed', statistics)
   ↓
9. WebhookDispatcher processes events:
   - file.added for each new file
   - file.deleted for each deleted file
   - file.modified for each modified file
   - large_change.detected if threshold exceeded
   - scan.completed
   ↓
10. UI updates to reflect new scan results
```

### Scan Resumption Flow

```
1. Application starts, loads incomplete scans from database
   ↓
2. For each scan with status 'running':
   a. Load checkpoint (last_processed_path)
   b. Create new scan record with resumed_from reference
   c. Continue walking from last checkpoint
   d. Process remaining files
   ↓
3. Update original scan status to 'partial'
   ↓
4. Complete resumed scan normally
```

## Security Considerations

### Authentication & Authorization

1. **Password Storage**: bcrypt with work factor 12 (configurable)
2. **Session Management**:
   - Secure HTTP-only cookies
   - CSRF protection
   - Configurable timeout (default: 60 minutes)
3. **API Authentication**: Session cookies or bearer tokens (future)

### Credential Management

1. **Storage Target Credentials**:
   - Reference external files (Kubernetes Secrets)
   - Encrypt sensitive fields in database with AES-256
   - Never log credentials

2. **Credential Rotation**:
   - Support updating credentials without recreating targets
   - Validate credentials before saving

### Database Security

1. **Connection**: TLS/SSL for PostgreSQL connection
2. **Least Privilege**: Application uses dedicated PostgreSQL user with minimal permissions
3. **SQL Injection**: Use parameterized queries (no string concatenation)

### Network Security

1. **Ingress**: HTTPS via cert-manager (Let's Encrypt)
2. **Internal**: HTTP within cluster (Ingress terminates TLS)
3. **Database**: PostgreSQL on internal cluster network only

## Performance Optimization

### Checksum Computation

1. **Streaming I/O**: Read files in 16MB chunks
2. **Worker Pool**: Parallel checksumming (configurable, default 1)
3. **Smart Skipping**: Use mtime/size to avoid unnecessary checksums
4. **Timeout**: Per-file timeout (5 minutes for large files)

### Database Performance

1. **Batch Operations**: Insert/update in batches of 1000 (configurable)
2. **Indexes**: Composite indexes on high-traffic queries
3. **Connection Pooling**: Reuse database connections
4. **Partitioning**: Consider partitioning change_events by date (future)

### Memory Management

1. **Streaming**: Never load entire files into memory
2. **Batch Processing**: Process files in batches, not all at once
3. **Bounded Channels**: Limit queue sizes to prevent unbounded growth

## Monitoring & Observability

### Prometheus Metrics

```
# Scan metrics
fixity_scan_duration_seconds{target="synology-media"}
fixity_scan_files_total{target="synology-media",type="scanned|added|deleted|modified|verified"}
fixity_scan_errors_total{target="synology-media"}
fixity_scan_last_success_timestamp{target="synology-media"}

# File metrics
fixity_files_total{target="synology-media",status="active|deleted"}
fixity_files_verification_age_seconds{target="synology-media",quantile="0.5|0.9|0.99"}

# System metrics
fixity_active_scans{target="synology-media"}
fixity_checksum_duration_seconds{algorithm="md5|sha256|blake3"}
fixity_database_queries_total{operation="select|insert|update|delete"}
fixity_webhook_deliveries_total{webhook="webhook1",status="success|failure"}
```

### Structured Logging

```json
{
  "level": "info",
  "timestamp": "2025-11-17T14:32:00Z",
  "component": "scanner",
  "scan_id": 12345,
  "target_id": 1,
  "target_name": "synology-media",
  "message": "Scan completed successfully",
  "files_scanned": 500000,
  "files_added": 1500,
  "files_deleted": 300,
  "files_modified": 50,
  "duration_seconds": 7200
}
```

## Deployment

### Kubernetes Manifests

```yaml
# Deployment
apiVersion: apps/v1
kind: Deployment
metadata:
  name: fixity
  namespace: monitoring
spec:
  replicas: 1
  selector:
    matchLabels:
      app: fixity
  template:
    metadata:
      labels:
        app: fixity
    spec:
      containers:
      - name: fixity
        image: fixity:latest
        ports:
        - containerPort: 8080
        env:
        - name: DATABASE_URL
          valueFrom:
            secretKeyRef:
              name: fixity-db
              key: url
        - name: SMB_CREDENTIALS
          value: /etc/fixity/creds/smb.yaml
        volumeMounts:
        - name: credentials
          mountPath: /etc/fixity/creds
          readOnly: true
        - name: nfs-ceph
          mountPath: /mnt/ceph
        livenessProbe:
          httpGet:
            path: /healthz
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /readyz
            port: 8080
          initialDelaySeconds: 10
          periodSeconds: 5
        resources:
          requests:
            cpu: 500m
            memory: 512Mi
          limits:
            cpu: 4000m
            memory: 2Gi
      volumes:
      - name: credentials
        secret:
          secretName: fixity-creds
      - name: nfs-ceph
        nfs:
          server: ceph.homelab.justdev.ca
          path: /backup
---
# Service
apiVersion: v1
kind: Service
metadata:
  name: fixity
  namespace: monitoring
spec:
  selector:
    app: fixity
  ports:
  - port: 80
    targetPort: 8080
---
# Ingress
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: fixity
  namespace: monitoring
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-prod
spec:
  rules:
  - host: fixity.homelab.justdev.ca
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: fixity
            port:
              number: 80
  tls:
  - hosts:
    - fixity.homelab.justdev.ca
    secretName: fixity-tls
```

## Migration Strategy

### Database Migrations

Using `golang-migrate`:

```
migrations/
├── 000001_initial_schema.up.sql
├── 000001_initial_schema.down.sql
├── 000002_add_verification_tracking.up.sql
├── 000002_add_verification_tracking.down.sql
└── ...
```

Migration execution:
```bash
migrate -path ./migrations -database "postgres://fixity:password@ra:5432/fixity?sslmode=require" up
```

## Testing Strategy

### Unit Tests
- Scanner logic (change detection, sampling)
- Checksum computation
- Database repositories (use test database)
- Webhook filtering and retry logic

### Integration Tests
- End-to-end scan execution
- Database transactions
- Storage backend integration
- Webhook delivery

### Performance Tests
- Scan 100k files (benchmark)
- Concurrent checksum computation
- Database batch operations

---

**Document Status**: Draft v1.0
**Last Updated**: 2025-11-17
**Author**: Jeff (with Claude Code assistance)
