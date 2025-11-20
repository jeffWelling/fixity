# Fixity Quick Start Guide

Get Fixity up and running in 5 minutes!

## Prerequisites

- **Go 1.21+** (for building from source)
- **PostgreSQL 15+** (for database)
- **Storage to monitor** (local directory, NFS mount, or SMB share)

## Option 1: Quick Local Setup (Recommended)

### 1. Set up PostgreSQL

```bash
# Create database and user
createdb fixity
createuser fixity -P  # Will prompt for password
```

Or using SQL:
```sql
CREATE DATABASE fixity;
CREATE USER fixity WITH PASSWORD 'your_password';
GRANT ALL PRIVILEGES ON DATABASE fixity TO fixity;
```

### 2. Build Fixity

```bash
cd /Users/jeff/claude/projects/fixity
go build -o fixity ./cmd/fixity
```

### 3. Configure Environment

```bash
# Set database URL
export DATABASE_URL="postgres://fixity:your_password@localhost/fixity?sslmode=disable"

# Optional: Custom port (default is :8080)
export LISTEN_ADDR=":8080"
```

### 4. Create Admin User

```bash
# Migrations run automatically, but you need to create a user
./fixity user create \
  --username admin \
  --password your_secure_password \
  --email admin@example.com \
  --admin
```

Expected output:
```
✓ User created successfully
  ID: 1
  Username: admin
  Email: admin@example.com
  Admin: true
  Created: 2025-11-18 20:30:00
```

### 5. Start Fixity

```bash
./fixity serve
```

Expected output:
```
Fixity v0.1.0-alpha
====================
Connecting to database...
✓ Database connected
Running database migrations...
Database is up to date (version 1)
✓ Database migrations complete
✓ Services initialized

🚀 Fixity server listening on :8080
Press Ctrl+C to stop
```

### 6. Access Web UI

Open your browser to: **http://localhost:8080**

Login with:
- Username: `admin`
- Password: (the password you set in step 4)

## What Happens on First Run

1. **Auto-Migration**: Database schema is created automatically
2. **Service Initialization**: Auth and coordinator services start
3. **HTTP Server**: Web UI becomes available immediately
4. **Graceful Shutdown**: Ctrl+C stops cleanly

## Next Steps

### Add a Storage Target

#### Local Filesystem

1. Navigate to **Storage Targets** → **Add New Target**
2. Configure your first target:
   - **Name**: "My Documents"
   - **Type**: Local Filesystem
   - **Path**: `/path/to/monitor`
   - **Enabled**: ✓

3. Click **Create Target**

#### NFS (Network File System)

For NFS shares, ensure the share is mounted first:

```bash
# Mount NFS share (example)
sudo mkdir -p /mnt/nfs
sudo mount -t nfs nfs-server.example.com:/exports/data /mnt/nfs
```

Then create the target:
- **Name**: "NFS Storage"
- **Type**: NFS (Network File System)
- **Server Address**: `nfs-server.example.com`
- **Share Path**: `/exports/data`
- **Mount Path**: `/mnt/nfs`
- **Enabled**: ✓

#### SMB/CIFS (Windows Shares)

For SMB shares, mount the share first:

```bash
# Mount SMB share (example)
sudo mkdir -p /mnt/smb
sudo mount -t cifs //smb-server.example.com/ShareName /mnt/smb -o username=user,password=pass
```

Then create the target:
- **Name**: "Windows Share"
- **Type**: SMB/CIFS (Windows Share)
- **Server Address**: `smb-server.example.com`
- **Share Name**: `ShareName`
- **Mount Path**: `/mnt/smb`
- **Enabled**: ✓

**Note**: For NFS/SMB targets, Fixity expects the share to be already mounted at the specified path. In containerized/Kubernetes environments, this is typically handled by volume mounts in your deployment configuration.

### Run Your First Scan

1. Go to **Storage Targets**
2. Click **Trigger Scan** on your target
3. Watch **Dashboard** for progress
4. View results in **Scans** and **Files**

## Configuration Options

### Environment Variables

```bash
# Required
DATABASE_URL="postgres://user:password@host/db?sslmode=disable"

# Optional
LISTEN_ADDR=":8080"                  # HTTP server port
SESSION_COOKIE_NAME="fixity_session" # Cookie name
SESSION_SECRET="random-secret-key"   # Session encryption key
MAX_CONCURRENT_SCANS="5"             # Max parallel scans
```

### Database URL Format

```
postgres://[user]:[password]@[host]:[port]/[database]?sslmode=[mode]
```

Examples:
```bash
# Local development
DATABASE_URL="postgres://fixity:password@localhost/fixity?sslmode=disable"

# Production with SSL
DATABASE_URL="postgres://fixity:password@db.example.com:5432/fixity?sslmode=require"

# Using Unix socket
DATABASE_URL="postgres://fixity@/fixity?host=/var/run/postgresql&sslmode=disable"
```

## CLI Commands

### Server

```bash
# Start server (auto-migrates database)
./fixity serve

# Check version
./fixity version
```

### User Management

```bash
# Create admin user
./fixity user create --username admin --password pass --admin

# Create regular user
./fixity user create --username user --password pass --email user@example.com
```

### Database Migrations

```bash
# Run migrations (usually automatic)
./fixity migrate up

# Rollback last migration
./fixity migrate down

# List migrations
./fixity migrate list
```

## Troubleshooting

### "No users found in database"

Create an admin user:
```bash
./fixity user create --username admin --password your_password --admin
```

### "failed to connect to database"

Check your DATABASE_URL:
```bash
# Test connection
psql "$DATABASE_URL"
```

Verify:
- PostgreSQL is running (`pg_isready`)
- Database exists (`\l` in psql)
- User has permissions (`\du` in psql)

### "port already in use"

Change the port:
```bash
export LISTEN_ADDR=":9090"
./fixity serve
```

### Migration errors

Check database state:
```bash
./fixity migrate list
psql $DATABASE_URL -c "SELECT version, dirty FROM schema_migrations;"
```

If dirty, manual intervention needed - see migrations in `migrations/postgres/`.

## Production Deployment

### Security Checklist

- ✓ Set `SESSION_SECRET` to random value (32+ chars)
- ✓ Use `sslmode=require` for database
- ✓ Use HTTPS (reverse proxy with nginx/Caddy)
- ✓ Strong passwords for all users
- ✓ Regular PostgreSQL backups
- ✓ Firewall rules (restrict database access)

### Systemd Service

```ini
[Unit]
Description=Fixity File Integrity Monitor
After=network.target postgresql.service

[Service]
Type=simple
User=fixity
Environment="DATABASE_URL=postgres://fixity:password@localhost/fixity?sslmode=require"
Environment="SESSION_SECRET=your-random-32-char-secret"
ExecStart=/usr/local/bin/fixity serve
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
```

Enable and start:
```bash
sudo systemctl enable fixity
sudo systemctl start fixity
```

## Docker Deployment

Coming soon! See [DEPLOYMENT.md](docs/DEPLOYMENT.md)

## Kubernetes Deployment

Coming soon! See [DEPLOYMENT.md](docs/DEPLOYMENT.md)

## Getting Help

- **Documentation**: [README.md](README.md)
- **Issues**: https://github.com/yourusername/fixity/issues
- **Architecture**: [ARCHITECTURE.md](ARCHITECTURE.md)

## What's Different from README

The README showed manual migration steps:
```bash
./fixity migrate up  # Manual migration
./fixity serve       # Then start server
```

Now migrations happen **automatically** when you run:
```bash
./fixity serve  # Auto-migrates, then starts
```

This is safer and more convenient for self-hosted deployments!
