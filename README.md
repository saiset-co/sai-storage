# SAI Storage Service

A microservice for document storage built with Go using the SAI framework. This service provides a RESTful API for managing documents in collections with comprehensive CRUD operations and pluggable database backends.

## Features

- **Full CRUD Operations**: Create, Read, Update, Delete documents in collections
- **Pluggable Database Support**: Extensible architecture for multiple database backends
- **MongoDB Implementation**: Native MongoDB driver with connection pooling and optimization (currently available)
- **Flexible Filtering**: Support for complex database queries and operations
- **Configurable**: Environment-based configuration with templates
- **Docker Ready**: Containerized deployment with Docker Compose
- **Health Checks**: Built-in health monitoring
- **API Documentation**: Automatic OpenAPI documentation generation
- **Middleware Support**: CORS, logging, recovery, and authentication
- **Validation**: Request validation with comprehensive error handling
- **Database Management**: Built-in tools for database administration

## Quick Start

### Prerequisites

- Go 1.21+ (for local development)
- Docker and Docker Compose (for containerized deployment)
- Database (MongoDB supported out of the box, others can be implemented)

### Local Development

1. **Clone the repository**
   ```bash
   git clone <repository-url>
   cd sai-storage
   ```

2. **Set up environment**
   ```bash
   make setup
   # Edit .env with your configuration
   ```

3. **Install dependencies**
   ```bash
   make deps
   ```

4. **Start Database (MongoDB example using Docker)**
   ```bash
   make up
   ```

5. **Generate configuration**
   ```bash
   make config
   ```

6. **Run the service**
   ```bash
   make run
   ```

The service will start on `http://localhost:8080` (configurable via `SERVER_PORT`)

### Docker Deployment

1. **Start all services with Docker Compose**
   ```bash
   make up
   ```

2. **View logs**
   ```bash
   make logs
   ```

3. **Access MongoDB Express (Web UI)**
   ```bash
   make mongo-express
   # Or navigate to http://localhost:8081
   ```

4. **Stop services**
   ```bash
   make down
   ```

## API Endpoints

### Base URL
```
http://localhost:8080/api/v1/documents
```

### Create Documents
```http
POST /api/v1/documents/
Content-Type: application/json

{
  "collection": "users",
  "data": [
    {"name": "John", "age": 30, "email": "john@example.com"},
    {"name": "Jane", "age": 25, "email": "jane@example.com"}
  ]
}
```

### Read Documents
```http
GET /api/v1/documents/
Content-Type: application/json

{
  "collection": "users",
  "filter": {"age": {"$gte": 25}},
  "sort": {"name": 1},
  "limit": 10,
  "skip": 0
}
```

### Update Documents
```http
PUT /api/v1/documents/
Content-Type: application/json

{
  "collection": "users",
  "filter": {"name": "John"},
  "data": {"$set": {"age": 31, "status": "active"}},
  "upsert": false
}
```

**Update operators:**
- `$set`: Set field values
- `$unset`: Remove fields
- `$inc`: Increment numeric values
- `$push`: Add to arrays

**Examples:**
```json
// Set fields
{"data": {"$set": {"age": 31, "email": "john@example.com"}}}

// Remove fields  
{"data": {"$unset": {"tempField": ""}}}

// Increment value
{"data": {"$inc": {"views": 1}}}
```

### Delete Documents
```http
DELETE /api/v1/documents/
Content-Type: application/json

{
  "collection": "users",
  "filter": {"age": {"$lt": 18}}
}
```

## Configuration

The service uses environment variables for configuration. Key settings include:

### Service Configuration
- `SERVICE_NAME`: Service name (default: `sai-storage`)
- `SERVICE_VERSION`: Service version (default: `1.0.0`)

### Server Configuration
- `SERVER_HOST`: Server host (default: `127.0.0.1`)
- `SERVER_PORT`: Server port (default: `8080`)
- `SERVER_READ_TIMEOUT`: Read timeout in seconds (default: `30`)
- `SERVER_WRITE_TIMEOUT`: Write timeout in seconds (default: `30`)
- `SERVER_IDLE_TIMEOUT`: Idle timeout in seconds (default: `120`)

### Database Configuration
- `DATABASE_TYPE`: Database type (default: `mongo`, extensible architecture for other databases)

### MongoDB-specific Configuration (when DATABASE_TYPE=mongo)
- `MONGODB_CONNECTION_STRING`: MongoDB connection string
- `MONGO_DATABASE`: Database name (default: `sai`)
- `MONGODB_TIMEOUT`: Connection timeout (default: `30`)
- `MONGODB_MAX_POOL_SIZE`: Maximum pool size (default: `100`)
- `MONGODB_MIN_POOL_SIZE`: Minimum pool size (default: `5`)
- `MONGODB_SELECT_TIMEOUT`: Server selection timeout (default: `5`)
- `MONGODB_IDLE_TIMEOUT`: Connection idle timeout (default: `30`)
- `MONGODB_SOCKET_TIMEOUT`: Socket timeout (default: `30`)
- `MONGO_ROOT_USERNAME`: MongoDB admin username
- `MONGO_ROOT_PASSWORD`: MongoDB admin password

### Logging Configuration
- `LOG_LEVEL`: Log level (default: `debug`)
- `LOG_OUTPUT`: Log output (default: `stdout`)
- `LOG_FORMAT`: Log format (default: `console`)

## Development

### Available Make Commands

```bash
# Development
make deps          # Download Go dependencies
make setup         # Create .env file from template
make config        # Generate config from template
make config-debug  # Debug config generation
make run           # Run the application locally
make build         # Build the application binary

# Docker
make docker-build  # Build Docker image
make docker-run    # Run Docker container
make up            # Start all services with docker-compose
make down          # Stop all services
make logs          # Show logs from all services
make logs-app      # Show logs from application only
make logs-mongo    # Show logs from MongoDB only
make restart       # Restart all services
make rebuild       # Rebuild and restart all services

# Database Management
make mongo-shell   # Connect to MongoDB shell
make mongo-express # Open MongoDB Express in browser
make mongo-reset   # Reset MongoDB data (WARNING: deletes all data!)

# Code Quality
make fmt           # Format Go code
make vet           # Run go vet
make lint          # Run linter (requires golangci-lint)
make mod-tidy      # Tidy Go modules

# Testing
make test          # Run tests
make test-coverage # Run tests with coverage

# Monitoring
make status        # Show status of all services
make health        # Check health of the application
make version       # Show version information

# Cleanup
make clean         # Clean build artifacts
make clean-docker  # Clean Docker resources
make clean-all     # Clean everything

# Tools
make check-tools   # Check if required tools are available
```

### Project Structure

```
.
├── cmd/                    # Application entry points
│   └── main.go            # Main application
├── internal/              # Internal packages
│   ├── handlers/          # HTTP handlers
│   │   └── handler.go
│   ├── mongo/             # MongoDB implementation
│   │   ├── client.go      # MongoDB client
│   │   └── repository.go  # MongoDB repository
│   └── service/           # Business logic
│       └── service.go
├── types/                 # Type definitions
│   ├── request.go         # Request types
│   ├── response.go        # Response types
│   ├── storage.go         # Storage interfaces
│   └── types.go           # Configuration types
├── scripts/               # Deployment scripts
├── config.template.yml   # Configuration template
├── .env                   # Environment variables
├── Dockerfile             # Docker configuration
├── docker-compose.yml     # Docker Compose configuration
└── Makefile              # Build automation
```

## Health Checks

The service includes built-in health checks:

```bash
curl http://localhost:8080/health
```

## API Documentation

When `DOCS_ENABLED=true`, interactive API documentation is available at:

```
http://localhost:8080/docs
```

## Authentication

The service supports two types of authentication:

### Incoming Request Authentication
Configure authentication for incoming API requests:

```env
USERNAME=your-username
PASSWORD=your-password
```

### Authentication Providers
The service supports the following authentication methods:

```yaml
auth_providers:
  basic:
    params:
      username: "name"
      password: "pass"
  token:
    params:
      token: "your-token"
```

**Supported authentication providers:**
- `basic`: HTTP Basic Authentication
- `token`: Token-based authentication

**Environment variables for authentication:**
```env
AUTH_ENABLED=true
AUTH_PROVIDER=basic
USERNAME=name
PASSWORD=pass
TOKEN=your-token
```

## Database Management

**Note: The following commands are MongoDB-specific. For other database implementations, equivalent commands would be provided.**

### MongoDB Express Web UI

Access the MongoDB administration interface:

```bash
make mongo-express
# Navigate to http://localhost:8081
```

### MongoDB Shell Access

Connect directly to MongoDB:

```bash
make mongo-shell
```

### Database Reset

**Warning: This will delete all data!**

```bash
make mongo-reset
```

## Extending Database Support

The service uses a pluggable architecture for database backends. To add support for a new database:

1. Implement the `StorageRepository` interface in `types/storage.go`
2. Add your implementation in a new package (e.g., `internal/postgres/`)
3. Update the switch statement in `cmd/main.go` to include your database type
4. Add configuration options in `config.template.yml`

**Current implementations:**
- MongoDB (`DATABASE_TYPE=mongo`) - Full implementation available

## Error Handling

All API responses follow a consistent format:

**Success Response:**
```json
{
  "data": ["60f1b2b3c8f4b3b3c8f4b3b3"],
  "created": 1
}
```

**Error Response:**
```json
{
  "error": "validation failed",
  "details": "specific error details"
}
```

## Document Metadata

**Note: Metadata behavior may vary depending on the database implementation.**

For MongoDB implementation, the service automatically adds metadata to documents:

- `internal_id`: Unique UUID for the document
- `cr_time`: Creation timestamp (Unix nanoseconds)
- `ch_time`: Last change timestamp (Unix nanoseconds)

## Monitoring

The service includes:

- **Health Checks**: `/health` endpoint with database connectivity check
- **Metrics**: Built-in metrics collection
- **Logging**: Structured logging with configurable levels
- **Request Tracing**: Request/response logging middleware
- **Database Stats**: Database statistics and monitoring (implementation-specific)

## Performance Features

- **Connection Pooling**: Configurable database connection pool
- **Timeout Management**: Configurable timeouts for all operations
- **Batch Operations**: Support for batch inserts and updates
- **Indexing**: Database indexing support (implementation-specific)
- **Query Optimization**: Efficient database query patterns

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Run tests and linting
5. Submit a pull request

## License

This project is licensed under the MIT License - see the LICENSE file for details.

## Support

For issues and questions, please create an issue in the repository or contact the development team.