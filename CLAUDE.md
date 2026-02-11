# CLAUDE.md

## Project: Grantsy — Entitlements Service

Microservice for managing SaaS feature entitlements. External billing (LemonSqueezy) is the source of truth for payments; this service manages feature access via Casbin RBAC.

---

## Tech Stack

- **Go 1.24+** with native `net/http` routing
- **Casbin** for RBAC policy enforcement
- **SQLite** (default) or **PostgreSQL**
- **valmid** for request validation
- **Prometheus** metrics
- **slog** structured logging

---

## Project Structure

```
grantsy/
├── cmd/grantsy/main.go           # Entry point, wiring, graceful shutdown
├── db/migrations/                # SQL migrations (golang-migrate)
├── internal/
│   ├── entitlements/             # Core feature: Casbin service + API routes
│   │   ├── service.go            # Policy loading, feature checks
│   │   ├── route_check.go        # GET /v1/check
│   │   ├── route_features.go     # GET /v1/features
│   │   ├── route_plans.go        # GET /v1/plans
│   │   └── casbin_model.conf     # RBAC model
│   ├── subscriptions/            # Subscription management
│   │   ├── repo.go               # Database repository
│   │   ├── provider_lemonsqueezy.go # LemonSqueezy SDK client, pricing cache, webhook verification
│   │   ├── route_subscription.go # GET /v1/subscription
│   │   └── route_webhook.go      # POST /v1/webhook/lemonsqueezy
│   ├── auth/                     # API key authentication middleware
│   ├── httptools/                # HTTP utilities
│   │   ├── response.go           # JSON envelope, RFC 9457 errors
│   │   ├── middleware.go         # Middleware type
│   │   ├── wrap.go               # Middleware composition
│   │   ├── mw_hidden.go          # Hide routes from external
│   │   └── mw_skip.go            # Skip middleware for paths
│   └── infra/                    # Infrastructure
│       ├── config/config.go      # YAML + env vars + validation
│       ├── db/                   # Connection, migrations, placeholders
│       ├── logger/               # slog setup + middleware
│       ├── metrics/              # Prometheus metrics + middleware
│       ├── server/server.go      # HTTP server with timeouts
│       ├── tracing/middleware.go # X-Request-ID
│       └── validation/           # valmid setup
├── pkg/gracefulshutdown/         # 3-phase graceful shutdown
├── config.yaml                   # Dev config
├── Dockerfile                    # Multi-stage (golang:alpine → scratch)
├── docker-compose.yml
└── Taskfile.yml
```

---

## API Endpoints

### Response Format

All responses use wrapped JSON:
```json
{
  "data": { ... },
  "meta": {
    "request_id": "uuid",
    "timestamp": "RFC3339",
    "version": "1.0"
  }
}
```

Errors follow RFC 9457 Problem Details.

### Public API

```
GET /v1/check?user_id={uid}&feature={feature}
→ { allowed, feature, user_id, plan, reason }

GET /v1/features?user_id={uid}
→ { user_id, plan, features[] }

GET /v1/subscription?user_id={uid}
→ { user_id, plan, status, features[], has_subscription, trial_ends_at, renews_at, cancelled }

GET /v1/plans?expand=features
→ { plans[]{id, name, features[], variants[]?}, all_features[] }
```

### Webhooks

```
POST /v1/webhook/lemonsqueezy
```
Validates signature via SDK. Handles `subscription_created` and `subscription_updated` events.

### Infrastructure (hidden from external)

```
GET /healthz   → 200 OK (503 during shutdown)
GET /metrics   → Prometheus format
```

---

## Configuration

```yaml
env: dev
server:
  host: 0.0.0.0
  port: 8080

database:
  driver: sqlite    # or postgres
  dsn: grantsy.db

entitlements:
  default_plan: free
  plans:
    - id: free
      name: Free
      features: [dashboard]
    - id: pro
      name: Pro
      features: [dashboard, api, sso]
  features:
    - id: dashboard
      name: Dashboard
      description: Basic dashboard access

auth:
  api_key: ${API_KEY}        # Required

sync_period: ""              # Optional periodic pricing/variant data refresh (e.g. "15m", "1h30m")

providers:
  lemonsqueezy:
    api_key: ${LEMONSQUEEZY_API_KEY}  # Required, for fetching pricing/variants
    products:
      - product_id: 12345
        plan_id: pro
    webhook:
      secret: ${LEMONSQUEEZY_WEBHOOK_SECRET}

# Outgoing webhooks (optional)
webhooks:
  endpoints:
    - url: https://your-app.com/webhooks/grantsy
      secret: ${OUTGOING_WEBHOOK_SECRET}

log:
  level: info
  format: json

metrics:
  enable: true
  go_metrics: false
  path: /metrics
```

---

## Database

Single table for LemonSqueezy subscriptions:

```sql
CREATE TABLE subscriptions_lemonsqueezy (
    id INTEGER PRIMARY KEY,        -- LemonSqueezy subscription ID
    user_id TEXT NOT NULL UNIQUE,
    product_id INT,
    status TEXT,                   -- active, on_trial, paused, past_due, cancelled, expired
    trial_ends_at INTEGER,         -- Unix timestamp
    renews_at INTEGER,
    ends_at INTEGER,
    ...
);
```

Active subscriptions: `status IN ('active', 'on_trial')`

---

## Casbin Integration

**Model:** RBAC with plan groupings

- Policies: `(plan_id, feature_id, "access")` — loaded from config
- Groupings: `(user_id, plan_id)` — loaded from DB, updated on webhooks

**Enforcement:** `enforcer.Enforce(userID, featureID, "access")`

---

## Middleware Stack

Applied in order:
1. Tracing (X-Request-ID)
2. Logger (request/response logging)
3. Recovery (panic handling)
4. Auth (API key validation via `X-Api-Key` header)
5. Metrics (Prometheus)

Infrastructure routes (`/healthz`, `/metrics`) and webhooks skip auth.
Infrastructure routes also skip tracing, logging, and metrics.

---

## Testing

- **testify** for assertions/mocks, **mockery v2** for mock generation
- Mock config in `.mockery.yaml`, mocks generated to `{package}/mocks/`
- Tests in separate `_test` packages (e.g., `subscriptions_test`)
- DB tests in `internal/subscriptions/integration_*_test.go` — test repo against real SQLite and PostgreSQL (via testcontainers). Docker is required.
- Route tests need `_ "github.com/grantsy/grantsy/internal/infra/validation"` import for valmid init

---

## Key Dependencies

```
github.com/casbin/casbin/v2
github.com/iamolegga/valmid
github.com/iamolegga/lemonsqueezy-go
github.com/golang-migrate/migrate/v4
github.com/prometheus/client_golang
modernc.org/sqlite
github.com/jackc/pgx/v5
gopkg.in/yaml.v3
github.com/go-playground/validator/v10
```

---

## Tasks

```bash
task dev              # Run with hot reload (air)
task run              # Run without hot reload
task build            # Build to bin/grantsy
task lint             # Run linter
task generate-mocks   # Generate mocks with mockery
task test-unit        # Unit tests with coverage (requires Docker for DB tests)
task test-coverage    # View coverage report
task docker           # Build Docker image
task docker-run       # Run in Docker
```

---

## Patterns

**Interface-based design:**
- `entitlements.SubscriptionLoader` — repo implements
- `entitlements.PricingProvider` — LemonSqueezyProvider implements
- `subscriptions.SubscriptionObserver` — service implements
- `subscriptions.PlanProvider` — service implements

**Dependency injection:** Services receive deps via constructors

**Repository pattern:** `subscriptions.Repo` abstracts DB access

**Middleware composition:** `httptools.Wrap(handler, mw1, mw2, ...)`

---

## Notes

- API key authentication via `X-Api-Key` header (required)
- Plans/features defined in YAML config (version controlled)
- Pricing/variants fetched from LemonSqueezy API at startup, cached in memory
- `sync_period` config controls periodic pricing/variant data refresh from providers
- All LemonSqueezy SDK usage consolidated in `subscriptions.LemonSqueezyProvider`
- DB stores only subscriptions
- Free tier: users without subscription get `default_plan`
- Graceful shutdown: 3-phase (drain → shutdown → cancel)
- Do not write tests unless asked
