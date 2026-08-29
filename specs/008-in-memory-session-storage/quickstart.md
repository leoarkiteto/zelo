# Quickstart: In-Memory Session Storage (Redis)

Validation guide for the feature. Implementation details live in `tasks.md` and the code; this file documents how to prove the behavior end to end.

## Prerequisites

- Docker + Docker Compose (for PostgreSQL and Redis)
- Go 1.27+ and the repo's normal toolchain (`make`, `templ`, `tailwindcss`)
- `.env` based on `.env.example` (create once with real secrets)

## 1. Start dependencies

```bash
docker compose up -d
```

Expected: `zelo-postgres` and `zelo-redis` containers running and healthy.

## 2. Configure environment

```bash
cp .env.example .env   # then fill SESSION_SECRET and PASSWORD_PEPPER
```

Ensure `.env` contains:

```text
DATABASE_URL=postgres://zelo:zelo@localhost:5432/zelo?sslmode=disable
REDIS_URL=redis://localhost:6379/0
```

## 3. Run migrations and start the app

```bash
make migrate
make run
```

Expected: migrations apply (including `0012_drop_sessions.sql`) and the server listens on `:8080`.

## 4. Verify the PostgreSQL table is gone

```bash
docker compose exec postgres psql -U zelo -d zelo -c '\dt'
```

Expected: no `sessions` table appears in the list.

## 5. Log in and inspect Redis

1. Open `http://localhost:8080/login` and sign in with a seeded/registered user.
2. Inspect the session key:

```bash
docker compose exec redis redis-cli --scan --pattern 'zelo:session:*'
docker compose exec redis redis-cli TTL <returned-key>
```

Expected: one `zelo:session:{hash}` key exists; TTL is positive and ≤ 604800 seconds (7 days).

## 6. Application restart does NOT log the user out

1. Stop and restart only the app (`Ctrl-C`, then `make run` again). Do not touch Redis.
2. Refresh a protected page with the same browser cookie.

Expected: the user stays authenticated.

## 7. Redis restart DOES log the user out

```bash
docker compose restart redis
```

Then refresh the protected page.

Expected: the session cookie is rejected and the user is redirected to login.

## 8. Logout removes the session immediately

1. Log in again and note the Redis key.
2. Log out in the UI.
3. Check the key:

```bash
docker compose exec redis redis-cli EXISTS <returned-key>
```

Expected: `(integer) 0`.

## 9. Expiry is automatic

1. Log in and read the TTL (step 5).
2. Either wait for TTL to reach 0 or shorten the session in Redis for a quick check:

```bash
docker compose exec redis redis-cli EXPIRE <key> 2
```

3. After 2 seconds, refresh a protected page.

Expected: the session is rejected and the key no longer exists.

## 10. Run the automated tests

```bash
make test
```

Expected: all unit tests pass, including the new Redis adapter tests (they use `miniredis` and need no Docker).

Integration tests (optional, require services):

```bash
TEST_DATABASE_URL=postgres://zelo:zelo@localhost:5432/zelo?sslmode=disable \
TEST_REDIS_URL=redis://localhost:6379/0 \
go test ./tests/integration/...
```

Expected: integration suite passes against real PostgreSQL + Redis.
