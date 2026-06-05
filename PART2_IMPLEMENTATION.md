# Part 2 — My Implemented Features

> Two additional features implemented beyond the Part 1 baseline, demonstrating independent learning, advanced Go patterns, and improved system capability.

---

## Feature 1: PostgreSQL Database Integration

### What it is
A full persistence layer using PostgreSQL, replacing any in-memory or file-based storage. Every user and track is durably stored and survives server restarts.

### Files
| File | Role |
|---|---|
| `internal/database/database.go` | Connection pool, auto-migrations |
| `internal/database/user_repo.go` | User CRUD with bcrypt-safe queries |
| `internal/database/track_repo.go` | Track CRUD, search, UPSERT |

### How it works

**Connection Pool (`database.go`)**

Uses `pgxpool` — Go's high-performance PostgreSQL driver — with a pool of up to 10 concurrent connections. On startup, it automatically runs idempotent schema migrations so the database is always ready without manual SQL setup:

```go
db, err := database.New(ctx, os.Getenv("DATABASE_URL"))
// → creates tables: users, tracks, playlists, playlist_tracks
```

**Repository Pattern (`user_repo.go`, `track_repo.go`)**

Handlers never write raw SQL. Instead they call typed repository methods that use parameterized queries to prevent SQL injection:

```go
// Register a user
user, err := userRepo.Create(ctx, id, username, passwordHash, role)

// Get all tracks
tracks, err := trackRepo.GetAll(ctx)

// Search by title/artist/album
results, err := trackRepo.Search(ctx, "beethoven")
```

**Idempotent Track Syncing (`track_repo.go`)**

The background scanner calls `Upsert()` — uses PostgreSQL's `ON CONFLICT DO UPDATE` so re-scanning a directory never creates duplicate tracks:

```sql
INSERT INTO tracks (...) VALUES (...)
ON CONFLICT (file_path) DO UPDATE SET title=..., artist=..., updated_at=NOW()
```

**Why it improves the system:**
- Data persists across restarts — no lost library scans
- Multiple users can be stored safely with unique constraints
- `pgxpool` handles concurrent requests without connection exhaustion
- Migrations run automatically — zero manual DB setup required

---

## Feature 2: JWT Authentication (Custom Implementation)

### What it is
A complete stateless authentication system built from scratch using Go's standard library — no external JWT package. Every protected API route requires a valid signed token.

### Files
| File | Role |
|---|---|
| `internal/auth/jwt.go` | Token generation and validation |
| `internal/server/middleware.go` | `AuthMiddleware` that enforces JWT on protected routes |
| `internal/auth/auth_test.go` | `TestJWTRoundtrip` — validates correctness |

### How it works

**Token Generation (`jwt.go`)**

When a user logs in successfully, a JWT is generated using HMAC-SHA256 — implemented using only `crypto/hmac` and `crypto/sha256` from the Go standard library:

```go
func GenerateToken(username, role string, secret []byte, ttl time.Duration) (string, error)
```

The token contains these claims:
- `sub` — username
- `role` — user or admin
- `iat` — issued at (Unix timestamp)
- `exp` — expires at (24 hours from issue)

**Token Validation (`jwt.go`)**

```go
func ValidateToken(tokenStr string, secret []byte) (*Claims, error)
```

Validation performs:
1. **Structure check** — must have exactly 3 dot-separated parts
2. **Signature verification** — uses `hmac.Equal()` for constant-time comparison (prevents timing attacks)
3. **Expiry check** — rejects tokens past their `exp` timestamp
4. **Payload decode** — extracts username and role from base64-encoded JSON

**AuthMiddleware (`middleware.go`)**

Every protected route is wrapped with `AuthMiddleware`. It reads the `Authorization: Bearer <token>` header, validates the token, and injects the user's identity into the request for downstream handlers:

```go
// Inject into request context for handlers downstream
r.Header.Set("X-User", claims.Username)
r.Header.Set("X-Role", claims.Role)
```

If the token is missing, invalid, or expired — the request is rejected with `401 Unauthorized` before it reaches the handler.

**Route Protection (`server.go`)**

```
Public  → /api/health, /api/auth/register, /api/auth/login
Protected (JWT required) → /api/library/*, /api/stream/*
```

**Tested in `auth_test.go`:**

`TestJWTRoundtrip` verifies:
- Token generated for `alice` with role `admin` decodes correctly
- Wrong secret produces `ErrInvalidToken`

**Why it improves the system:**
- **Stateless** — no session table needed; the token itself carries identity
- **Secure** — constant-time HMAC comparison prevents timing side-channel attacks
- **Extensible** — `role` claim enables future admin-only endpoints
- **Standard** — any HTTP client (browser, curl, mobile app) can use it with a simple `Authorization` header

---

## How the Two Features Work Together

```
POST /api/auth/login
  → bcrypt verifies password against PostgreSQL (UserRepo)
  → JWT generated with HMAC-SHA256
  → Token returned to client

GET /api/library/tracks
  Authorization: Bearer <token>
  → AuthMiddleware validates JWT signature + expiry
  → TrackRepo queries PostgreSQL
  → Tracks returned
```

The database stores verifiable credentials; JWT proves identity on every subsequent request without hitting the database again — a clean separation of authentication (proving who you are) from authorization (what you can access).
