#!/bin/bash
# Start in test mode (no data persistence)

set -e

echo "=========================================="
echo "🧪 NOFX Test Mode Startup"
echo "=========================================="
echo ""
echo "Test mode features:"
echo "  ✓ Database stored inside container (not mounted locally)"
echo "  ✓ Decision logs use in-memory filesystem"
echo "  ✓ Data automatically cleared after container restart"
echo "  ✓ Fully isolated, does not affect production environment"
echo ""
read -p "Are you sure you want to start test mode? (yes/no): " confirm

if [ "$confirm" != "yes" ]; then
  echo "Cancelled"
  exit 0
fi

echo ""
echo "🛑 Stopping existing containers (if any)..."
docker-compose -f docker-compose.test.yml down 2>/dev/null || true
docker-compose down 2>/dev/null || true

echo ""
echo "🚀 Starting with test configuration..."
docker-compose -f docker-compose.test.yml up -d

echo ""
echo "⏳ Waiting for containers to start..."
sleep 5

echo ""
echo "=========================================="
echo "✅ Test mode started!"
echo "=========================================="
echo ""
echo "Container information:"
docker-compose -f docker-compose.test.yml ps

echo ""
echo "Access URLs:"
echo "  Frontend: http://localhost:3000"
echo "  Backend: http://localhost:8080"
echo ""
echo "View logs:"
echo "  docker-compose -f docker-compose.test.yml logs -f"
echo ""
echo "Stop test mode:"
echo "  docker-compose -f docker-compose.test.yml down"
echo ""
echo "⚠️  Note: Data will be cleared after container restart!"
echo ""
