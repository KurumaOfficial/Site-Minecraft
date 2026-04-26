# ESTELAR Backend

Go Fiber backend for the `Site` storefront.

What is implemented:

- public catalog API
- order quote and order creation API
- order lookup by public ID
- hidden admin panel backend with Discord auth verification through Supabase
- manual fulfillment queue for Minecraft nickname-based delivery
- product and promo management endpoints
- promo usage counters
- file storage, direct Postgres storage, and Supabase REST storage modes

Public endpoints:

- `GET /api/v1/health`
- `GET /api/v1/meta`
- `GET /api/v1/catalog`
- `POST /api/v1/orders/quote`
- `POST /api/v1/orders`
- `GET /api/v1/orders/:id`

Admin endpoints:

- `GET /api/v1/admin/session`
- `GET /api/v1/admin/dashboard`
- `GET /api/v1/admin/orders`
- `PATCH /api/v1/admin/orders/:id`
- `GET /api/v1/admin/catalog`
- `POST /api/v1/admin/catalog`
- `GET /api/v1/admin/promos`
- `POST /api/v1/admin/promos`

Storage modes:

- `DB_DRIVER=auto` with empty `DATABASE_URL`: local file mode
- `DB_DRIVER=postgres` or `auto` with `DATABASE_URL`: direct Postgres mode
- `DB_DRIVER=supabase`: Supabase REST mode through the server-side service key

Supabase notes:

- `SUPABASE_PUBLISHABLE_KEY` or `SUPABASE_ANON_KEY` is used only by the frontend auth client.
- `SUPABASE_SECRET_KEY` or `SUPABASE_SERVICE_ROLE_KEY` must stay server-side only.
- Discord provider must be enabled in the Supabase dashboard for admin login.
- `ADMIN_ALLOWED_DISCORD_IDS` is the preferred whitelist for admins.
- `ADMIN_ALLOWED_EMAILS` can stay as a fallback whitelist when the Discord account has a confirmed email.
- Promo codes should be managed from the database or admin panel in persistent storage mode, not from `.env`.
- Supabase REST mode requires the SQL schema to already exist in the project.

Apply schema:

- run [db/supabase/schema.sql](/C:/Users/gameg/Desktop/Holo_Project/Site/backend/db/supabase/schema.sql:1)
- then run [db/supabase/seed_catalog.sql](/C:/Users/gameg/Desktop/Holo_Project/Site/backend/db/supabase/seed_catalog.sql:1)

Local start:

```powershell
cd Site\backend
Copy-Item .env.example .env
& 'C:\Program Files\Go\bin\go.exe' run ./cmd/server
```

Production checklist:

- set `PUBLIC_SITE_URL` and `ADMIN_REDIRECT_URL` to final `https://...` values
- configure one persistent storage mode:
  - `DB_DRIVER=supabase` + `SUPABASE_URL` + `SUPABASE_SECRET_KEY`
  - or `DB_DRIVER=postgres` + `DATABASE_URL`
- fill `ADMIN_ALLOWED_DISCORD_IDS` and/or `ADMIN_ALLOWED_EMAILS`
- keep `ADMIN_LOCAL_BYPASS=false` in production
- review `CORS_ALLOW_ORIGINS` only if you need custom origins; otherwise it is derived from the public URLs

Security hardening already enabled:

- body limits and request timeouts
- recover, request id, etag, compression
- CSP and safe cross-origin headers for Supabase auth and CDN script loading
- API, order, and admin rate limits
- strict JSON parsing with unknown-field rejection
- server-side token verification against Supabase Auth
- no-store cache policy on API responses
