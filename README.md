<h1 align="center">metar.live</h1>
<p align="center">🛫☀️🌦</p>
<p align="center">Latest worldwide aviation weather and airport data, delivered via GraphQL</p>

## What is this?

metar.live (previously metar.gg) is a server written in Go that provides a GraphQL API, for querying the latest METARs, TAFs, airports and runways.
It imports airport data from [OurAirports.com](https://github.com/davidmegginson/ourairports-data) and METAR and TAF data from [NOAA](https://www.aviationweather.gov).

You can either host it yourself or use the hosted version of this API available at [rapidapi.com](https://rapidapi.com/benjasper/api/aviation-weather-and-airport-data).
The hosted version imports airport data every 7 days and weather data every 5 minutes, and retains the weather data for 7 days.

### Features ✨
- Imports Airports, METARs and TAFs
  - Hashing, for duplicate detection
  - Syncs all Airport Data and removes old ones
  - Timezone data for airports
  - Generate a sitemap for all airports with a weather station
- GraphQL API
  - Unit Conversion API
  - Airport vicinity search
  - Next METAR Time Prediction
  - Search API
  - Pagination

## Tech 🛠

- Go
- GraphQL
- MySQL Database
- [ent](https://entgo.io/docs/getting-started): Entity framework for Go
- [gqlgen](https://gqlgen.com): GraphQL server library for Go
- [gin](https://github.com/gin-gonic/gin): Web framework for Go
- Optional: [Axiom](https://www.axiom.co/) for logging data ingestion

## How to run ▶️
The easiest way to run the server is by using the [Docker image](https://github.com/benjasper/metar.live-backend/pkgs/container/metar.live).
You only need to provide it with a MySQL database and off you go! 🛫

See the [sample configuration](./docs/sample-configuration.md) for an example of how to get the app running via containers!

### Environment
See [`.env.example`](.env.example) to see what environment variables are required.

### Compiling
You can also easily compile the server yourself and run the binary.
See the [`Dockerfile`](Dockerfile) for an example of how to compile the server.

### How to access the GraphQL API
The GraphQL API is available at `/graphql`. You can use introspection to see what queries are available.

### How to import data
You can either use the builtin CRON_* environment Variables (see [`.env.example`](.env.example)) or use the HTTP API to trigger the imports.
With the environment variables you are not dependent on another system, because the app will trigger the imports itself.

The server also has a few triggers to start the data import from the sources mentioned above.
Those can be triggered via HTTP POST requests to:

- POST `/import/airports`: Imports airport data
- POST `/import/weather`: Imports weather data
- POST `/clean`: Gets rid of old weather data see [`.env.example`](.env.example) for the `WEATHER_DATA_RETENTION_DAYS` variable
- POST `/sitemap`: Generate sitemap asynchronously
- GET `/health`: Health check endpoint

❗️Send the requests including the secret (from the `ADMIN_SECRET` env variable) in the `Authorization` header.

In every case, the server will respond with a `204 No content` if the import was successfully triggered. You can use some kind of cron service to trigger these endpoints at your desired intervals.
The airport data is synced while the weather data will only be added and not automatically removed. You can use the `/clean` endpoint to remove old weather data.

## Development 💻

### Prerequisites

- [Go](https://golang.org/doc/install)
- [Docker Compose](https://docs.docker.com/compose/install/)

### Setup

1. Clone the repository
2. Run `go mod download` to download the dependencies
3. Set up the environment variables in a .env file, see [`.env.example`](.env.example)
4. Run `docker-compose up -d` to start the database
5. Run `go run main.go` to start the server

### Development
Execute code generation with
`go generate ./...`

### Pull Requests
PRs are welcome! Please make sure to run `go generate ./...` before committing.

## License 📝
[MIT](LICENSE)
