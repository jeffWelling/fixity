# Fixity Testing Guide

This guide covers testing Fixity using Podman for local development and testing.

## Prerequisites

- **Podman** installed (replaces Docker)
- **podman-compose** installed (`pip3 install podman-compose`)
- **curl** for health checks (usually pre-installed)

## Quick Start Testing

### 1. Automated Test Script

The fastest way to test Fixity:

```bash
./test-deployment.sh
```

This script will:
1. Build the Fixity container image
2. Start PostgreSQL and Fixity services
3. Wait for services to be healthy
4. Create an admin user
5. Display next steps

Expected output:
```
Fixity Podman Deployment Test Script
=====================================

✓ podman-compose is available

Step 1: Building and starting services...
[+] Building Fixity container...
[+] Starting services...

Step 2: Waiting for services to be healthy...
✓ Both containers are running

Step 3: Waiting for Fixity to be ready...
✓ Fixity is responding

Step 4: Creating admin user...
✓ User created successfully
  ID: 1
  Username: admin
  Email: admin@example.com
  Admin: true

✅ Deployment test complete!
```

### 2. Access the Web UI

Open your browser to: **http://localhost:8080**

Login credentials:
- **Username**: `admin`
- **Password**: `admin123`

## Manual Testing

### Start Services Manually

```bash
# Build and start
podman-compose up -d --build

# View logs
podman-compose logs -f

# View logs for specific service
podman-compose logs -f fixity
podman-compose logs -f postgres
```

### Create Admin User

```bash
podman exec fixity-app /app/fixity user create \
  --username admin \
  --password admin123 \
  --admin
```

### Stop Services

```bash
# Stop and remove containers
podman-compose down

# Stop, remove containers, and delete volumes (clean slate)
podman-compose down -v
```

## Testing Workflows

### Workflow 1: Basic Scan

1. **Login** to http://localhost:8080 with admin credentials
2. **Add Storage Target**:
   - Navigate to "Storage Targets" → "Add New Target"
   - Name: `Test Data`
   - Type: `Local Filesystem`
   - Path: `/data`
   - Enabled: ✓
   - Click "Create Target"
3. **Trigger Scan**:
   - Click "Trigger Scan" on the Test Data target
   - Navigate to "Dashboard" to watch progress
4. **View Results**:
   - Go to "Scans" to see completed scan
   - Click on scan to view details
   - Go to "Files" to browse discovered files

### Workflow 2: Change Detection

1. **Complete Workflow 1** first
2. **Modify a test file**:
   ```bash
   echo "Modified content" >> test-data/documents/readme.txt
   ```
3. **Run another scan**:
   - Go to "Storage Targets"
   - Click "Trigger Scan" on Test Data
4. **View change events**:
   - Go to "Scans" and view the new scan
   - Should show "Files Modified: 1"
   - Click on the modified file in "Files"
   - View "File History" to see change event with old/new checksums

### Workflow 3: File Deletion Detection

1. **Delete a test file**:
   ```bash
   rm test-data/documents/notes.md
   ```
2. **Run another scan**
3. **View results**:
   - Should show "Files Deleted: 1"
   - File will be marked as deleted in the database

### Workflow 4: User Management (Admin Only)

1. **Login as admin**
2. **Create regular user**:
   - Go to "Users" → "Add New User"
   - Username: `testuser`
   - Password: `testpass123`
   - Admin: ✗
   - Click "Create User"
3. **Logout and login as testuser**:
   - Verify you cannot access "Users" page (403 Forbidden)
   - Verify you can view targets, scans, and files

## Testing with Different Data

### Add More Test Files

```bash
# Add documents
echo "Important document" > test-data/documents/important.txt

# Add images (simulated)
dd if=/dev/urandom of=test-data/images/photo.jpg bs=1024 count=100

# Add configs
cat > test-data/configs/database.yml << 'EOF'
database:
  host: localhost
  port: 5432
  name: myapp
EOF

# Trigger new scan to detect new files
```

### Test Large File Sets

```bash
# Create 100 test files
for i in {1..100}; do
  echo "Test file $i" > test-data/documents/file_$i.txt
done

# Trigger scan and verify performance
```

## Database Access

### Connect to PostgreSQL

```bash
podman exec -it fixity-postgres psql -U fixity
```

Useful queries:
```sql
-- View all users
SELECT id, username, email, is_admin, created_at FROM users;

-- View storage targets
SELECT id, name, type, path, enabled FROM storage_targets;

-- View recent scans
SELECT id, storage_target_id, status, files_scanned, started_at
FROM scans
ORDER BY started_at DESC
LIMIT 10;

-- View files
SELECT id, path, size_bytes, checksum, last_seen_at
FROM files
LIMIT 10;

-- View change events
SELECT id, file_id, scan_id, event_type, detected_at
FROM change_events
ORDER BY detected_at DESC
LIMIT 10;
```

## Debugging

### Check Container Status

```bash
# List running containers
podman ps

# Inspect Fixity container
podman inspect fixity-app

# Check resource usage
podman stats
```

### View Application Logs

```bash
# Follow logs in real-time
podman-compose logs -f fixity

# View last 100 lines
podman logs --tail 100 fixity-app

# View logs with timestamps
podman logs -t fixity-app
```

### Check Health Endpoints

```bash
# Application health
curl http://localhost:8080/health

# Expected response:
# {"status":"ok"}

# PostgreSQL health
podman exec fixity-postgres pg_isready -U fixity

# Expected output:
# /var/run/postgresql:5432 - accepting connections
```

### Common Issues

#### "Connection refused" when accessing web UI

**Cause**: Services not started or not healthy yet.

**Solution**:
```bash
# Check if containers are running
podman ps

# Check logs for errors
podman-compose logs

# Restart services
podman-compose restart
```

#### "No users found in database" warning

**Cause**: No admin user created yet.

**Solution**:
```bash
podman exec fixity-app /app/fixity user create --username admin --password admin123 --admin
```

#### Migration errors

**Cause**: Database in dirty state from previous failed migration.

**Solution**:
```bash
# Stop services
podman-compose down -v

# Start fresh
podman-compose up -d --build
```

#### Cannot access /data in container

**Cause**: Volume mount issue or permissions.

**Solution**:
```bash
# Check test-data directory exists and has files
ls -la test-data/

# Verify volume mount in container
podman exec fixity-app ls -la /data

# Fix permissions if needed
chmod -R 755 test-data/
```

## Performance Testing

### Test Scan Performance

```bash
# Create 1000 test files
mkdir -p test-data/perf-test
for i in {1..1000}; do
  dd if=/dev/urandom of=test-data/perf-test/file_$i.bin bs=1024 count=10
done

# Trigger scan via UI and measure time
# Check logs for performance metrics:
podman-compose logs -f fixity | grep -i "scan complete"
```

### Monitor Resource Usage

```bash
# Real-time resource monitoring
podman stats fixity-app fixity-postgres

# Sample output:
# CONTAINER       CPU %    MEM USAGE / LIMIT    MEM %    NET I/O
# fixity-app      2.5%     45MB / 2GB          2.25%    1.2kB / 850B
# fixity-postgres 1.0%     120MB / 2GB         6.0%     2.1kB / 1.5kB
```

## Cleanup

### Remove All Test Data

```bash
# Stop and remove everything
podman-compose down -v

# Remove test files
rm -rf test-data/*

# Recreate test structure
mkdir -p test-data/documents test-data/images test-data/configs
```

### Remove Container Images

```bash
# List images
podman images | grep fixity

# Remove Fixity image
podman rmi fixity-fixity

# Remove all unused images
podman image prune -a
```

## Continuous Testing

### Automated Test Script

Create a comprehensive test script:

```bash
#!/bin/bash
# test-all.sh

echo "Starting comprehensive Fixity tests..."

# Start services
podman-compose up -d --build
sleep 10

# Create admin user
podman exec fixity-app /app/fixity user create --username admin --password admin --admin

# Test health endpoint
curl -f http://localhost:8080/health || exit 1

# Test login (requires API or browser automation)
# TODO: Add automated UI testing with Selenium/Playwright

# Cleanup
podman-compose down -v

echo "All tests passed!"
```

## Next Steps: E2E Testing with Selenium

For comprehensive E2E testing with browser automation, consider:

1. **Selenium Setup**:
   ```yaml
   # Add to docker-compose.yml
   selenium:
     image: selenium/standalone-chrome:latest
     ports:
       - "4444:4444"
     networks:
       - fixity-network
   ```

2. **Python E2E Tests**:
   ```python
   from selenium import webdriver
   from selenium.webdriver.common.by import By

   driver = webdriver.Remote(
       command_executor='http://localhost:4444',
       options=webdriver.ChromeOptions()
   )

   # Test login
   driver.get('http://fixity:8080')
   driver.find_element(By.NAME, 'username').send_keys('admin')
   driver.find_element(By.NAME, 'password').send_keys('admin123')
   driver.find_element(By.CSS_SELECTOR, 'button[type=submit]').click()

   # Verify dashboard loaded
   assert 'Dashboard' in driver.title
   ```

## Summary

This testing guide provides:
- ✅ Quick automated setup with `test-deployment.sh`
- ✅ Manual testing workflows
- ✅ Database access and inspection
- ✅ Debugging techniques
- ✅ Performance testing
- ✅ Cleanup procedures

For production deployment, see [DEPLOYMENT.md](docs/DEPLOYMENT.md) (coming soon).
