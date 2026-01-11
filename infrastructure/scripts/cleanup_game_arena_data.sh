#!/bin/bash

# =====================================================
# CLEANUP SCRIPT: Game Arena Data Reset
# =====================================================
# This script drops all game arena tables, enums, functions, and migration records
# from the production database via pg-tunnel
#
# Prerequisites:
#   - make pg-tunnel must be running in another terminal
#   - Creates tunnel to production database on localhost:5433
#
# Usage:
#   ./infrastructure/scripts/cleanup_game_arena_data.sh

set -e

echo "=================================================="
echo "Game Arena Data Cleanup Script"
echo "=================================================="
echo ""

# Check if pg-tunnel is running
if ! nc -z localhost 5433 2>/dev/null; then
    echo "❌ ERROR: Database tunnel not accessible on localhost:5433"
    echo ""
    echo "Make sure to run 'make pg-tunnel' in another terminal first!"
    echo "This creates a tunnel to the production database."
    echo ""
    exit 1
fi

echo "✓ Database tunnel detected on localhost:5433"
echo ""

# Confirm with user
echo "⚠️  WARNING: This will DELETE all game arena data from production!"
echo ""
echo "The following will be dropped:"
echo "  - Tables: game_matches, game_match_participants, game_match_rounds, game_leaderboard"
echo "  - Tables: game_ranked_tournaments, game_tournament_participants"
echo "  - Enums: game_match_type, game_match_format, game_match_status, game_participant_status, game_tournament_status"
echo "  - Functions: update_game_leaderboard, get_tournaments_needing_announcement, etc."
echo "  - Column: chats.ranked_tournaments_enabled"
echo "  - Migration records: versions 009, 010, 011, 012"
echo ""

read -p "Are you sure you want to proceed? (type 'yes' to confirm): " confirm

if [ "$confirm" != "yes" ]; then
    echo "Cleanup cancelled."
    exit 0
fi

echo ""
echo "Running cleanup..."
echo ""

# Get the directory where this script is located
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

# Run the SQL cleanup script
psql "postgresql://postgres@localhost:5433/beef_briefing?sslmode=disable" < "$SCRIPT_DIR/cleanup_game_arena_data.sql"

if [ $? -eq 0 ]; then
    echo ""
    echo "=================================================="
    echo "✓ Cleanup successful!"
    echo "=================================================="
    echo ""
    echo "Next steps:"
    echo "  1. Deploy the new code: make deploy"
    echo "  2. Verify migration: make logs-api COMPOSE_FILE=infrastructure/docker-compose.prod.yml"
    echo ""
else
    echo ""
    echo "❌ Cleanup failed! Check the errors above."
    exit 1
fi
