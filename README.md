# Fixity

**File Integrity & Lifecycle Monitoring System**

Fixity tracks file changes across multiple storage backends, verifies integrity through periodic checksumming, and provides a comprehensive audit trail of file lifecycle events. Named after the archival science term for checksum-based integrity verification.

## Problem Statement

Files mysteriously disappear from storage systems without clear explanation. When files go missing, there's no record of:
- When the deletion occurred
- What was deleted (filename, size, checksum)
- Whether it was intentional or data loss
- Historical patterns that might identify root causes

Fixity solves this by maintaining a complete historical record of all file changes with periodic integrity verification.

## Features

### Core Capabilities
- 📁 **Multi-Backend Support**: Monitor local filesystems, NFS mounts, and SMB/CIFS shares
- 🔍 **Integrity Verification**: Full-file checksumming with configurable algorithms (MD5, SHA-256, BLAKE3)
- 📊 **Change Tracking**: Record additions, deletions, and modifications with complete metadata
- 🎲 **Smart Sampling**: Weighted random verification of unchanged files to detect silent corruption
- 📈 **Historical Analysis**: 10-year default retention with comprehensive lifecycle tracking
- 🚨 **Anomaly Detection**: Configurable thresholds for large-scale changes
- 🔔 **Webhook Integration**: Event-driven notifications for deletions, modifications, and failures
- 🌐 **Web Interface**: User-friendly dashboard for exploring changes and file history

### Technical Highlights
- ⚡ **Performant**: Handles 1M+ files with intelligent mtime/size-based change detection
- 🔄 **Resilient**: Checkpoint-based scan resumption, graceful error handling
- 🐳 **Cloud-Native**: Kubernetes-ready with health checks and Prometheus metrics
- 🔒 **Secure**: Bcrypt authentication, encrypted credentials, TLS support
- 📦 **Modular**: Clean architecture with storage backend abstraction

## Quick Start

### Prerequisites
- Go 1.21+
- PostgreSQL 15+
- (Optional) Kubernetes cluster for deployment

### Installation

```bash
# Clone repository
git clone https://github.com/yourusername/fixity.git
cd fixity

# Build
go build -o fixity ./cmd/fixity

# Initialize database
./fixity migrate up

# Create admin user
./fixity user create --username admin --password <password> --admin

# Start server
./fixity serve --port 8080
```

### Configuration

Fixity uses database-backed configuration. Bootstrap with environment variables:

```bash
export DATABASE_URL="postgres://fixity:password@localhost:5432/fixity?sslmode=require"
export SESSION_SECRET="your-random-secret-key"
```

After first launch, configure storage targets via Web UI at `http://localhost:8080/config`.

## Architecture

Fixity consists of:
- **HTTP Server**: Web UI and REST API
- **Scan Coordinator**: Schedules and manages scans
- **Scanner Engine**: Walks filesystems and computes checksums
- **Storage Backends**: Pluggable filesystem, NFS, and SMB support
- **Alert Engine**: Webhook dispatcher with retry logic
- **Database Layer**: PostgreSQL for metadata and history

See [ARCHITECTURE.md](ARCHITECTURE.md) for detailed technical design.

## Documentation

- [Project Plan](PROJECT_PLAN.md) - Requirements, goals, and timeline
- [Architecture](ARCHITECTURE.md) - Technical design and database schema
- [API Documentation](docs/API.md) - REST API reference *(coming soon)*
- [Deployment Guide](docs/DEPLOYMENT.md) - Kubernetes setup *(coming soon)*

## Development

### Project Structure

```
fixity/
├── cmd/
│   └── fixity/           # Main application entrypoint
├── internal/
│   ├── server/           # HTTP server and handlers
│   ├── scanner/          # Scan engine and coordinator
│   ├── storage/          # Storage backend implementations
│   ├── database/         # Database layer and repositories
│   ├── webhooks/         # Webhook dispatcher
│   ├── auth/             # Authentication and sessions
│   └── config/           # Configuration management
├── migrations/           # Database migrations
├── web/                  # Web UI (templates or frontend)
├── tests/                # Integration and E2E tests
└── docs/                 # Additional documentation
```

### Running Tests

```bash
# Unit tests
go test ./...

# Integration tests (requires test database)
go test -tags=integration ./...

# E2E tests
go test -tags=e2e ./tests/e2e/...
```

### Building Container Image

```bash
podman build -t fixity:latest .
```

## Deployment

### Kubernetes

```bash
# Apply manifests (via ArgoCD)
kubectl apply -f k8s/

# Or use Helm
helm install fixity ./charts/fixity
```

See [Deployment Guide](docs/DEPLOYMENT.md) for detailed instructions.

## Roadmap

### Phase 1 (MVP) ✅
- [x] Local filesystem support
- [x] PostgreSQL backend
- [x] Basic web UI with authentication
- [x] File lifecycle tracking
- [x] Checksum verification
- [x] Webhook support

### Phase 2 (In Progress)
- [ ] NFS and SMB backend support
- [ ] Advanced UI (timeline, diff view, export)
- [ ] Large change detection
- [ ] Comprehensive webhook events
- [ ] Prometheus metrics
- [ ] Kubernetes deployment

### Phase 3 (Planned)
- [ ] Multi-user RBAC
- [ ] Full REST API
- [ ] Advanced analytics
- [ ] Deduplication detection
- [ ] Automated verification campaigns

## Contributing

Contributions welcome! Please:
1. Fork the repository
2. Create a feature branch
3. Add tests for new functionality
4. Ensure `go test ./...` passes
5. Submit a pull request

## License

AGPLv3 - See [LICENSE](LICENSE) for details.

This ensures users running modified versions as a service must share source code.

## Support

- **Issues**: https://github.com/yourusername/fixity/issues
- **Discussions**: https://github.com/yourusername/fixity/discussions
- **Email**: fixity@yourdomain.com

## Acknowledgments

Built with:
- [Go](https://golang.org/) - Programming language
- [PostgreSQL](https://www.postgresql.org/) - Database
- [Chi](https://github.com/go-chi/chi) - HTTP router *(pending)*
- [golang-migrate](https://github.com/golang-migrate/migrate) - Database migrations

Inspired by archival science practices and the need for better file lifecycle visibility.

---

**Status**: Under Active Development
**Version**: 0.1.0-alpha
**Last Updated**: 2025-11-17
