<p align="center">
  <img src="logo.png" alt="Grantsy" width="120" />
</p>

<h1 align="center">Grantsy</h1>

<p align="center">Feature entitlements service for SaaS applications</p>

---

Grantsy manages feature access for your SaaS product. It uses [Casbin](https://casbin.org/) RBAC to enforce which features each user can access based on their subscription plan, with [LemonSqueezy](https://www.lemonsqueezy.com/) as the billing source of truth.

Define your plans and features in a YAML config. Grantsy handles the rest: webhook processing from LemonSqueezy, subscription tracking, and a simple API your application calls to check feature access.

## Quick Start

1. Create a `config.yaml` (see [Configuration Reference](#configuration-reference) below)

2. Set environment variables:

```bash
export API_KEY="your-api-key"
export LEMONSQUEEZY_API_KEY="your-lemonsqueezy-api-key"
export LEMONSQUEEZY_WEBHOOK_SECRET="your-webhook-secret"
```

3. Run with Docker Compose:

```bash
docker compose up
```

4. Check feature access:

```bash
curl -H "X-Api-Key: your-api-key" \
  "http://localhost:8080/v1/check?user_id=user123&feature=dashboard"
```

## API Endpoints

All responses are wrapped in a JSON envelope with `data` and `meta` fields. Errors follow [RFC 9457](https://www.rfc-editor.org/rfc/rfc9457) Problem Details.

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/v1/check?user_id={uid}&feature={feature}` | Check if a user has access to a feature |
| `GET` | `/v1/features` | List all available features |
| `GET` | `/v1/features/{feature_id}` | Get a specific feature |
| `GET` | `/v1/plans?expand=features` | List all plans and their pricing variants |
| `GET` | `/v1/plans/{plan_id}?expand=features` | Get a specific plan |
| `GET` | `/v1/users/{user_id}?expand=plan,features,subscription` | Get user state |
| `POST` | `/v1/webhook/lemonsqueezy` | LemonSqueezy webhook endpoint |

All endpoints except the webhook require an `X-Api-Key` header.

## Configuration Reference

Configuration is loaded from a YAML file. Environment variables are expanded using `${VAR}` syntax.

### `env`

| | |
|---|---|
| **Type** | `string` |
| **Values** | `dev`, `prod` |
| **Default** | `prod` |

Application environment.

### `server`

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `host` | `string` | `0.0.0.0` | Listen address |
| `port` | `int` | `8080` | Listen port (1-65535) |

### `database`

| Key | Type | Required | Description |
|-----|------|----------|-------------|
| `driver` | `string` | Yes | `sqlite` or `postgres` |
| `dsn` | `string` | Yes | Database connection string. For SQLite: file path. For PostgreSQL: connection URI |

### `auth`

| Key | Type | Required | Description |
|-----|------|----------|-------------|
| `api_key` | `string` | Yes | API key for authenticating requests via `X-Api-Key` header |

### `entitlements`

| Key | Type | Required | Description |
|-----|------|----------|-------------|
| `default_plan` | `string` | No | Plan assigned to users without a subscription. If unset, users with no subscription have no features |
| `plans` | `list` | Yes | At least one plan definition |
| `features` | `list` | Yes | At least one feature definition |

**Plan definition:**

| Key | Type | Required | Description |
|-----|------|----------|-------------|
| `id` | `string` | Yes | Unique plan identifier |
| `name` | `string` | Yes | Display name |
| `features` | `list[string]` | Yes | Feature IDs included in this plan |

**Feature definition:**

| Key | Type | Required | Description |
|-----|------|----------|-------------|
| `id` | `string` | Yes | Unique feature identifier |
| `name` | `string` | Yes | Display name |
| `description` | `string` | No | Human-readable description |

### `providers.lemonsqueezy`

| Key | Type | Required | Description |
|-----|------|----------|-------------|
| `api_key` | `string` | Yes | LemonSqueezy API key for fetching pricing and variants |
| `products` | `list` | No | Mappings from LemonSqueezy products to plans |
| `webhook.secret` | `string` | No | Secret for verifying incoming LemonSqueezy webhook signatures |

**Product mapping:**

| Key | Type | Required | Description |
|-----|------|----------|-------------|
| `product_id` | `int` | Yes | LemonSqueezy product ID |
| `plan_id` | `string` | Yes | Plan ID to associate with this product |

### `webhooks`

Optional outgoing webhooks to notify external services of subscription changes.

| Key | Type | Required | Description |
|-----|------|----------|-------------|
| `endpoints` | `list` | No | Webhook destinations |

**Endpoint definition:**

| Key | Type | Required | Description |
|-----|------|----------|-------------|
| `url` | `string` | Yes | Destination URL |
| `secret` | `string` | Yes | Signing secret for HMAC verification |

### `sync_period`

| | |
|---|---|
| **Type** | `string` |
| **Default** | `""` (disabled) |

Periodic sync interval for refreshing pricing and variant data from the provider (e.g. `15m`, `1h30m`). Leave empty to disable.

### `log`

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `level` | `string` | `info` | Log level: `debug`, `info`, `warn`, `error` |
| `format` | `string` | `json` | Log format: `json`, `text` |

### `metrics`

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `enable` | `bool` | `false` | Enable Prometheus metrics endpoint |
| `go_metrics` | `bool` | `false` | Include Go runtime metrics |
| `path` | `string` | `/metrics` | Metrics endpoint path |

### Example Configuration

```yaml
env: prod

server:
  host: 0.0.0.0
  port: 8080

database:
  driver: sqlite
  dsn: /var/lib/grantsy/grantsy.db

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
    - id: api
      name: API Access
      description: REST API access
    - id: sso
      name: Single Sign-On
      description: SAML/OIDC integration

auth:
  api_key: "${API_KEY}"

providers:
  lemonsqueezy:
    api_key: "${LEMONSQUEEZY_API_KEY}"
    products:
      - product_id: 12345
        plan_id: pro
    webhook:
      secret: "${LEMONSQUEEZY_WEBHOOK_SECRET}"

log:
  level: info
  format: json

metrics:
  enable: true
```

## Deployment

### Docker

The image is published to GHCR at `ghcr.io/grantsy/grantsy`. It is built from `scratch` (no OS, no shell) for minimal attack surface and runs as an unprivileged user.

```bash
docker run -p 8080:8080 \
  -v ./config.yaml:/etc/grantsy/config.yaml:ro \
  -v grantsy-data:/var/lib/grantsy \
  -e API_KEY=your-api-key \
  -e LEMONSQUEEZY_API_KEY=your-ls-api-key \
  -e LEMONSQUEEZY_WEBHOOK_SECRET=your-secret \
  ghcr.io/grantsy/grantsy
```

### Health Check

The service exposes `GET /healthz` which returns `200 OK` when healthy and `503 Service Unavailable` during shutdown.

### Databases

**SQLite** (default) — set `dsn` to a file path. The volume mount at `/var/lib/grantsy` persists data across container restarts.

**PostgreSQL** — set `driver: postgres` and `dsn` to a PostgreSQL connection string (e.g. `postgres://user:pass@host:5432/grantsy?sslmode=disable`).

Migrations run automatically on startup.

## License

[Elastic License 2.0](LICENSE)
