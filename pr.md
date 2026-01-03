## Summary

Remove the `ml-dashboard` app (Streamlit-based analytics dashboard) from the repository. The app was already excluded from production and is no longer needed.

## Changes

- Delete `apps/ml-dashboard/` directory (23 files)
- Remove service and volume from `infrastructure/docker-compose.dev.yml`
- Remove `ML_DASHBOARD_PORT` config from `infrastructure/.env.dev.example`
- Remove variables, targets, and PHONY entries from `Makefile`
- Remove from `scripts/show-summary.sh` (DEV_SERVICES, DEV_IMAGES, loop)
- Remove service entry from `README.md`

## Test plan

- [ ] `make up-build` starts without errors
- [ ] `make help` shows no ml-dashboard targets
- [ ] Dev environment summary table no longer lists ml-dashboard
