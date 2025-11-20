#!/bin/bash
set -e

echo "Fixity Podman Deployment Test Script"
echo "====================================="
echo ""

# Check if podman-compose is available
if ! command -v podman-compose &> /dev/null; then
    echo "❌ Error: podman-compose is not installed"
    echo "Install with: pip3 install podman-compose"
    exit 1
fi

echo "✓ podman-compose is available"
echo ""

# Build and start services
echo "Step 1: Building and starting services..."
podman-compose up -d --build

echo ""
echo "Step 2: Waiting for services to be healthy..."
sleep 10

# Check if containers are running
if ! podman ps | grep -q fixity-app; then
    echo "❌ Error: Fixity container is not running"
    echo "Check logs with: podman-compose logs fixity"
    exit 1
fi

if ! podman ps | grep -q fixity-postgres; then
    echo "❌ Error: PostgreSQL container is not running"
    echo "Check logs with: podman-compose logs postgres"
    exit 1
fi

echo "✓ Both containers are running"
echo ""

# Wait for application to be ready
echo "Step 3: Waiting for Fixity to be ready..."
MAX_RETRIES=30
RETRY=0
while [ $RETRY -lt $MAX_RETRIES ]; do
    if curl -s http://localhost:8080/health > /dev/null 2>&1; then
        echo "✓ Fixity is responding"
        break
    fi
    RETRY=$((RETRY + 1))
    echo "Waiting... ($RETRY/$MAX_RETRIES)"
    sleep 2
done

if [ $RETRY -eq $MAX_RETRIES ]; then
    echo "❌ Error: Fixity did not become ready in time"
    echo "Check logs with: podman-compose logs fixity"
    exit 1
fi

echo ""
echo "Step 4: Creating admin user..."
podman exec fixity-app /app/fixity user create \
    --username admin \
    --password admin123 \
    --email admin@example.com \
    --admin

echo ""
echo "✅ Deployment test complete!"
echo ""
echo "Next steps:"
echo "1. Open your browser to: http://localhost:8080"
echo "2. Login with:"
echo "   Username: admin"
echo "   Password: admin123"
echo "3. Add a storage target:"
echo "   - Name: Test Data"
echo "   - Type: Local Filesystem"
echo "   - Path: /data"
echo "4. Trigger a scan and watch it work!"
echo ""
echo "Useful commands:"
echo "  View logs:        podman-compose logs -f fixity"
echo "  Stop services:    podman-compose down"
echo "  Restart services: podman-compose restart"
echo "  Access DB:        podman exec -it fixity-postgres psql -U fixity"
echo ""
