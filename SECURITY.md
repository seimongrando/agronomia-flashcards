# Security Policy

## Reporting a Vulnerability

If you discover a security vulnerability, **do not open a public issue**.  
Send a detailed report to the maintainers via the contact address in `PRIVACY.md`.  
We aim to respond within 5 business days and to release a fix within 30 days for
critical issues.

---

## Security Controls Implemented

### Authentication & Session

| Control | Implementation |
|---------|---------------|
| Authentication | Google OAuth 2.0 with PKCE state validation |
| Session token | JWT (HS256), stored **exclusively** in `HttpOnly` cookie |
| Cookie flags | `HttpOnly=true`, `SameSite=Lax`, `Secure=true` (production) |
| Token expiry | 24 h by default (configurable via `JWT_EXPIRY`) |
| Logout | Server clears cookie via `Max-Age=-1` |

### CSRF Protection

Every state-mutating request (POST / PUT / PATCH / DELETE) to `/api/*` is
validated at the middleware layer:

1. The `Origin` header is compared against the configured `ALLOWED_ORIGINS`.  
2. If `Origin` is absent, the `Referer` header is parsed and its host is compared.  
3. In **development** mode (`ENVIRONMENT != production`) a missing origin is allowed
   to keep local `curl` / Postman testing functional.  
4. The OAuth redirect paths (`/auth/google/*`) are exempt because they arrive
   as plain browser redirects, not XHR.

The frontend additionally sends `X-Requested-With: XMLHttpRequest` on every
API call (defence in depth).

### HTTP Security Headers

| Header | Value |
|--------|-------|
| `Content-Security-Policy` | `default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' https://lh3.googleusercontent.com data:; connect-src 'self'; object-src 'none'; frame-ancestors 'none'; base-uri 'self'; form-action 'self' https://accounts.google.com` |
| `X-Frame-Options` | `DENY` |
| `X-Content-Type-Options` | `nosniff` |
| `Referrer-Policy` | `strict-origin-when-cross-origin` |
| `Permissions-Policy` | `camera=(), microphone=(), geolocation=()` |
| `X-XSS-Protection` | `0` (disabled — CSP supersedes it; legacy value caused bugs in older browsers) |

> **Note on `unsafe-inline` for styles:** The frontend uses inline `style`
> attributes heavily. Removing this directive would require a full CSS refactor.
> The risk from inline *styles* is significantly lower than from inline *scripts*,
> which are **not** allowed (`script-src 'self'` only).

### Rate Limiting

Per-IP token-bucket limiter, tiered by path prefix:

| Path prefix | Default limit | Burst |
|-------------|--------------|-------|
| `/auth/*`   | 30 req / min (0.5 rps) | 5 |
| `/api/*`    | 120 req / min (2 rps) | 20 |

Configurable via environment variables:
`AUTH_RATE_LIMIT_RPS`, `AUTH_RATE_LIMIT_BURST`, `API_RATE_LIMIT_RPS`,
`API_RATE_LIMIT_BURST`.

### Input Validation

All string fields are sanitised (trimmed, null-byte stripped) and
length-bounded before reaching the database:

| Field | Maximum |
|-------|---------|
| `question` | 500 characters |
| `answer` | 2 000 characters |
| `topic` | 80 characters |
| `source` | 120 characters |
| deck `name` | 80 characters |
| CSV upload | 2 MB, max 2 000 rows |
| JSON body (non-multipart) | 1 MB (configurable via `MAX_BODY_SIZE`) |

Card type is validated against an explicit allowlist:
`{conceito, processo, aplicacao, comparacao}`.

### Role-Based Access Control

Three roles exist: `student`, `professor`, `admin`.  
Role checks are enforced by middleware on every protected route before the
handler executes. A user must retain at least one role at all times.

### Error Responses

- Auth errors (401 / 403) return a generic `Unauthorized` / `Forbidden`
  message — no implementation details, no JWT claims, no role names are leaked.
- Internal errors (500) return a generic message — stack traces are never
  written to HTTP responses.
- All error responses use `Content-Type: application/problem+json` (RFC 7807).

### Audit Logging

The following events are logged to structured JSON stdout (`log/slog`):

| Event | Fields logged |
|-------|--------------|
| Role change | `admin_user_id`, `target_user_id`, `roles_added`, `roles_removed` |
| CSV import | `user_id`, `filename`, `imported_count`, `updated_count`, `invalid_count` |
| HTTP request | `request_id`, `method`, `path`, `status`, `duration_ms`, `user_agent` |

**No PII** (emails, names, IP addresses) appears in application audit logs.
Access logs from infrastructure (reverse proxy / load balancer) are a separate
concern and should be governed by the infrastructure team's retention policy.

### Dependency Supply Chain

- Dependencies are pinned via `go.sum`.
- Run `go mod verify` to detect tampering.
- Dependabot / `govulncheck` should be enabled in CI.

---

## Environment Variables Reference

| Variable | Description | Default |
|----------|-------------|---------|
| `ENVIRONMENT` | `production` enables `Secure` cookie flag and strict CSRF | `development` |
| `ALLOWED_ORIGINS` | Comma-separated list of allowed CORS/CSRF origins | *(empty — same-site check)* |
| `JWT_SECRET` | HS256 signing key (min 32 chars recommended) | *(required)* |
| `JWT_EXPIRY` | Session token lifetime | `24h` |
| `AUTH_RATE_LIMIT_RPS` | Auth endpoint rate (req/s) | `0.5` |
| `AUTH_RATE_LIMIT_BURST` | Auth endpoint burst | `5` |
| `API_RATE_LIMIT_RPS` | API rate (req/s) | `2` |
| `API_RATE_LIMIT_BURST` | API burst | `20` |
| `MAX_BODY_SIZE` | Max JSON body bytes | `1048576` (1 MB) |
