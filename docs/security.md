# Security

This document covers the security model, threat mitigations, and operational procedures for TPT Online Video.

---

## Authentication model

| Mechanism | Usage |
|-----------|-------|
| JWT (HS256, 15-min TTL) | Access token; sent as `Authorization: Bearer <token>` |
| Opaque refresh token (SHA-256 hash stored in DB) | Long-lived session token; rotation + reuse detection |
| Argon2id | Password hashing |
| SHA-256 | Stream key hashing (key displayed once on creation) |

### CSRF strategy

The API is a pure JSON REST service.  All state-changing endpoints (`POST`, `PATCH`, `PUT`, `DELETE`) require the caller to supply an `Authorization: Bearer <jwt>` header.  Browsers cannot attach arbitrary headers to cross-origin form or navigation requests, so bearer-token authentication is inherently CSRF-safe for XHR and `fetch` callers.

The optional cookie fallback (`access_token` cookie, read in `extractToken`) is an escape hatch for environments that cannot set headers.  **If this cookie is used, it must be set with `HttpOnly`, `Secure`, and `SameSite=Strict` attributes to prevent CSRF from other origins.**

---

## CORS policy

Allowed origins are configured via `CORS_ALLOWED_ORIGINS` (comma-separated).  Wildcards are not accepted.  The middleware sets `Access-Control-Allow-Origin` only for requests whose `Origin` header exactly matches the allowlist; all other cross-origin requests receive no CORS headers and are blocked by the browser.

```
CORS_ALLOWED_ORIGINS=https://app.example.com,https://admin.example.com
```

---

## Input validation

All user-submitted text fields are validated and sanitised before persistence:

| Field | Max length |
|-------|-----------|
| Video title / Live stream title | 200 characters |
| Video / stream description | 5 000 characters |
| User display name | 100 characters |
| User bio | 1 000 characters |
| Comment body | 5 000 characters |

Sanitisation strips null bytes and non-printable control characters.  The API returns JSON; React escapes HTML on render, so no server-side HTML escaping is required.

---

## Content ownership checks

All mutating operations on user-owned resources verify the calling user owns the resource **before** executing the mutation.  Admins and moderators bypass ownership checks on routes where that is intentional (video management, user management).

---

## Admin authorisation

- All `/api/v1/admin/*` routes require `role = admin OR moderator` (enforced by `RequireModOrAdmin` middleware).
- Role changes and status changes (`PATCH /api/v1/admin/users/:id`) additionally require `role = admin` (enforced inside the handler).
- Admins cannot modify their own account via the admin endpoint to prevent accidental self-lockout.
- All admin actions are written to the audit log.

---

## Rate limiting

| Tier | Limit | Applies to |
|------|-------|-----------|
| Strict | 10 req/min, burst 3 | Auth endpoints (login, register, forgot-password, reset-password) |
| Admin | 30 req/min | Admin panel |
| Global | 100 req/min, burst 10 | All other endpoints |

Limits use a Redis sliding-window algorithm.  Keys are IP-based for unauthenticated routes and fall back to IP for authenticated routes (user-ID keying is available via `UserIDKeyFunc` if stricter per-account limiting is needed).

---

## Secret rotation procedures

### JWT secret (`JWT_SECRET`)

1. Generate a new 256-bit secret: `openssl rand -hex 32`
2. Update the environment variable in your deployment.
3. Restart the API service.  Existing access tokens (≤ 15 min TTL) will be rejected immediately; users will be prompted to log in again.  Refresh tokens are opaque and stored hashed in the DB — they are not affected by the JWT secret rotation.

### Live hook secret (`LIVE_HOOK_SECRET`)

1. Generate a new secret: `openssl rand -hex 32`
2. Update both `LIVE_HOOK_SECRET` (API) and the matching value in the MediaMTX configuration simultaneously to avoid a gap.
3. Rolling restart: update MediaMTX first, then the API (the gap is limited to the MediaMTX→API call window).

### Database passwords (`POSTGRES_PASSWORD`)

1. Create the new password in PostgreSQL: `ALTER USER tpt WITH PASSWORD 'new-password';`
2. Update `POSTGRES_PASSWORD` in the deployment environment.
3. Restart the API, worker, and any other services that hold a DB connection pool.

### S3/MinIO credentials (`S3_ACCESS_KEY_ID` / `S3_SECRET_ACCESS_KEY`)

1. Create a new access key in your S3-compatible provider.
2. Update the environment variables.
3. Restart all services.
4. Revoke the old access key only after confirming all services are using the new key.

### Redis password (`REDIS_PASSWORD`)

1. Set the new password in Redis: `CONFIG SET requirepass 'new-password'`
2. Update `REDIS_PASSWORD` in all services.
3. Restart services.

---

## Security checklist (pre-production)

- [ ] `JWT_SECRET` is a cryptographically random value (not the development default)
- [ ] `LIVE_HOOK_SECRET` is set to a random value (not `changeme-live-hook-secret`)
- [ ] `ADMIN_PASSWORD` is set to a strong password (not the `.env.example` placeholder)
- [ ] `CORS_ALLOWED_ORIGINS` contains only your actual frontend origins (no wildcards)
- [ ] Database is not publicly accessible; connection uses TLS in production (remove `sslmode=disable` from DSN)
- [ ] Redis is password-protected and not publicly accessible
- [ ] S3/MinIO credentials are scoped to the minimum required permissions (read/write the media bucket only)
- [ ] HTTPS is enforced at the load balancer / reverse proxy level
- [ ] `APP_ENV=production` is set (enables JWT secret enforcement)
- [ ] Upload size limits are reviewed for your infrastructure capacity
- [ ] Audit log is retained and monitored
- [ ] Admin seed account email/password are changed after first login

---

## Known limitations / future work

- **OAuth 2.0 with PKCE**: the `POST /api/v1/auth/oauth/{provider}` endpoint returns `501 Not Implemented`.  When implemented, the state parameter must be validated against a server-side nonce (Redis) before exchanging the authorisation code, and PKCE must be used for public clients.
- **Per-user rate limiting**: current rate limits are IP-based.  For shared-IP environments (NAT, IPv6 pools), consider enabling `UserIDKeyFunc` for authenticated endpoints.
- **Presigned URL TTL**: currently hardcoded to 1 hour for most GET operations.  Make configurable per operation type if shorter-lived URLs are required.
- **Stream key rotation**: users can rotate their stream key by deleting and recreating the live stream.  A dedicated key-rotation endpoint would be safer.
