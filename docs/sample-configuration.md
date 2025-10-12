# Sample configuration

We'll use a combination of a `.env` file and a `docker-compose.yml` file to get the app running!

## Create a .env file

There is a sample `.env.example` located in the root of this repository. Check that out to find all available configuration settings.

Create a `.env` file with your secrets and configuration values, like this:
```
ADMIN_SECRET=SuperSecretAdminPassword
ALLOWED_CORS_ORIGINS=
GIN_MODE=release
MAX_CONCURRENT_IMPORTS=4
PORT=80
WEATHER_DATA_RETENTION_DAYS=1

MARIADB_DATABASE=metarlive
MARIADB_PORT=3306
MARIADB_ROOT_PASSWORD=SuperSecretDBPassword!

# GraphQL Query Complexity Limit
GRAPHQL_QUERY_COMPLEXITY_LIMIT=200

# Let the Go App schedule Imports
CRON_WEATHER_IMPORT="*/2 * * * *"
CRON_AIRPORTS_IMPORT="0 2 * * SUN"
CRON_DATA_CLEAN="0 3 * * *"
CRON_SITEMAP_GENERATION="0 22 * * *"
```

## Create a docker-compose.yml file

Let's create a `docker-compose.yml` file. We'll use `${ENV_VALUE}` notation so the values from the `.env` file get populated in our Docker Compose file. Make it look like this:
```
services:
  metar.live-backend:
    container_name: metar.live-backend
    image: ghcr.io/benjasper/metar.live:latest
    depends_on:
      - db
    environment:
      # A secret to protect the maintenance REST endpoints
      - ADMIN_SECRET=${ADMIN_SECRET}
      - ALLOWED_CORS_ORIGINS=${ALLOWED_CORS_ORIGINS}
      - DATABASE=root:${MARIADB_ROOT_PASSWORD}@tcp(db:${MARIADB_PORT})/${MARIADB_DATABASE}?parseTime=True&tls=false
      - GIN_MODE=${GIN_MODE}
      - MAX_CONCURRENT_IMPORTS=${MAX_CONCURRENT_IMPORTS}
      - PORT=${PORT}
      - WEATHER_DATA_RETENTION_DAYS=${WEATHER_DATA_RETENTION_DAYS}
      # GraphQL Query Complexity Limit
      - GRAPHQL_QUERY_COMPLEXITY_LIMIT=${GRAPHQL_QUERY_COMPLEXITY_LIMIT}
      # Let the Go App schedule Imports
      - CRON_WEATHER_IMPORT=${CRON_WEATHER_IMPORT}
      - CRON_AIRPORTS_IMPORT=${CRON_AIRPORTS_IMPORT}
      - CRON_DATA_CLEAN=${CRON_DATA_CLEAN}
      - CRON_SITEMAP_GENERATION=${CRON_SITEMAP_GENERATION}
    ports:
      - 80:80
    restart: unless-stopped
  db:
    container_name: mariadb
    image: mariadb:latest
    command: --ssl=0
    ports:
      - ${MARIADB_PORT}:${MARIADB_PORT}
    volumes:
      - db_data:/var/lib/mysql
    environment:
      - MARIADB_DATABASE=${MARIADB_DATABASE}
      - MARIADB_ROOT_PASSWORD=${MARIADB_ROOT_PASSWORD}
    restart: unless-stopped

volumes:
  db_data:
```

## A few notes about this configuration

- Since this is a Docker Compose file, we're assuming you're running this locally or on a server. The Docker Compose file creates a MariaDB (a fork of MySQL) and runs it without SSL. The `DATABASE` environment variable in the Docker Compose file appends `&tls=false` so that the application knows not to use SSL either.
- The Docker Compose file also creates a volume to store the database data. This is using a Docker volume called `db_data`. This allows for persistence across reboots of the app and your server/computer.
- The application is running on port 80. Feel free to adjust the `80` values to be whatever port you like. For instance, perhaps you want to run nginx in front of the app. You could pick port 8080 thereby allowing nginx to run on port 80 to renew a Let's Encrypt certificate. Be sure to expose the port in the firewall if you're wanting to use this publicly!
