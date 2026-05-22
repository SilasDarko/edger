# Edger

A lightweight Go API gateway and reliability proxy. Edger routes HTTP requests to multiple mock backend services using YAML-based route configuration, and adds practical reliability patterns on top: API-key auth, rate limiting, retry handling, circuit breaking, health checks, and request metrics.

Built as a learning project to explore backend and platform engineering concepts in Go.

---

## Why I Built This

I wanted a project that went beyond a basic CRUD app and forced me to think about what happens between a client and a backend service. Edger gave me hands-on experience with reverse proxying, middleware chains, reliability patterns, and how to structure a Go service clearly.

---

## Architecture

```
Client
  │
  ▼
┌──────────────────────────────────────────┐
│              Edger Gateway (:8080)        │
│                                          │
│  Request ID → Auth → Rate Limit →        │
│  Circuit Breaker → Reverse Proxy         │
│                    (with retries)        │
│                                          │
│  GET /health          (gateway liveness) │
│  GET /upstreams/health (checks backends) │
│  GET /gateway/metrics  (counters)        │
└──────┬──────────────┬──────────────┬─────┘
       │              │              │
       ▼              ▼              ▼
 user-service   claims-service  payments-service
   (:4001)         (:4002)          (:4003)
```

Each request goes through a fixed middleware chain before being forwarded:

1. **Request ID** — reads `X-Request-ID` from the client or generates one
2. **Auth** — checks `X-API-Key` header when `auth_required: true`
3. **Rate limit** — fixed-window limiter keyed by API key or client IP
4. **Circuit breaker** — rejects requests when an upstream is repeatedly failing
5. **Reverse proxy** — forwards to upstream with per-attempt timeout and retries
6. **Logging + metrics** — writes a structured JSON log line and updates counters

---

## Features

- YAML-based route configuration (path prefix, upstream, timeout, retries, rate limit)
- Reverse proxy using `net/http` and `httputil.ReverseProxy`
- API-key authentication (configurable via environment variable)
- In-memory fixed-window rate limiting per key or IP
- Retry logic for safe HTTP methods (GET, HEAD) on 502/503/504 or connection errors
- Three-state circuit breaker (closed → open → half-open) per upstream
- Structured JSON request logging
- `/health`, `/upstreams/health`, and `/gateway/metrics` endpoints
- Three mock backend services with failure simulation (`?fail=true`)
- Docker Compose setup for running the full stack locally
- GitHub Actions CI (format check, tests, build)

---

## Folder Structure

```
.
├── cmd/
│   ├── gateway/          # Gateway server entry point
│   ├── user-service/     # Mock user backend (port 4001)
│   ├── claims-service/   # Mock claims backend (port 4002)
│   └── payments-service/ # Mock payments backend (port 4003)
├── internal/
│   ├── config/           # YAML config loader and validation
│   ├── circuitbreaker/   # Three-state circuit breaker
│   ├── logging/          # Structured JSON request logger
│   ├── metrics/          # In-memory request counters
│   ├── middleware/        # Auth validation and request ID
│   ├── proxy/            # Reverse proxy handler with retry logic
│   └── ratelimit/        # Fixed-window rate limiter
├── config/
│   ├── routes.yaml        # Local dev config (localhost upstreams)
│   └── routes.docker.yaml # Docker Compose config (service-name upstreams)
├── .github/workflows/ci.yml
├── Dockerfile.gateway
├── Dockerfile.user-service
├── Dockerfile.claims-service
├── Dockerfile.payments-service
├── docker-compose.yml
└── go.mod
```

---

## Running Locally (Without Docker)

**Prerequisites:** Go 1.21+

```bash
# 1. Install dependencies
go mod download

# 2. Start the three mock backend services in separate terminals
go run ./cmd/user-service
go run ./cmd/claims-service
go run ./cmd/payments-service

# 3. Start the gateway (uses config/routes.yaml by default)
go run ./cmd/gateway
```

The gateway listens on `http://localhost:8080`.

**Environment variables (all optional for local dev):**

| Variable | Default | Description |
|---|---|---|
| `EDGER_API_KEY` | `dev-api-key` | Expected API key value |
| `EDGER_CONFIG_PATH` | `config/routes.yaml` | Path to route config file |
| `PORT` | `8080` | Gateway listen port |

---

## Running With Docker Compose

```bash
docker compose up --build
```

This builds and starts all four services. The gateway will use `config/routes.docker.yaml`, which points upstreams at Docker Compose service names instead of localhost.

To stop:
```bash
docker compose down
```

---

## Example curl Commands

### Gateway health

```bash
curl http://localhost:8080/health
# {"status":"ok","service":"edger-gateway"}
```

### Upstream health check

```bash
curl http://localhost:8080/upstreams/health
```

### Metrics

```bash
curl http://localhost:8080/gateway/metrics
```

### Authenticated request

```bash
curl -H "X-API-Key: dev-api-key" http://localhost:8080/users/profile
curl -H "X-API-Key: dev-api-key" http://localhost:8080/claims/status
curl -H "X-API-Key: dev-api-key" http://localhost:8080/payments/history
```

---

## Testing Auth

**Missing key → 401:**
```bash
curl -i http://localhost:8080/users/profile
# HTTP/1.1 401 Unauthorized
```

**Wrong key → 401:**
```bash
curl -i -H "X-API-Key: wrong-key" http://localhost:8080/users/profile
# HTTP/1.1 401 Unauthorized
```

**Correct key → 200:**
```bash
curl -i -H "X-API-Key: dev-api-key" http://localhost:8080/users/profile
# HTTP/1.1 200 OK
```

---

## Testing Rate Limiting

The `/payments` route allows 30 requests per minute. Run this to exceed it:

```bash
for i in $(seq 1 35); do
  curl -s -o /dev/null -w "%{http_code}\n" \
    -H "X-API-Key: dev-api-key" \
    http://localhost:8080/payments/history
done
```

After 30 requests you'll see `429` responses.

---

## Testing Retries

The mock services support `?fail=true` to return a 503. The gateway retries GET requests up to `retries` times before giving up.

Watch the gateway logs while triggering a failure:

```bash
# This will fail on the backend — the gateway retries 2 times for /users
curl -H "X-API-Key: dev-api-key" "http://localhost:8080/users/profile?fail=true"
```

Look for `"retried":true` in the gateway log output.

---

## Testing the Circuit Breaker

The circuit breaker opens after 3 consecutive upstream failures and blocks requests for 10 seconds.

```bash
# Trigger 3+ failures in a row to open the circuit
for i in 1 2 3 4; do
  curl -s -o /dev/null -w "%{http_code}\n" \
    -H "X-API-Key: dev-api-key" \
    "http://localhost:8080/users/profile?fail=true"
done
```

After the 3rd consecutive 503 from the upstream, subsequent requests will return `503` immediately without reaching the backend. The circuit resets after 10 seconds.

---

## Health Checks and Metrics

**Gateway liveness:**
```bash
curl http://localhost:8080/health
```

**Upstream status (checks each /health endpoint):**
```bash
curl http://localhost:8080/upstreams/health
```

**In-memory metrics:**
```bash
curl http://localhost:8080/gateway/metrics
```

Example metrics response:
```json
{
  "total_requests": 42,
  "requests_by_route": {"/users": 20, "/claims": 12, "/payments": 10},
  "upstream_failures": {},
  "rate_limited_requests": 3,
  "retried_requests": 1,
  "circuit_open_events": 0,
  "average_latency_ms": 8.4
}
```

---

## Running Tests

```bash
go test ./...

# With verbose output and race detector
go test ./... -v -race
```

Tests cover:
- Config loading and validation
- Auth middleware
- Rate limiter window behaviour
- Circuit breaker state transitions
- Retry/safe-method decision logic
- Metrics counter correctness

---

## Design Decisions

**Single catch-all handler instead of per-route mux registration**
Using `mux.HandleFunc("/", ...)` with manual prefix matching keeps the middleware chain in one place and makes it easy to add cross-cutting logic later.

**Response buffering for retries**
The retry loop buffers the upstream response in memory before writing to the client. This lets us discard a failed attempt cleanly. For small API responses this is a fine trade-off; it would be a problem for large streaming responses.

**Fixed-window rate limiting**
Simple and easy to explain. The window resets once per minute per key. This means a client could technically send 2x the limit across a window boundary, which is a known trade-off of fixed-window vs. sliding-window. A sliding window or token bucket would be more precise.

**Circuit breaker per upstream URL**
Each upstream gets its own circuit breaker. Only 5xx responses and connection errors count as failures — 4xx responses are the client's fault, not the upstream's.

**Two YAML config files**
`config/routes.yaml` uses localhost addresses for running without Docker. `config/routes.docker.yaml` uses Docker Compose service names. The active config is selected via the `EDGER_CONFIG_PATH` environment variable.

**Mock services instead of real backends**
The three backend services are intentionally minimal. They exist to give the gateway something to route to and to support failure simulation.

---

## Limitations

- Rate limiter state is in-memory and resets on restart. Not suitable for multi-instance deployments.
- Circuit breakers are per-process. Two gateway instances would have independent circuit state.
- No TLS support — this is a local development tool.
- No request body logging (could be a privacy or size concern in real use).
- Metrics reset on process restart.
- The retry loop adds a flat 200ms sleep between attempts — no exponential backoff.

---

## Future Improvements

- Sliding-window or token-bucket rate limiting
- Exponential backoff with jitter for retries
- Timeout per retry attempt (currently the timeout covers all attempts)
- Prometheus metrics endpoint instead of custom JSON
- TLS termination
- Config hot-reload without restart
- Header-based routing in addition to path prefix
- Request/response logging middleware with size limits

---

## Resume Description

**Edger | Go API Gateway and Reliability Proxy | Go, Docker, YAML, GitHub Actions**
- Built a lightweight Go API gateway that routes requests to multiple mock backend services using YAML-based route configuration and reverse proxy middleware.
- Added API-key authentication, in-memory rate limiting, structured request logging, retry handling for failed GET requests, and basic circuit breaker logic.
- Exposed health checks and request-level metrics for latency, error rates, blocked requests, and downstream service failures, with Docker Compose and GitHub Actions CI.
