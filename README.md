# Golang Job Scheduler

A scalable and high-performance job scheduler built with Go. This project implements a dual-service architecture consisting of a **Scheduling Service** and an **Execution Service**. It features a REST API for managing jobs, database-driven scheduling persistence, and Redis-based task polling for distributed job execution.

## Project Status

This project is currently under active development.

## Features

*   **Dual Service Architecture**: Separate services for scheduling and execution of jobs, promoting scalability and resilience.
*   **REST API**: Manage jobs (create, read, update, delete, trigger) via an HTTP API (built with Echo).
*   **Database-driven Scheduling**: Persists job definitions and schedules in a PostgreSQL database.
*   **Redis-based Task Polling**: Uses Redis for inter-service communication and task queueing.
*   **Cron-based Scheduling**: Supports cron expressions for flexible job scheduling.
*   **CLI Interface**: Utilizes Cobra for command-line operations (e.g., service startup, migrations).
*   **Configuration Management**: Leverages Viper for flexible configuration management (e.g., from files, environment variables).
*   **Database Migrations**: Managed using `golang-migrate`.
*   **API Documentation**: Auto-generated Swagger (OpenAPI) documentation.
*   **Logging**: Structured logging with Zap.
*   **Docker Support**: Comes with Docker and Docker Compose configurations for easy setup and deployment.

## Tech Stack

*   **Language**: Go (version 1.23.0 or higher, as per `go.mod`)
*   **Web Framework**: Echo
*   **CLI**: Cobra
*   **Configuration**: Viper
*   **Database**: PostgreSQL (with GORM as ORM)
*   **In-memory Data Store/Queue**: Redis
*   **Migrations**: golang-migrate/migrate
*   **API Documentation**: Swaggo
*   **Logging**: Zap
*   **Containerization**: Docker, Docker Compose

## Project Structure

```
.
├── Makefile             # Make commands for building, running, testing, etc.
├── README.md            # This file
├── bin/                 # Compiled application binaries
├── cmd/                 # Main applications for each service and CLI tools
│   ├── execution-service/ # Job execution service
│   ├── main.go            # (Potentially main CLI entry point if not service specific)
│   ├── migrate/           # Database migration tool
│   └── scheduling-service/ # Job scheduling service (API)
├── configs/             # Configuration files (e.g., config.yaml)
├── deployments/         # Docker Compose files and other deployment assets
├── go.mod               # Go module definition
├── go.sum               # Go module checksums
├── internal/            # Private application and library code
│   ├── scheduler/       # Code specific to the scheduling service (includes API docs)
│   └── executor/        # Code specific to the execution service
├── migrations/          # SQL migration files
├── pkg/                 # Public library code (shared utilities)
└── scripts/             # Utility scripts (if any)
```

## Prerequisites

*   Go (version 1.23.0 or as specified in `go.mod`)
*   Make
*   Docker
*   Docker Compose

## Getting Started

### 1. Clone the Repository

```bash
git clone <repository-url>
cd golang-stock-scryper 
```
*(Replace `<repository-url>` with the actual URL. The project directory might be `golang-stock-scryper` locally, while the Go module is `golang-stock-scryper`.)*

### 2. Configuration

Configuration is managed by Viper and typically loaded from `configs/config.yaml`. You may need to create or customize this file (e.g., from a `config.example.yaml` if provided) or set environment variables. Key configurations include database connection strings, Redis address, and service ports.

### 3. Build the Services

Compile the scheduler and executor services:
```bash
make build
```
This will place the binaries in the `bin/` directory.

### 4. Database Setup & Migrations

Ensure PostgreSQL and Redis instances are running. If using Docker for dependencies (recommended for local setup):
```bash
make docker-up # This will start dependencies and services
```
If you manage dependencies separately, ensure they are accessible. Then, run database migrations:
```bash
make migrate
```

### 5. Running the Application

You can run the services individually or using Docker Compose.

**Option A: Run services directly (after `make build`)**
```bash
# Run the scheduling service
make run-scheduler
# In a new terminal, run the execution service
make run-executor
```

**Option B: Run with Docker Compose** (Recommended for local development)
This will start PostgreSQL, Redis, the scheduling service, and the execution service.
```bash
make docker-up
```
To view logs:
```bash
make docker-logs
```
To stop services:
```bash
make docker-down
```

## Usage

### API Interaction

Once the scheduling service is running (e.g., on port `8080` by default), you can interact with its REST API.
API documentation is available via Swagger:
*   Generate/update docs: `make docs` (output to `internal/scheduler/docs`)
*   Access docs (usually): `http://localhost:<SCHEDULER_PORT>/swagger/index.html` (e.g., `http://localhost:8080/swagger/index.html`)

### CLI Commands

The application uses Cobra for CLI commands. The primary commands are used to start the services:
*   `./bin/scheduling-service serve`
*   `./bin/execution-service serve`
*   The migration tool is run via `go run cmd/migrate/main.go <up|down...>`, wrapped by `make migrate` for `up`.

Further CLI commands for job management might be available or planned.

## API Usage Examples

### Create a Job

You can create a new job by sending a `POST` request to the `/api/v1/jobs` endpoint.

**Example `curl` command:**

```bash
curl -X POST http://localhost:8080/api/v1/jobs \
-H "Content-Type: application/json" \
-d '{
  "name": "Sample HTTP Job",
  "description": "A job that makes an HTTP GET request.",
  "type": "http_request",
  "payload": {
    "url": "https://jsonplaceholder.typicode.com/todos",
    "method": "GET",
    "headers": {
      "Authorization": "Bearer your-token-here"
    }
  },
  "retry_policy": {
    "max_retries": 3,
    "backoff_strategy": "exponential",
    "initial_interval": "5s"
  },
  "timeout": 60,
  "schedules": [
    {
      "cron_expression": "5 * * * * *",
      "is_active": true
    }
  ]
}'
```

```bash
curl -X POST http://localhost:8080/api/v1/jobs \
  -H "Content-Type: application/json" \
  -d '{
    "name": "STOCK NEWS SCRAPER",
    "description": "stock news scraper by using google rss for get updated news & gemini for summary & analyze",
    "type": "stock_news_scraper",
    "payload": {
      "max_news": 5,
      "stock_codes": [
        "ANTM",
        "RAJA",
        "SMBR",
        "BBNI",
        "BBRI",
        "PTBA",
        "ADRO",
        "UNTR",
        "BBCA",
        "SMGR",
        "KLBF",
        "ICBP",
        "ASII",
        "CPIN",
        "INDY"
      ],
      "blacklisted_domains": [
        "padek.jawapos.com"
      ],
      "max_news_age_in_days": 5,
      "max_request_per_minute": 15
  },
    "retry_policy": {
      "backoff_strategy": "string",
      "initial_interval": "string",
      "max_retries": 0
    },
    "schedules": [
      {
        "cron_expression": "@every 30s",
        "is_active": true
      }
    ],
    "timeout": 360
  }'

This example creates a job named "Sample HTTP Job" that is scheduled to run at the beginning of every hour (`0 * * * *`). The job is of type `http_request` and includes a payload with the target URL, method, and headers. It also defines a retry policy and a timeout.


## Makefile Commands

The `Makefile` provides several useful commands:

*   `make all`: Build all services (default).
*   `make build`: Build all services.
*   `make run-scheduler`: Run the scheduling service.
*   `make run-executor`: Run the execution service.
*   `make clean`: Remove build artifacts.
*   `make test`: Run tests (currently a placeholder).
*   `make swag-install`: Install the `swag` CLI tool for Swagger.
*   `make docs`: Generate Swagger API documentation for the scheduling service.
*   `make migrate`: Run database migrations (up).
*   `make docker-up`: Start all services and dependencies with Docker Compose.
*   `make docker-down`: Stop services started with Docker Compose.
*   `make docker-logs`: Follow Docker logs for services run with Docker Compose.
*   `make help`: Show help for make targets.

## Contributing

Contributions are welcome! Please follow these general guidelines:
1.  Fork the repository.
2.  Create a new branch for your feature or bug fix.
3.  Write clear and concise commit messages.
4.  Ensure your code adheres to existing style and conventions.
5.  Write tests for your changes (if applicable).
6.  Open a pull request.

(More specific guidelines can be added as the project matures.)

## License

(To be determined - e.g., MIT, Apache 2.0. Consider adding a `LICENSE` file to the project root.)


