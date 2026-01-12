# Game Arena Data Cleanup

This directory contains scripts to clean up game arena data before deploying the consolidated migration.

## Overview

After consolidating migrations 009-012 into a single `009_game_arena.sql` file, the production database needs to be cleaned before deployment. This is because the migration system tracks which migrations have been applied, and will skip the new consolidated migration if it sees the old 009-012 already applied.

## Files

- `cleanup_game_arena_data.sh` - Interactive shell script for running cleanup
- `cleanup_game_arena_data.sql` - Raw SQL cleanup commands

## Prerequisites

Before running cleanup, ensure:
1. You have `make pg-tunnel` running in another terminal
   - This creates a secure tunnel to the production database on localhost:5433
2. You have `nc` (netcat) installed for connection checking
3. You have `psql` installed locally

## Option 1: Interactive Shell Script (Recommended)

This is the safest option as it confirms your intentions before proceeding.

```bash
# Run from repository root
./infrastructure/scripts/cleanup_game_arena_data.sh
```

The script will:
1. ✓ Check if pg-tunnel is running
2. ✓ Display what will be deleted
3. ✓ Ask for confirmation
4. ✓ Run the cleanup SQL
5. ✓ Verify the cleanup was successful

## Option 2: Direct SQL

If you prefer to run SQL directly:

```bash
# Ensure pg-tunnel is running first!
psql "postgresql://postgres@localhost:5433/beef?sslmode=disable" < infrastructure/scripts/cleanup_game_arena_data.sql
```

## What Gets Deleted

The cleanup script removes:

### Tables (in dependency order)
- `game_tournament_participants`
- `game_ranked_tournaments`
- `game_match_rounds`
- `game_match_participants`
- `game_leaderboard`
- `game_matches`

### Enums
- `game_tournament_status`
- `game_participant_status`
- `game_match_status`
- `game_match_format`
- `game_match_type`

### Functions
- `update_game_leaderboard()`
- `get_tournaments_needing_announcement()`
- `get_tournaments_needing_close()`
- `get_or_create_tournament()`

### Columns
- `chats.ranked_tournaments_enabled`

### Migration Records
- Removes migration tracking for versions 009, 010, 011, 012

## Production Deployment Flow

1. **Ensure pg-tunnel is running**
   ```bash
   make pg-tunnel
   # This runs in a separate terminal
   ```

2. **Run cleanup** (in another terminal)
   ```bash
   ./infrastructure/scripts/cleanup_game_arena_data.sh
   ```

3. **Deploy** (after cleanup completes)
   ```bash
   make deploy
   ```

4. **Verify** (optional, check logs)
   ```bash
   make logs-api COMPOSE_FILE=infrastructure/docker-compose.prod.yml
   # Look for: "Applied migration 009_game_arena.sql"
   ```

## Rollback

If something goes wrong after cleanup but before successful deployment:

1. Stop the current deployment
2. Check production database backups
3. Restore from the most recent backup
4. Re-run the deployment

## Important Notes

⚠️ **This cleanup script is destructive and permanent**
- All game arena data will be deleted
- Cannot be undone without restoring from backup
- Always ensure you have current backups
- Confirm before running

✓ **Safety features**
- Interactive confirmation required
- Connection verification before running
- Transaction-based (all or nothing)
- Verification queries show what was deleted

## Troubleshooting

### "Database tunnel not accessible on localhost:5433"
- Run `make pg-tunnel` in another terminal
- Ensure it's still running before executing cleanup

### "psql: FATAL: database 'beef' does not exist"
- The database might not exist or connection string is wrong
- Check that pg-tunnel is properly forwarding to production

### "Permission denied"
- Ensure you have write permissions to the production database
- Check that your user has sufficient PostgreSQL privileges

## See Also

- [Migration Consolidation Plan](../../.claude/plans/swirling-kindling-riddle.md)
- [CLAUDE.md](../CLAUDE.md) - Project documentation
