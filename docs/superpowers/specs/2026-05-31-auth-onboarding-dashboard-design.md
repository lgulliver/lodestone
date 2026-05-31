# Auth, Onboarding & Dashboard — Design

Date: 2026-05-31
Status: Approved (pending spec review)
Scope: Login/register pages, first-time setup wizard, personalized dashboard, plus the backend changes they require.

## Goal

Give the self-hosted Lodestone web UI a complete first-run and authenticated experience:

1. A fresh install boots into a **setup wizard** that creates the first admin and configures the instance.
2. Existing users hit a **login page** (local accounts; SSO placeholder for the future).
3. After login, users land on a **personalized dashboard** ("my stuff").

Auth uses an **httpOnly cookie** for the JWT (not localStorage), so the token is not readable by JavaScript.

## Delivery approach

**Backend-first, one PR per slice.** Each slice merges independently and must stay green on the 80% coverage gate (`make test`). Frontend is built against real, merged endpoints — no mocked auth.

Order: Slice 1 (cookie auth) → Slice 2 (setup) → frontend login + wizard → Slice 3 / dashboard.

## Backend changes

### Slice 1 — Cookie auth + session endpoints

- **`handleLogin`** (`cmd/api-gateway/routes/auth.go`): in addition to the existing JSON `{token, expires_at, user}` response, set cookie `lodestone_token`:
  - `HttpOnly=true`, `Secure=true`, `SameSite=Strict`, `Path=/`, expiry = JWT `expires_at`.
  - JSON body retained so CLI/scripts keep working.
- **`AuthMiddleware`** (`cmd/api-gateway/middleware/auth.go`): after the existing `Authorization: Bearer` header check and **before** the API-key checks, read the JWT from the `lodestone_token` cookie. Header always takes precedence — zero impact on Docker CLI / API-key / OCI flows.
- **`POST /api/v1/auth/logout`**: clears `lodestone_token` (MaxAge -1). No-op if not authenticated. 200.
- **`GET /api/v1/auth/me`**: returns the current user (from cookie or header) as `{id, username, email, is_admin}`. 401 if unauthenticated. Used by the SPA to rehydrate session state on boot.

CSRF: `SameSite=Strict` + same-origin SPA is sufficient for v1. No CSRF token unless cross-site is later allowed. Documented as a known constraint.

### Slice 2 — First-time setup / admin bootstrap

- **`GET /api/v1/auth/setup-status`** (public): `{needs_setup: bool}`, true when `users` count == 0.
- **`POST /api/v1/auth/setup`** (public, single-use): creates the first user with `IsAdmin:true`, sets the auth cookie, returns the user. Returns **409 Conflict** if any user already exists. This is the only path that creates an admin without an existing admin.
- Wizard steps 2 (registries) and 3 (instance) call the **existing** registry-settings service behind the normal admin guard. No new endpoints for those.

### Slice 3 — Dashboard data

- **API keys**: `GET /api/v1/auth/api-keys` — already exists.
- **My artifacts**: reuse existing package-ownership routes.
- **Recent activity (my pulls/publishes)**: **no endpoint exists.** v1 ships this one dashboard card with mock data behind a visible `TODO`/`mock` badge. A real `GET /api/v1/me/activity` is deferred to a later change — explicitly out of scope here to avoid building an analytics subsystem.

## Frontend architecture

### Auth layer

- **Pinia `useAuthStore`**: holds `user` only (never the token — the token lives in the httpOnly cookie, invisible to JS). No token or user persisted to localStorage.
- **Boot sequence**: on app start the store calls `GET /auth/me`. 200 → hydrate `user`. 401 → unauthenticated. The cookie is the sole source of truth; a valid cookie in any browser rehydrates correctly.
- **`lib/api.ts` fetch wrapper**: `credentials: 'include'` on every request so the cookie rides along. Centralized 401 handling → clear store → redirect to `/login`.
- **Logout**: `POST /auth/logout` → clear store → `/login`.

### Routing & guards (`router/index.ts`)

- Public routes: `/login`, `/register`, `/setup`.
- Everything under `AppLayout` requires `user`.
- Global `beforeEach`:
  1. Call `GET /auth/setup-status` once (cached for the session). If `needs_setup` → force redirect to `/setup` (and `/setup` redirects away once setup is done).
  2. Else if the target route requires auth and `user` is null → redirect to `/login`.

## UI designs

Theme: Technical Precision (DESIGN.md) — primary navy `#00288e`, background `#f7f9fb`, Geist + JetBrains Mono, 4px radius.

### Login page — **split brand panel** (layout A)

- Two-column: left navy gradient brand panel (logo, tagline, supported formats); right form panel.
- Fields: username, password, primary "Sign in" button, disabled "SSO (coming soon)" button, link to register.
- Register page: same shell, register fields, calls `POST /auth/register` (normal non-admin user).

### Setup wizard — **sidebar steps** (layout B)

- Navy left sidebar lists all 3 steps with progress; form on the right. "Installer" feel, consistent with login's brand panel.
- Step 1 — Create admin account: username/email/password → `POST /auth/setup`.
- Step 2 — Enable registries: toggles for package formats (npm, oci, maven, cargo, go, helm, nuget, rubygems, opa) via registry-settings service.
- Step 3 — Instance & storage: instance name, base URL, storage backend info (mostly display/confirm since storage is env-configured).
- On completion → dashboard.

### Dashboard — **stat row + 2-col grid** (layout A)

- Inside existing `AppLayout` (sidebar + topbar).
- Top: greeting ("Welcome back, {username}") + quick actions (publish, new API key).
- Stat tiles: My Artifacts count, API Keys count, My Pulls (24h, from mocked activity).
- Body: **My Artifacts** table/list beside a stacked column of **API Keys** and **Recent Activity** (activity = mock + TODO badge).

## Error handling

- 401 anywhere → clear store, redirect to login.
- `POST /auth/setup` 409 → setup already done; redirect to login.
- `GET /auth/me` 401 on boot → treat as unauthenticated, no error toast.
- Login invalid credentials (401) → inline form error, no redirect.
- Backend unreachable (existing pattern, cf. `useHealth`) → graceful messaging, no crash.

## Testing

- Backend slices: handler + middleware tests for cookie set/read precedence, logout clearing, `setup-status` true/false, `setup` single-use 409, `me` 200/401. Keep coverage ≥ 80% per slice.
- Frontend: router guard logic (setup redirect, auth redirect), auth store boot/hydrate/401, login + setup form submit happy/error paths.

## Out of scope (v1)

- Real SSO/SAML (placeholder button only).
- Real "my activity" analytics endpoint (card mocked).
- CSRF token (relying on SameSite=Strict).
- Refresh-token flow (cookie expiry = JWT expiry; re-login on expiry).
