# PostgreSQL Database Service
# Placeholder Dockerfile for PostgreSQL with custom initialization
FROM postgres:17.1-bookworm

COPY ./migrations /docker-entrypoint-initdb.d/
COPY ./seeds /docker-entrypoint-initdb.d/

EXPOSE 5432
