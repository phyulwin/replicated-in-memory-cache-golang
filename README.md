# Replicated In-Memory Cache (Go)

This project is a replicated in-memory key/value cache written in Go. It supports running on multiple nodes, replicates data between them (eventual consistency), and provides fast local reads across distributed servers.

## Features
- Replicated in-memory cach
- Fast local reads, distributed writes
- HTTP/JSON API for clients and peers
- Thread-safe, concurrent map
- Last-write-wins conflict resolution
- Key TTL and automatic expiration
- Peer health checks
- CLI client
- Docker and docker-compose support
- Unit and integration tests

## Tech Stack
- Go (concurrency, HTTP server, sync primitives)
- net/http, sync, time, context, encoding/json (stdlib)
- Docker, docker-compose

## Getting Started

### Build & Test
```sh
# Run tests from the repo root
go test ./...

# Build the node and the client
go build -o bin/cache-node ./cmd/cache-node
go build -o bin/cachectl   ./cmd/cachectl
```

### Run a 3-Node Cluster (Manually)
Open three terminals and run:
```sh
# Terminal 1
./bin/cache-node -addr=:8081 -peers=http://localhost:8082,http://localhost:8083

# Terminal 2
./bin/cache-node -addr=:8082 -peers=http://localhost:8081,http://localhost:8083

# Terminal 3
./bin/cache-node -addr=:8083 -peers=http://localhost:8081,http://localhost:8082
```

### Use the CLI
```sh
# Set a value with TTL and wait for 2 peers to acknowledge
./bin/cachectl -server http://localhost:8081 set greeting "hello world" -ttl=30s -min=2

# Get the value from a different node
./bin/cachectl -server http://localhost:8083 get greeting
# -> hello world

# Delete everywhere (full replication)
./bin/cachectl -server http://localhost:8082 del greeting -full
```

### Build Docker Images

```sh
# Build server image
docker build -t rc-node --target node .

# Build CLI image
docker build -t rc-cli --target cli .
```

### Run with Docker Compose

```sh
docker-compose up --build
```

---

**Author:** Phyu Lwin | **Last Updated:** Aug 10th, 2025