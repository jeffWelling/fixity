# Fixity - File Integrity & Lifecycle Monitoring System

## Project Overview

**Fixity** is a file integrity monitoring and lifecycle tracking system designed to detect missing, deleted, modified, or corrupted files across multiple storage backends. Named after the archival science term for checksum-based integrity verification, Fixity provides a comprehensive audit trail of file changes over time.

## Problem Statement

### The Core Problem

Files are mysteriously disappearing from storage systems (Synology NAS and Ceph cluster) without clear explanation. When files go missing, there is no:
- Record of when the deletion occurred
- Information about what was deleted (filename, size, checksum)
- Ability to distinguish between intentional deletion and data loss
- Historical context to identify patterns or root causes

Additionally, silent data corruption can occur without detection:
- Files may become corrupted without changing size or modification time
- Traditional backup verification only checks that files exist, not that they're intact
- No periodic integrity verification happens for stable files

### Current Gaps

Existing solutions don't meet requirements:
- **File Integrity Monitoring (FIM) tools** (Wazuh, AIDE, OSSEC): Security-focused, designed to detect unauthorized tampering, not track legitimate file lifecycle or provide user-friendly historical visualization
- **Backup verification tools** (Duplicati, Bacula): Verify backup integrity but don't track changes over time or provide change analytics
- **File synchronization tools** (Syncthing, rsync): Focus on keeping files in sync, not on historical tracking or forensics

## Goals & Objectives

### Primary Goals

1. **Detect and record file lifecycle events**: Track when files are added, modified, or deleted across all monitored storage locations
2. **Verify file integrity over time**: Use checksum verification to detect silent corruption or unauthorized modifications
3. **Provide historical audit trail**: Maintain 10-year history of all file changes for forensic analysis
4. **Enable root cause analysis**: Help identify patterns in file disappearances and determine if deletions were intentional or accidental
5. **Alert on anomalies**: Notify when files disappear, when large-scale changes occur, or when file integrity verification fails

### Secondary Goals

1. **Support multiple storage backends**: Seamlessly monitor local filesystems, NFS, and SMB/CIFS shares
2. **Scale to large collections**: Handle 100k-1M+ files efficiently with reasonable scan times
3. **Minimize performance impact**: Default to conservative resource usage to avoid starving other processes
4. **Provide actionable insights**: Web UI for exploring changes, filtering events, and understanding file history
5. **Enable automation**: Webhook integration for alerting and integration with other systems

## Requirements

### Functional Requirements

#### Core Functionality

1. **File Scanning**
   - Scan configured storage targets on scheduled intervals (default: daily)
   - Support local filesystem, NFS, and SMB/CIFS backends
   - Record file metadata: path, size, modification time, checksum
   - Detect new files, deleted files, and modified files (content changes)

2. **Checksum Verification**
   - Compute full file checksums (configurable algorithm: MD5, SHA-256, BLAKE3)
   - Use modification time and size as first-pass change detection (performance optimization)
   - Perform weighted random sampling: periodically re-verify checksums of apparently unchanged files
   - Prioritize files not recently verified for random sampling
   - Track when each file was last checksummed

3. **Change Detection & Recording**
   - Record four event types:
     - **Added**: New file discovered
     - **Deleted**: Previously tracked file no longer exists
     - **Modified**: File content changed (same path, different checksum)
     - **Verified**: File checksummed and confirmed unchanged (for tracking verification age)
   - Store file lifecycle history: creation date, modification events, deletion date
   - Preserve metadata of deleted files for forensic analysis

4. **Storage Backend Management**
   - Configure multiple storage targets with independent settings
   - Support different credentials per share (SMB authentication)
   - Handle mount unavailability gracefully (mark scan as failed, don't false-positive as deletions)
   - Track scan health: success, partial failure, complete failure

5. **Large Change Detection**
   - Configurable thresholds for "large changes":
     - Absolute count (e.g., >1000 files changed)
     - Percentage (e.g., >5% of inventory changed)
     - Size-based (e.g., >100GB deleted)
   - Flag scans with large changes for user review
   - Special UI treatment for anomalous scans

6. **Webhook Integration**
   - Trigger webhooks for configurable event types:
     - `file.deleted` - File removed
     - `file.added` - New file discovered
     - `file.modified` - File content changed
     - `large_change.detected` - Threshold exceeded
     - `scan.completed` - Scan finished successfully
     - `scan.failed` - Scan failed (mount unavailable, errors)
     - `verification.stale` - Files exceed verification age threshold
   - Event filtering: include/exclude specific event types per webhook
   - Retry logic with exponential backoff for failed deliveries

#### Web User Interface

1. **Authentication**
   - Username/password authentication (bcrypt hashed)
   - Session management with configurable timeout
   - Audit log of user logins and configuration changes

2. **Dashboard View**
   - High-level overview of changes over the last year
   - Grouped by deletions (top, highlighted red) and additions (bottom, highlighted green)
   - Visual indicators for scans with large changes (alert badges, color coding)
   - Verification status summary: files verified in last 7/30/90+ days
   - Per-storage-target statistics

3. **Scan Log View**
   - Browse scans by date/time
   - View files added, deleted, modified in each scan
   - Scan metadata: duration, files scanned, errors encountered
   - Filter by storage target, date range, event type

4. **File History View**
   - Search/filter files by path pattern, size, date range
   - View complete lifecycle for any file:
     - Creation date and initial checksum
     - Modification events with old/new checksums
     - Deletion date (if applicable)
     - Last verification timestamp
   - Show verification age with color coding (green/yellow/orange/red)

5. **Configuration UI**
   - Manage storage targets: add/edit/disable monitoring locations
   - Configure scan schedules per target (cron syntax)
   - Set checksum algorithm, parallel workers, random sample percentage
   - Configure large change thresholds
   - Manage webhooks: URL, event filters, retry settings
   - Set data retention policy (default 10 years)
   - Batch size for database operations

6. **Advanced Features**
   - Export change reports (CSV, JSON)
   - Timeline visualization of additions/deletions over time
   - Diff view: compare file inventory between two dates
   - Manual scan triggering
   - "Re-verify old files" action: force checksum of files exceeding verification age

#### Configuration Management

1. **Database-Backed Configuration**
   - Store configuration in PostgreSQL (not config files)
   - Single source of truth: database
   - Allows HA deployment: multiple pods read from shared database
   - Configuration changes via Web UI persist to database immediately

2. **Credential Management**
   - Prefer environment variables or mounted files (Kubernetes ConfigMap/Secret)
   - Main config references credential file paths (not inline credentials)
   - Support different credentials per storage target
   - Encrypt sensitive fields in database (AES-256)

3. **Bootstrap Configuration**
   - Minimal startup config: database URL and credentials (env vars or CLI flags)
   - All other configuration loaded from database
   - Database initialization on first run

### Non-Functional Requirements

#### Performance

1. **Scalability**
   - Handle 1M+ files efficiently
   - Daily scans complete within reasonable timeframe (target: <4 hours for 1M files with 1% churn)
   - Initial baseline scan acceptable to run over weekend (~24-48 hours)

2. **Resource Efficiency**
   - Configurable parallelism (default: 1 worker, conservative)
   - Streaming file I/O with chunked reading (16MB buffers)
   - Batch database operations (configurable batch size, default: 1000)
   - Efficient indexing strategy for fast queries

3. **Resilience**
   - Checkpoint scan progress every N files (configurable, default: 1000)
   - Resume interrupted scans from last checkpoint
   - Per-file read timeout (5 minutes for large files)
   - Retry failed files up to 3 times with exponential backoff
   - Graceful handling of partial failures (continue scanning remaining files)

4. **Database Performance**
   - Composite indexes on high-traffic columns
   - Batch inserts for new files (hundreds at a time)
   - Efficient queries for random sampling (ORDER BY last_checksummed_at)
   - Automatic retention policy enforcement (scheduled cleanup of old data)

#### Deployment

1. **Kubernetes-Native**
   - Run as Kubernetes pod (Deployment)
   - Health endpoints: `/healthz` (liveness), `/readyz` (readiness)
   - Graceful shutdown on SIGTERM (save checkpoint, close connections)
   - TerminationGracePeriodSeconds: 300 (allow time to checkpoint)

2. **High Availability**
   - Stateless application design (state in PostgreSQL)
   - Multiple replicas possible (with scan coordination/locking)
   - Database-backed configuration enables shared config across pods

3. **Observability**
   - Structured JSON logging with configurable levels
   - Prometheus metrics export (`/metrics`):
     - `fixity_scan_duration_seconds{target}`
     - `fixity_files_scanned_total{target}`
     - `fixity_changes_detected_total{type, target}`
     - `fixity_checksum_errors_total{target}`
     - `fixity_scan_queue_length`
   - Detailed error logging for troubleshooting

4. **Security**
   - No HTTPS required (handled by Kubernetes Ingress with cert-manager)
   - Bcrypt password hashing (configurable work factor)
   - Secure session cookies
   - Audit logging for security events
   - Encrypted credentials in database

#### Code Quality

1. **Go Best Practices**
   - Idiomatic Go code style (gofmt, golint)
   - Comprehensive error handling
   - Context-based cancellation for graceful shutdown
   - Proper use of goroutines and channels

2. **Modularity**
   - Clear separation of concerns:
     - Scanner (file discovery, checksumming)
     - Storage backends (local, NFS, SMB abstraction)
     - Database layer (queries, migrations)
     - API/HTTP handlers (web UI, REST API)
     - Webhook dispatcher
     - Configuration manager
   - Interfaces for testability (mock storage, mock database)
   - Dependency injection

3. **Testing**
   - Unit tests for core logic
   - Integration tests with test database
   - End-to-end tests for critical workflows
   - Table-driven tests where appropriate
   - Target: >80% code coverage

4. **Documentation**
   - Comprehensive README with setup instructions
   - GoDoc comments for all exported functions
   - Architecture documentation (this file and ARCHITECTURE.md)
   - API documentation (OpenAPI/Swagger)
   - Deployment guide for Kubernetes

## Scope

### In Scope - Phase 1 (MVP)

1. Local filesystem scanning
2. PostgreSQL database backend
3. Basic web UI with authentication
4. File lifecycle tracking (add/delete/modify events)
5. Full file checksumming with MD5 (configurable algorithm)
6. Weighted random sampling for verification
7. Scheduled scans (cron-based)
8. Dashboard and scan log views
9. Basic configuration UI
10. Webhook support for file.deleted events

### In Scope - Phase 2

1. NFS and SMB/CIFS backend support
2. Advanced UI features (timeline, diff view, export)
3. Large change detection and alerting
4. Comprehensive webhook event types
5. Prometheus metrics export
6. Kubernetes deployment manifests
7. Scan resumption after interruption
8. Performance optimizations (parallel workers, batching)

### In Scope - Phase 3

1. Multi-user support with role-based access control
2. API-first design with full REST API
3. Advanced analytics (file age distribution, growth trends)
4. File deduplication detection (same hash, multiple paths)
5. Symbolic link and hard link handling
6. Custom retention policies per storage target
7. Automated verification campaigns ("verify all files >90 days old")
8. Integration with external systems (Prometheus Alertmanager, Slack, etc.)

### Out of Scope

1. **File restoration**: Fixity only tracks changes, does not backup or restore files
2. **Real-time monitoring**: Uses scheduled scans, not inotify/fsnotify (performance at scale)
3. **Content deduplication**: Does not deduplicate storage, only tracks duplicate hashes
4. **Encryption at rest**: Database encryption is PostgreSQL's responsibility
5. **Multi-tenancy**: Single organization/user model (no tenant isolation)
6. **Mobile app**: Web UI only, responsive design for mobile browsers

## Success Criteria

### MVP Success (Phase 1)

- [ ] Successfully tracks 100k+ files on local filesystem
- [ ] Detects and records file additions, deletions, and modifications
- [ ] Computes full checksums with configurable algorithm (MD5 default)
- [ ] Random sampling verifies 1% of unchanged files per scan
- [ ] Web UI displays scan history and file lifecycle events
- [ ] Scans complete in reasonable time (<2 hours for 100k files on fast storage)
- [ ] Database stores 10 years of history with <10GB storage
- [ ] Webhooks trigger on file deletion events
- [ ] Deployment on Kubernetes with Ingress

### Full Project Success (Phase 3)

- [ ] Monitors 1M+ files across NAS (SMB) and Ceph (NFS)
- [ ] Daily scans complete in <4 hours
- [ ] Zero false positives for deletions (mount unavailability handled correctly)
- [ ] Detects at least one instance of silent corruption via random sampling
- [ ] Identifies root cause of mysterious file deletions through historical analysis
- [ ] Large change alerts prevent accidental mass deletions
- [ ] Integration with homelab monitoring (Prometheus/Grafana)
- [ ] 99.9% scan success rate over 90 days

## Technology Stack

### Backend
- **Language**: Go 1.21+
- **Database**: PostgreSQL 15+ (existing server on Ra)
- **Migrations**: golang-migrate or goose
- **Web Framework**: Go standard library (net/http) or Chi router
- **Logging**: zerolog (structured JSON logging)
- **Configuration**: Viper or custom solution

### Frontend
- **UI Framework**: Go html/template + HTMX (decision pending, may use Vue.js)
- **CSS**: Tailwind CSS or similar
- **Charts**: Chart.js or similar for timeline visualization

### Infrastructure
- **Container Runtime**: Podman (local dev), Kubernetes (production)
- **Deployment**: ArgoCD GitOps
- **Ingress**: Traefik with cert-manager for HTTPS
- **Monitoring**: Prometheus + Grafana
- **Secrets Management**: Kubernetes Secrets + 1Password CLI

### Development Tools
- **Version Control**: Git
- **CI/CD**: GitHub Actions or GitLab CI (TBD)
- **Linting**: golangci-lint
- **Testing**: Go testing package + testify
- **Documentation**: Markdown + GoDoc

## Risks & Mitigations

### Risk 1: Scan Performance at Scale
**Risk**: 1M files with full checksums may take too long to scan daily.
**Mitigation**:
- Use mtime/size as first-pass detection (skip checksum for unchanged files)
- Implement parallel scanning with configurable workers
- Optimize I/O with efficient buffering and batching
- Monitor scan duration and adjust random sample percentage

### Risk 2: Database Growth
**Risk**: 10 years of history for 1M files could result in massive database.
**Mitigation**:
- Store only current checksums + change events (not every checksum for every scan)
- Implement aggressive retention policies with auto-cleanup
- Use PostgreSQL partitioning for large tables
- Monitor database size and optimize schema

### Risk 3: False Positives (Mount Unavailability)
**Risk**: Network mount unavailable during scan triggers false deletion alerts.
**Mitigation**:
- Pre-scan health check: verify mount is accessible before starting
- If mount fails during scan, mark scan as failed (don't record deletions)
- Retry logic with exponential backoff
- Clear UI indication of partial/failed scans

### Risk 4: Silent Corruption Not Detected
**Risk**: Random sampling might miss corrupted files.
**Mitigation**:
- Weighted sampling prioritizes files not recently verified (ensures coverage)
- Configurable sample percentage (increase if corruption detected)
- Manual "force verify" option for critical files
- Monitor verification age distribution in UI

### Risk 5: Resource Contention
**Risk**: Checksumming starves NAS/Ceph or other applications.
**Mitigation**:
- Default to single-threaded (conservative)
- User opts into parallelism after understanding impact
- Configurable CPU/memory limits in Kubernetes
- Nice process priority for checksumming

## Timeline & Phases

### Phase 1: MVP (Weeks 1-4)
- Week 1: Project setup, database schema, basic Go structure
- Week 2: Scanner implementation (local filesystem only)
- Week 3: Web UI (authentication, dashboard, scan logs)
- Week 4: Testing, debugging, initial Kubernetes deployment

### Phase 2: Storage Backends & Features (Weeks 5-8)
- Week 5: NFS and SMB backend support
- Week 6: Advanced UI features (timeline, export)
- Week 7: Large change detection, comprehensive webhooks
- Week 8: Performance optimization, Prometheus metrics

### Phase 3: Production Hardening (Weeks 9-12)
- Week 9: Multi-user support, API design
- Week 10: Advanced analytics, deduplication detection
- Week 11: Automated verification campaigns
- Week 12: Documentation, integration testing, production rollout

## License

**AGPLv3** - Ensures that users running modified versions as a service must share source code (important for web-based tools).

## Next Steps

1. Create ARCHITECTURE.md with detailed technical design
2. Design database schema and migration strategy
3. Set up Go project structure with proper module organization
4. Initialize PostgreSQL database and create credentials
5. Implement core scanner logic with local filesystem support
6. Build basic web UI with authentication
7. Deploy to Kubernetes and begin dogfooding on homelab

---

**Document Status**: Draft v1.0
**Last Updated**: 2025-11-17
**Author**: Jeff (with Claude Code assistance)
