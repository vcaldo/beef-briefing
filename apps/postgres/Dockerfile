FROM postgis/postgis:17-3.4

COPY ./migrations /docker-entrypoint-initdb.d/
COPY ./seeds /docker-entrypoint-initdb.d/

EXPOSE 5432
