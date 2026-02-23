# Sentinel-Go

A high-performance, distributed rate limiting service built in Go. Sentinel-Go provides a flexible and extensible framework for protecting APIs and services from abuse using various rate limiting algorithms.

> ℹ️ This project is a work in progress.

## Features

- **Multiple Algorithms**: Token Bucket, Leaky Bucket, Fixed Window, Sliding Window Log, Sliding Window Counter
- **Dynamic Switching**: Change rate limiting algorithms on-the-fly via gRPC without restarting
  > **Note**: Switching algorithms will reset the rate limit window for all keys (state is stored differently per algorithm).
- **Distributed Architecture**: Redis Sentinel for high-availability state management across multiple instances
- **HTTP + gRPC**: HTTP for protected endpoints, gRPC for rate limiter management
- **Prometheus Metrics**: Built-in observability for monitoring rate limiter decisions
- **Flexible Identification**: IP-based or API Key-based rate limiting

## Quick Start

### Prerequisites

- Go 1.25+
- Redis Sentinel cluster (for distributed, high-available rate limiting state)
- `protoc` (for gRPC code generation)

### Run with Docker

```bash
docker run -p 8080:8080 -p 50051:50051 \
  -e REDIS_SENTINELS=<address to redis sentinels> \
  -e REDIS_MASTERNAME=<redis master name> \
  sentinel-go
```

### Build from Source

```bash
# Clone the repository
git clone https://github.com/JesterSe7en/Sentinel-Go.git
cd Sentinel-Go

# Copy environment configuration
cp .env.example .env
# Edit .env with your Redis Sentinel settings

# Build
make build

# Run
make run
```

The server starts on:
- **HTTP**: http://localhost:8080
- **gRPC**: localhost:50051

## Usage

Sentinel-Go exposes two protocols:

- **HTTP (port 8080)**: Protected API endpoints go here.  The rate limiter middleware checks each request.
- **gRPC (port 50051)**: Control plane for managing the rate limiter (list algorithms, switch algorithms, check status).

### Test Rate Limiting

```bash
# Basic request (IP-based)
curl http://localhost:8080/data

# With API key
curl -H "X-API-Key: mykey" http://localhost:8080/data

# View metrics
curl http://localhost:8080/metrics
```

### gRPC API

```bash
# List available algorithms
grpcurl -plaintext localhost:50051 limiter.RateLimiter/ListAlgorithms

# Get current algorithm
grpcurl -plaintext localhost:50051 limiter.RateLimiter/GetCurrentAlgorithm

# Update algorithm
grpcurl -plaintext -d '{"algorithm":"LeakyBucket"}' localhost:50051 limiter.RateLimiter/UpdateAlgorithm
```

## Configuration

| Variable | Description | Default |
|----------|-------------|---------|
| `HTTP_PORT` | HTTP server port | 8080 |
| `GRPC_PORT` | gRPC server port | 50051 |
| `REDIS_SENTINELS` | Redis Sentinel addresses (comma-separated) | - |
| `REDIS_MASTERNAME` | Redis Sentinel master name | - |
| `REDIS_PASSWORD` | Redis password | - |
| `RATE_LIMIT_ALGORITHM` | Initial rate limiting algorithm | TokenBucket |

## Project Structure

```
Sentinel-Go/
├── api/v1                  # Protocol Buffer definitions
│   ├── pb/                 # Generated protobuf code
├── cmd/server/             # Application entry point
├── internal/
│   ├── algorithm/          # Algorithm type definitions
│   ├── app/                # Application initialization
│   ├── config/             # Configuration management
│   ├── limiter/            # Core rate limiting engine
│   ├── logger/             # Structured logging
│   └── storage/            # Redis storage layer
├── .env.example            # Environment configuration template
└── Makefile                # Build automation
```

## Testing

```bash
go test ./...
```

## Roadmap

### Completed

- [x] Multiple rate limiting algorithms
- [x] Dynamic algorithm switching via gRPC
- [x] Redis Sentinel support for distributed deployment
- [x] Prometheus metrics
- [x] Custom response headers (X-RateLimit-Limit, X-RateLimit-Remaining, X-RateLimit-Reset)
- [x] Configurable fail-open/fail-closed behavior on Redis timeout
- [x] Health check endpoints (/health, /ready)

### In Progress

- [ ] Unit tests coverage improvements

### Future Work

- [ ] Rate limiting by endpoint/path
- [ ] Configurable rate limits via config file (YAML/JSON)
- [ ] Graceful algorithm switching with warm-up period (run algorithms in parallel during transition)
- [ ] Authentication/authorization for gRPC control plane

## License

MIT License - see [LICENSE](LICENSE) for details.
