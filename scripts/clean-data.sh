#!/bin/bash
# NOFX Data Cleanup Script

set -e

echo "=========================================="
echo "NOFX Data Cleanup Tool"
echo "=========================================="
echo ""

# Show options
echo "Please select cleanup level:"
echo "1. Complete reset (delete all data including users and configs)"
echo "2. Clear trading data (preserve users and AI/exchange configs)"
echo "3. Clear decision logs only"
echo "4. Backup database"
echo "5. Cancel"
echo ""
read -p "Enter option (1-5): " choice

case $choice in
  1)
    echo ""
    echo "⚠️  Warning: This will delete all data including:"
    echo "  - User accounts"
    echo "  - AI model configurations"
    echo "  - Exchange configurations"
    echo "  - Trader configurations"
    echo "  - All trading records"
    echo ""
    read -p "Are you sure you want to continue? (yes/no): " confirm
    if [ "$confirm" = "yes" ]; then
      echo "Stopping containers..."
      docker-compose down

      echo "Backing up database..."
      if [ -f config.db ]; then
        cp config.db "config.db.backup.$(date +%Y%m%d_%H%M%S)"
        echo "✓ Backed up to config.db.backup.$(date +%Y%m%d_%H%M%S)"
      fi

      echo "Deleting database..."
      rm -f config.db

      echo "Clearing decision logs..."
      rm -rf decision_logs/*

      echo ""
      echo "✅ Complete reset finished!"
      echo "Run 'docker-compose up -d' to restart"
    else
      echo "Cancelled"
    fi
    ;;

  2)
    echo ""
    echo "Will clear the following data:"
    echo "  - Trader configurations"
    echo "  - User signal sources"
    echo "  - Decision logs"
    echo ""
    echo "Will preserve the following data:"
    echo "  - User accounts"
    echo "  - AI model configurations"
    echo "  - Exchange configurations"
    echo ""
    read -p "Are you sure you want to continue? (yes/no): " confirm
    if [ "$confirm" = "yes" ]; then
      echo "Clearing trader data..."
      sqlite3 config.db "DELETE FROM traders;"
      sqlite3 config.db "DELETE FROM user_signal_sources;"

      echo "Clearing decision logs..."
      rm -rf decision_logs/*

      echo ""
      echo "✅ Trading data cleared!"
      echo "Recommend restarting containers: docker-compose restart"
    else
      echo "Cancelled"
    fi
    ;;

  3)
    echo ""
    echo "Will clear all decision log files"
    read -p "Are you sure you want to continue? (yes/no): " confirm
    if [ "$confirm" = "yes" ]; then
      rm -rf decision_logs/*
      echo "✅ Decision logs cleared!"
    else
      echo "Cancelled"
    fi
    ;;

  4)
    backup_file="config.db.backup.$(date +%Y%m%d_%H%M%S)"
    cp config.db "$backup_file"
    echo "✅ Database backed up to: $backup_file"

    # Show database size
    size=$(ls -lh config.db | awk '{print $5}')
    echo "Database size: $size"
    ;;

  5)
    echo "Cancelled"
    ;;

  *)
    echo "Invalid option"
    ;;
esac
