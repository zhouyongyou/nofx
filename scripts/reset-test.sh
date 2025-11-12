#!/bin/bash
# Complete reset to first-time startup state (manual, safe)

set -e

echo "=========================================="
echo "🧹 NOFX Complete Reset Script"
echo "=========================================="
echo ""
echo "This will delete all data and return to first-time startup state"
echo ""
echo "Will delete:"
echo "  ✗ config.db         - Users, configs, trader data"
echo "  ✗ decision_logs/*   - All trading decision logs"
echo "  ✗ secrets/*         - RSA keys (will be regenerated)"
echo ""
read -p "Are you sure you want to continue? (yes/no): " confirm

if [ "$confirm" != "yes" ]; then
  echo "Cancelled"
  exit 0
fi

echo ""
echo "📦 Step 1/6: Stopping containers..."
docker-compose down

echo ""
echo "💾 Step 2/6: Backing up existing data (if exists)..."
if [ -f config.db ]; then
  backup_file="config.db.backup.$(date +%Y%m%d_%H%M%S)"
  cp config.db "$backup_file"
  echo "✓ Backed up to: $backup_file"
else
  echo "ℹ️  config.db not found, skipping backup"
fi

echo ""
echo "🗑️  Step 3/6: Deleting database..."
rm -f config.db
echo "✓ config.db deleted"

echo ""
echo "🗑️  Step 4/6: Deleting decision logs..."
if [ -d decision_logs ]; then
  rm -rf decision_logs/*
  echo "✓ decision_logs/* cleared"
else
  echo "ℹ️  decision_logs directory does not exist"
fi

echo ""
echo "🗑️  Step 5/6: Deleting RSA keys (optional)..."
if [ -d secrets ]; then
  rm -rf secrets/*
  echo "✓ secrets/* cleared (will be regenerated on startup)"
else
  echo "ℹ️  secrets directory does not exist"
fi

echo ""
echo "🧽 Step 6/6: Cleaning Docker resources..."
docker system prune -f
echo "✓ Docker resources cleaned"

echo ""
echo "=========================================="
echo "✅ Reset complete!"
echo "=========================================="
echo ""
echo "Next steps:"
echo "  1. Start containers: docker-compose up -d"
echo "  2. View logs: docker-compose logs -f nofx"
echo "  3. Wait for initialization (about 30-60 seconds)"
echo "  4. Visit: http://localhost:3000"
echo ""
