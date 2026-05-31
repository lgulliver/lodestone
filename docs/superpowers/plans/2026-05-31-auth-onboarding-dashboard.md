# Auth, Onboarding & Dashboard Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add cookie-based login/register, a first-run setup wizard that bootstraps the first admin, and a personalized dashboard to the self-hosted Lodestone web UI.

**Architecture:** Backend-first, one PR per phase. The JWT is delivered to the SPA as an httpOnly cookie (additive to the existing `Authorization: Bearer` header path, which CLI/Docker/API-key clients keep using). The SPA holds only the `user` object in a Pinia store, rehydrated on boot via `GET /auth/me`. Setup is gated by a public `setup-status` check that is true only while zero users exist.

**Tech Stack:** Go 1.26 (gin, gorm), Vue 3.5 (`<script setup>` + TS), Pinia, Vue Router, @tanstack/vue-query, Tailwind v4, Vitest + @vue/test-utils (added in Phase 3).

**Spec:** `docs/superpowers/specs/2026-05-31-auth-onboarding-dashboard-design.md`

**Conventions for this repo:**
- Run Go tooling through the RTK proxy: `rtk proxy go test ...`.
- Backend route tests mirror the harness in `cmd/api-gateway/routes/harness_test.go` (sqlite in-memory + real services).
- Coverage gate: `make test` enforces ≥80% on the configured scope. Each backend phase must keep it green.
- Cookie name: `lodestone_token`. Set with `HttpOnly`, `Secure`, `SameSite=Strict`, `Path=/`.

---

## Phase 1 — Cookie auth + session endpoints (PR: `feat/auth-cookie-session`)

Files in play:
- Modify: `cmd/api-gateway/routes/auth.go` — login sets cookie; add logout + me handlers; register routes.
- Modify: `cmd/api-gateway/middleware/auth.go` — read token from cookie after header, before API-key.
- Create: `cmd/api-gateway/routes/auth_session_test.go` — tests for cookie/login/logout/me.
- Modify: `cmd/api-gateway/middleware/auth_test.go` — cookie-read test.

### Task 1: Login sets the auth cookie

**Files:**
- Modify: `cmd/api-gateway/routes/auth.go:130-135` (the `handleLogin` JSON response)
- Test: `cmd/api-gateway/routes/auth_session_test.go`

- [ ] **Step 1: Write the failing test**

Create `cmd/api-gateway/routes/auth_session_test.go`:

```go
package routes

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/lgulliver/lodestone/cmd/api-gateway/middleware"
)

// sessionHarness wires AuthRoutes onto the test harness router.
func sessionHarness(t *testing.T) *testHarness {
	h := newHarness(t)
	api := h.router.Group("/api/v1")
	AuthRoutes(api, h.auth)
	return h
}

func loginBody(username, password string) []byte {
	b, _ := json.Marshal(map[string]string{"username": username, "password": password})
	return b
}

func findCookie(resp *http.Response, name string) *http.Cookie {
	for _, c := range resp.Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}

func TestLogin_SetsHttpOnlyCookie(t *testing.T) {
	h := sessionHarness(t)

	w := h.do("POST", "/api/v1/auth/login", loginBody("routeuser", "testpassword123"), "application/json")
	require.Equal(t, http.StatusOK, w.Code)

	c := findCookie(w.Result(), "lodestone_token")
	require.NotNil(t, c, "expected lodestone_token cookie to be set")
	require.True(t, c.HttpOnly, "cookie must be HttpOnly")
	require.Equal(t, http.SameSiteStrictMode, c.SameSite)
	require.Equal(t, "/", c.Path)
	require.NotEmpty(t, c.Value)
}

var _ = gin.Version
var _ = strings.TrimSpace
var _ = middleware.GetUserFromContext
```

> Note: the `var _ =` lines keep imports used while tasks are built incrementally; remove them once every test in the file references the packages.

- [ ] **Step 2: Run test to verify it fails**

Run: `rtk proxy go test ./cmd/api-gateway/routes/ -run TestLogin_SetsHttpOnlyCookie -v`
Expected: FAIL — no `lodestone_token` cookie set (cookie is nil).

- [ ] **Step 3: Implement — set the cookie in `handleLogin`**

In `cmd/api-gateway/routes/auth.go`, replace the success block (currently lines 130-135):

```go
		c.SetSameSite(http.SameSiteStrictMode)
		c.SetCookie(
			"lodestone_token",
			authToken.Token,
			int(time.Until(authToken.ExpiresAt).Seconds()),
			"/",
			"",   // domain: default to request host
			true, // secure
			true, // httpOnly
		)

		c.JSON(http.StatusOK, gin.H{
			"token":      authToken.Token,
			"expires_at": authToken.ExpiresAt,
			"user": gin.H{
				"id": authToken.UserID,
			},
		})
```

Add `"time"` to the import block in `auth.go` if not already present.

- [ ] **Step 4: Run test to verify it passes**

Run: `rtk proxy go test ./cmd/api-gateway/routes/ -run TestLogin_SetsHttpOnlyCookie -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add cmd/api-gateway/routes/auth.go cmd/api-gateway/routes/auth_session_test.go
git commit -m "feat(auth): set httpOnly JWT cookie on login"
```

### Task 2: Middleware reads token from cookie

**Files:**
- Modify: `cmd/api-gateway/middleware/auth.go` (insert cookie check after the header block, before the `X-API-Key` block — after line 97)
- Test: `cmd/api-gateway/middleware/auth_test.go`

- [ ] **Step 1: Write the failing test**

Append to `cmd/api-gateway/middleware/auth_test.go` (reuse the existing mock auth service in that file — it already defines a fake implementing `AuthServiceInterface` with `ValidateToken`):

```go
func TestAuthMiddleware_ValidCookieToken(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mock := &mockAuthService{
		validateTokenFunc: func(ctx context.Context, token string) (*types.User, error) {
			if token == "cookie-jwt" {
				return &types.User{Username: "cookieuser"}, nil
			}
			return nil, errors.New("invalid")
		},
	}

	router := gin.New()
	router.Use(authMiddlewareWithInterface(mock))
	router.GET("/protected", func(c *gin.Context) {
		u, _ := GetUserFromContext(c)
		c.JSON(200, gin.H{"username": u.Username})
	})

	req := httptest.NewRequest("GET", "/protected", nil)
	req.AddCookie(&http.Cookie{Name: "lodestone_token", Value: "cookie-jwt"})
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, 200, w.Code)
	require.Contains(t, w.Body.String(), "cookieuser")
}
```

> Match `mockAuthService`'s actual field/constructor names in `auth_test.go`; the snippet above assumes a `validateTokenFunc` hook. If the existing mock is struct-based with fixed returns, follow that file's established pattern instead and configure it to accept `"cookie-jwt"`.

- [ ] **Step 2: Run test to verify it fails**

Run: `rtk proxy go test ./cmd/api-gateway/middleware/ -run TestAuthMiddleware_ValidCookieToken -v`
Expected: FAIL — 401, cookie never consulted.

- [ ] **Step 3: Implement — cookie read block**

In `cmd/api-gateway/middleware/auth.go`, immediately after the closing brace of the `if authHeader != ""` block (after line 97) and before `// Check for API key in X-API-Key header` (line 99), insert:

```go
		// Check for JWT in the session cookie (browser SPA). Header always wins;
		// this only runs when no Authorization header was supplied.
		if cookie, err := c.Cookie("lodestone_token"); err == nil && cookie != "" {
			ctx := context.WithValue(c.Request.Context(), contextKeyToken, cookie)
			if user, err := authService.ValidateToken(ctx, cookie); err == nil {
				c.Set("user", user)
				c.Next()
				return
			}
			log.Debug().Str("path", c.Request.URL.Path).Msg("cookie JWT validation failed")
		}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `rtk proxy go test ./cmd/api-gateway/middleware/ -run TestAuthMiddleware_ValidCookieToken -v`
Expected: PASS.

- [ ] **Step 5: Run the full middleware suite to confirm no header/API-key regression**

Run: `rtk proxy go test ./cmd/api-gateway/middleware/ -v`
Expected: PASS (existing Bearer/API-key/OCI tests unaffected).

- [ ] **Step 6: Commit**

```bash
git add cmd/api-gateway/middleware/auth.go cmd/api-gateway/middleware/auth_test.go
git commit -m "feat(auth): read JWT from session cookie in auth middleware"
```

### Task 3: Logout endpoint

**Files:**
- Modify: `cmd/api-gateway/routes/auth.go` (add `handleLogout`, register route)
- Test: `cmd/api-gateway/routes/auth_session_test.go`

- [ ] **Step 1: Write the failing test**

Append to `auth_session_test.go`:

```go
func TestLogout_ClearsCookie(t *testing.T) {
	h := sessionHarness(t)

	w := h.do("POST", "/api/v1/auth/logout", nil, "")
	require.Equal(t, http.StatusOK, w.Code)

	c := findCookie(w.Result(), "lodestone_token")
	require.NotNil(t, c)
	require.True(t, c.MaxAge < 0, "logout must expire the cookie (MaxAge < 0)")
	require.Empty(t, c.Value)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `rtk proxy go test ./cmd/api-gateway/routes/ -run TestLogout_ClearsCookie -v`
Expected: FAIL — route 404 (not registered).

- [ ] **Step 3: Implement — handler + route**

In `AuthRoutes` (`auth.go`), add to the public routes group:

```go
	auth.POST("/logout", handleLogout())
```

Add the handler:

```go
// Logout godoc
//
//	@Summary		Log out
//	@Description	Clears the session cookie
//	@Tags			Authentication
//	@Success		200	{object}	object{message=string}
//	@Router			/auth/logout [post]
func handleLogout() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.SetSameSite(http.SameSiteStrictMode)
		c.SetCookie("lodestone_token", "", -1, "/", "", true, true)
		c.JSON(http.StatusOK, gin.H{"message": "logged out"})
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `rtk proxy go test ./cmd/api-gateway/routes/ -run TestLogout_ClearsCookie -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add cmd/api-gateway/routes/auth.go cmd/api-gateway/routes/auth_session_test.go
git commit -m "feat(auth): add logout endpoint that clears session cookie"
```

### Task 4: `GET /auth/me` endpoint

**Files:**
- Modify: `cmd/api-gateway/routes/auth.go` (add `handleMe`, register under authenticated group)
- Test: `cmd/api-gateway/routes/auth_session_test.go`

- [ ] **Step 1: Write the failing test**

Append to `auth_session_test.go`. The harness must apply `AuthMiddleware` to the authenticated group, so register routes through a helper that mirrors production wiring:

```go
func TestMe_ReturnsCurrentUser(t *testing.T) {
	h := sessionHarness(t)

	w := h.do("GET", "/api/v1/auth/me", nil, "")
	require.Equal(t, http.StatusOK, w.Code)

	var body struct {
		ID       string `json:"id"`
		Username string `json:"username"`
		Email    string `json:"email"`
		IsAdmin  bool   `json:"is_admin"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Equal(t, "routeuser", body.Username)
	require.Equal(t, "route@example.com", body.Email)
	require.False(t, body.IsAdmin)
}

func TestMe_Unauthenticated(t *testing.T) {
	h := sessionHarness(t)
	w := h.doNoAuth("GET", "/api/v1/auth/me")
	require.Equal(t, http.StatusUnauthorized, w.Code)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `rtk proxy go test ./cmd/api-gateway/routes/ -run TestMe -v`
Expected: FAIL — route 404.

- [ ] **Step 3: Implement — handler + route**

In `AuthRoutes`, add to the `authenticated` group:

```go
	authenticated.GET("/me", handleMe())
```

Add the handler (uses the user already placed in context by `AuthMiddleware`):

```go
// Me godoc
//
//	@Summary		Get current user
//	@Description	Returns the authenticated user from the session cookie or bearer token
//	@Tags			Authentication
//	@Produce		json
//	@Success		200	{object}	object{id=string,username=string,email=string,is_admin=bool}
//	@Failure		401	{object}	object{error=string}
//	@Router			/auth/me [get]
func handleMe() gin.HandlerFunc {
	return func(c *gin.Context) {
		user, exists := middleware.GetUserFromContext(c)
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"id":       user.ID,
			"username": user.Username,
			"email":    user.Email,
			"is_admin": user.IsAdmin,
		})
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `rtk proxy go test ./cmd/api-gateway/routes/ -run TestMe -v`
Expected: PASS.

- [ ] **Step 5: Run full routes suite + coverage gate**

Run: `make test`
Expected: PASS, coverage ≥80%.

- [ ] **Step 6: Commit and open PR**

```bash
git add cmd/api-gateway/routes/auth.go cmd/api-gateway/routes/auth_session_test.go
git commit -m "feat(auth): add GET /auth/me for SPA session rehydration"
git push -u origin feat/auth-cookie-session
gh pr create --title "feat(auth): cookie session, logout, and /auth/me" --body "Implements Phase 1 of the auth/onboarding spec: httpOnly cookie on login, cookie read in middleware (header still wins), logout, and /auth/me. See docs/superpowers/specs/2026-05-31-auth-onboarding-dashboard-design.md"
```

---

## Phase 2 — Setup / admin bootstrap (PR: `feat/auth-setup-bootstrap`)

Files in play:
- Modify: `internal/auth/service.go` — add `CountUsers` and `CreateInitialAdmin`.
- Create: `internal/auth/setup_test.go` — service tests.
- Modify: `cmd/api-gateway/routes/auth.go` — add `handleSetupStatus`, `handleSetup`, register routes.
- Modify: `cmd/api-gateway/routes/auth_session_test.go` — endpoint tests.

### Task 5: `CountUsers` service method

**Files:**
- Modify: `internal/auth/service.go`
- Test: `internal/auth/setup_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/auth/setup_test.go`. Reuse the existing service test setup pattern from `internal/auth/service_test.go` (look for its `setupTestService`/`newTestService` helper and call it the same way):

```go
package auth

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/lgulliver/lodestone/pkg/types"
)

func TestCountUsers(t *testing.T) {
	service := setupTestService(t) // same helper used by service_test.go
	ctx := context.Background()

	n, err := service.CountUsers(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(0), n)

	_, err = service.Register(ctx, &types.RegisterRequest{
		Username: "alice", Email: "alice@example.com", Password: "password123",
	})
	require.NoError(t, err)

	n, err = service.CountUsers(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(1), n)
}
```

> If `service_test.go`'s helper has a different name/signature, use that exact one. Do not invent a new DB setup.

- [ ] **Step 2: Run test to verify it fails**

Run: `rtk proxy go test ./internal/auth/ -run TestCountUsers -v`
Expected: FAIL — `service.CountUsers` undefined.

- [ ] **Step 3: Implement `CountUsers`**

Add to `internal/auth/service.go`:

```go
// CountUsers returns the total number of user accounts.
func (s *Service) CountUsers(ctx context.Context) (int64, error) {
	var count int64
	if err := s.db.Model(&types.User{}).Count(&count).Error; err != nil {
		return 0, fmt.Errorf("failed to count users: %w", err)
	}
	return count, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `rtk proxy go test ./internal/auth/ -run TestCountUsers -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/auth/service.go internal/auth/setup_test.go
git commit -m "feat(auth): add CountUsers service method"
```

### Task 6: `CreateInitialAdmin` service method

**Files:**
- Modify: `internal/auth/service.go`
- Test: `internal/auth/setup_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/auth/setup_test.go`:

```go
func TestCreateInitialAdmin(t *testing.T) {
	service := setupTestService(t)
	ctx := context.Background()

	admin, err := service.CreateInitialAdmin(ctx, &types.RegisterRequest{
		Username: "root", Email: "root@example.com", Password: "password123",
	})
	require.NoError(t, err)
	require.True(t, admin.IsAdmin)
	require.Empty(t, admin.Password, "password must be stripped from response")
}

func TestCreateInitialAdmin_RejectsWhenUsersExist(t *testing.T) {
	service := setupTestService(t)
	ctx := context.Background()

	_, err := service.Register(ctx, &types.RegisterRequest{
		Username: "alice", Email: "alice@example.com", Password: "password123",
	})
	require.NoError(t, err)

	_, err = service.CreateInitialAdmin(ctx, &types.RegisterRequest{
		Username: "root", Email: "root@example.com", Password: "password123",
	})
	require.Error(t, err, "must refuse to create an admin once any user exists")
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `rtk proxy go test ./internal/auth/ -run TestCreateInitialAdmin -v`
Expected: FAIL — `CreateInitialAdmin` undefined.

- [ ] **Step 3: Implement `CreateInitialAdmin`**

Add to `internal/auth/service.go`:

```go
// ErrSetupComplete is returned when initial admin creation is attempted after setup.
var ErrSetupComplete = errors.New("setup already completed: a user already exists")

// CreateInitialAdmin creates the first user as an admin. It refuses if any user
// already exists, making it safe to expose on a public, single-use setup route.
func (s *Service) CreateInitialAdmin(ctx context.Context, req *types.RegisterRequest) (*types.User, error) {
	count, err := s.CountUsers(ctx)
	if err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, ErrSetupComplete
	}

	hashedPassword, err := utils.HashPassword(req.Password, s.config.BCryptCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	user := &types.User{
		Username: req.Username,
		Email:    req.Email,
		Password: hashedPassword,
		IsActive: true,
		IsAdmin:  true,
	}
	if err := s.db.Create(user).Error; err != nil {
		return nil, fmt.Errorf("failed to create initial admin: %w", err)
	}

	log.Info().Str("username", user.Username).Msg("Initial admin created")
	user.Password = ""
	return user, nil
}
```

Ensure `"errors"` is imported in `service.go` (add if missing). `utils` and `log` are already imported.

- [ ] **Step 4: Run test to verify it passes**

Run: `rtk proxy go test ./internal/auth/ -run TestCreateInitialAdmin -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/auth/service.go internal/auth/setup_test.go
git commit -m "feat(auth): add CreateInitialAdmin with single-use guard"
```

### Task 7: `GET /auth/setup-status` endpoint

**Files:**
- Modify: `cmd/api-gateway/routes/auth.go`
- Test: `cmd/api-gateway/routes/auth_session_test.go`

- [ ] **Step 1: Write the failing test**

The session harness registers a user in `newHarness`, so setup is already complete there. Add a fresh-DB variant. Append to `auth_session_test.go`:

```go
func TestSetupStatus_NeedsSetupWhenEmpty(t *testing.T) {
	// Build a harness with NO seeded user.
	h := newEmptyAuthHarness(t)

	w := h.doNoAuth("GET", "/api/v1/auth/setup-status")
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `"needs_setup":true`)
}

func TestSetupStatus_CompleteWhenUserExists(t *testing.T) {
	h := sessionHarness(t) // newHarness seeds "routeuser"
	w := h.doNoAuth("GET", "/api/v1/auth/setup-status")
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `"needs_setup":false`)
}
```

Add a no-seed harness builder in `auth_session_test.go` (copy `newHarness` but skip the `Register`/`Login` calls):

```go
func newEmptyAuthHarness(t *testing.T) *testHarness {
	t.Helper()
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&types.User{}, &types.APIKey{}, &types.RegistrySetting{}))

	cdb := &common.Database{DB: db}
	authSvc := auth.NewService(cdb, nil, &config.AuthConfig{
		JWTSecret: "test-secret-key-for-routes", JWTExpiration: time.Hour, BCryptCost: 4,
	})

	router := gin.New()
	api := router.Group("/api/v1")
	AuthRoutes(api, authSvc)

	return &testHarness{router: router, api: api, auth: authSvc, db: cdb}
}
```

Add the matching imports to `auth_session_test.go`: `"time"`, `"gorm.io/driver/sqlite"`, `"gorm.io/gorm"`, `"github.com/lgulliver/lodestone/internal/auth"`, `"github.com/lgulliver/lodestone/internal/common"`, `"github.com/lgulliver/lodestone/pkg/config"`, `"github.com/lgulliver/lodestone/pkg/types"`.

- [ ] **Step 2: Run test to verify it fails**

Run: `rtk proxy go test ./cmd/api-gateway/routes/ -run TestSetupStatus -v`
Expected: FAIL — route 404.

- [ ] **Step 3: Implement — handler + route**

In `AuthRoutes`, add to the public group:

```go
	auth.GET("/setup-status", handleSetupStatus(authService))
```

Add the handler:

```go
// SetupStatus godoc
//
//	@Summary		First-run setup status
//	@Description	Reports whether the instance still needs initial admin setup
//	@Tags			Authentication
//	@Produce		json
//	@Success		200	{object}	object{needs_setup=bool}
//	@Router			/auth/setup-status [get]
func handleSetupStatus(authService *auth.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		count, err := authService.CountUsers(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read setup status"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"needs_setup": count == 0})
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `rtk proxy go test ./cmd/api-gateway/routes/ -run TestSetupStatus -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add cmd/api-gateway/routes/auth.go cmd/api-gateway/routes/auth_session_test.go
git commit -m "feat(auth): add GET /auth/setup-status"
```

### Task 8: `POST /auth/setup` endpoint

**Files:**
- Modify: `cmd/api-gateway/routes/auth.go`
- Test: `cmd/api-gateway/routes/auth_session_test.go`

- [ ] **Step 1: Write the failing test**

Append to `auth_session_test.go`:

```go
func setupBody(username, email, password string) []byte {
	b, _ := json.Marshal(map[string]string{"username": username, "email": email, "password": password})
	return b
}

func TestSetup_CreatesAdminAndSetsCookie(t *testing.T) {
	h := newEmptyAuthHarness(t)

	w := h.do("POST", "/api/v1/auth/setup", setupBody("root", "root@example.com", "password123"), "application/json")
	require.Equal(t, http.StatusCreated, w.Code)
	require.Contains(t, w.Body.String(), `"is_admin":true`)

	c := findCookie(w.Result(), "lodestone_token")
	require.NotNil(t, c)
	require.True(t, c.HttpOnly)
	require.NotEmpty(t, c.Value)
}

func TestSetup_RejectedAfterSetup(t *testing.T) {
	h := sessionHarness(t) // already has a user

	w := h.do("POST", "/api/v1/auth/setup", setupBody("root", "root@example.com", "password123"), "application/json")
	require.Equal(t, http.StatusConflict, w.Code)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `rtk proxy go test ./cmd/api-gateway/routes/ -run TestSetup_ -v`
Expected: FAIL — route 404.

- [ ] **Step 3: Implement — handler + route**

In `AuthRoutes`, add to the public group:

```go
	auth.POST("/setup", handleSetup(authService))
```

Add the handler:

```go
// Setup godoc
//
//	@Summary		First-run setup
//	@Description	Creates the first admin account; rejected once any user exists
//	@Tags			Authentication
//	@Accept			json
//	@Produce		json
//	@Param			user	body		types.RegisterRequest	true	"Initial admin"
//	@Success		201		{object}	object{id=string,username=string,email=string,is_admin=bool}
//	@Failure		400		{object}	object{error=string}
//	@Failure		409		{object}	object{error=string}
//	@Router			/auth/setup [post]
func handleSetup(authService *auth.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req types.RegisterRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		admin, err := authService.CreateInitialAdmin(c.Request.Context(), &req)
		if errors.Is(err, auth.ErrSetupComplete) {
			c.JSON(http.StatusConflict, gin.H{"error": "setup already completed"})
			return
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "setup failed"})
			return
		}

		authToken, err := authService.Login(c.Request.Context(), &types.LoginRequest{
			Username: req.Username, Password: req.Password,
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "setup succeeded but auto-login failed"})
			return
		}

		c.SetSameSite(http.SameSiteStrictMode)
		c.SetCookie("lodestone_token", authToken.Token,
			int(time.Until(authToken.ExpiresAt).Seconds()), "/", "", true, true)

		c.JSON(http.StatusCreated, gin.H{
			"id": admin.ID, "username": admin.Username,
			"email": admin.Email, "is_admin": admin.IsAdmin,
		})
	}
}
```

Add `"errors"` to `auth.go` imports if missing.

- [ ] **Step 4: Run test to verify it passes**

Run: `rtk proxy go test ./cmd/api-gateway/routes/ -run TestSetup_ -v`
Expected: PASS.

- [ ] **Step 5: Full gate + commit + PR**

Run: `make test`
Expected: PASS, coverage ≥80%.

```bash
git add cmd/api-gateway/routes/auth.go cmd/api-gateway/routes/auth_session_test.go
git commit -m "feat(auth): add POST /auth/setup single-use admin bootstrap"
git push -u origin feat/auth-setup-bootstrap
gh pr create --title "feat(auth): first-run setup bootstrap" --body "Phase 2: CountUsers, CreateInitialAdmin, GET /auth/setup-status, POST /auth/setup (sets cookie, 409 after setup)."
```

> No backend changes are needed for wizard steps 2 (registries) and 3 (instance): the admin registry-settings endpoints already exist at `/api/v1/admin/registries/*` (`cmd/api-gateway/routes/admin.go`) behind the `IsAdmin` guard.

---

## Phase 3 — Frontend auth layer (PR: `feat/web-auth-layer`)

All paths under `web/`. This phase adds Vitest, the API client, the Pinia auth store, router guards, and the login/register pages.

Files in play:
- Modify: `web/package.json`, `web/vite.config.ts` (vitest config)
- Create: `web/src/lib/api.ts`
- Create: `web/src/stores/auth.ts`
- Create: `web/src/stores/__tests__/auth.spec.ts`
- Create: `web/src/views/LoginView.vue`, `web/src/views/RegisterView.vue`
- Create: `web/src/layouts/AuthLayout.vue` (split brand panel shell)
- Modify: `web/src/router/index.ts` (public routes + guards)

### Task 9: Add Vitest

**Files:**
- Modify: `web/package.json`, `web/vite.config.ts`

- [ ] **Step 1: Install dev deps**

Run: `cd web && pnpm add -D vitest @vue/test-utils jsdom`
Expected: deps added.

- [ ] **Step 2: Configure vitest in `web/vite.config.ts`**

Add a `test` block to the existing `defineConfig` (keep all existing plugins/resolve/server):

```ts
  test: {
    environment: 'jsdom',
    globals: true,
  },
```

Add a `"test"` script to `web/package.json`:

```json
    "test": "vitest run",
```

- [ ] **Step 3: Smoke test**

Create `web/src/lib/__tests__/smoke.spec.ts`:

```ts
import { describe, it, expect } from 'vitest'
describe('smoke', () => { it('runs', () => { expect(1 + 1).toBe(2) }) })
```

Run: `cd web && pnpm test`
Expected: 1 passing test.

- [ ] **Step 4: Commit**

```bash
git add web/package.json web/pnpm-lock.yaml web/vite.config.ts web/src/lib/__tests__/smoke.spec.ts
git commit -m "chore(web): add vitest test runner"
```

### Task 10: API client wrapper

**Files:**
- Create: `web/src/lib/api.ts`
- Test: `web/src/lib/__tests__/api.spec.ts`

- [ ] **Step 1: Write the failing test**

Create `web/src/lib/__tests__/api.spec.ts`:

```ts
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { apiFetch, ApiError } from '@/lib/api'

beforeEach(() => { vi.restoreAllMocks() })

describe('apiFetch', () => {
  it('sends credentials and parses JSON', async () => {
    const spy = vi.spyOn(global, 'fetch').mockResolvedValue(
      new Response(JSON.stringify({ ok: true }), { status: 200, headers: { 'Content-Type': 'application/json' } }),
    )
    const data = await apiFetch<{ ok: boolean }>('/api/v1/auth/me')
    expect(data.ok).toBe(true)
    expect(spy).toHaveBeenCalledWith('/api/v1/auth/me', expect.objectContaining({ credentials: 'include' }))
  })

  it('throws ApiError with status on non-2xx', async () => {
    vi.spyOn(global, 'fetch').mockResolvedValue(new Response('{}', { status: 401 }))
    await expect(apiFetch('/api/v1/auth/me')).rejects.toMatchObject({ status: 401 })
    await expect(apiFetch('/api/v1/auth/me')).rejects.toBeInstanceOf(ApiError)
  })
})
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd web && pnpm test src/lib/__tests__/api.spec.ts`
Expected: FAIL — module `@/lib/api` not found.

- [ ] **Step 3: Implement `web/src/lib/api.ts`**

```ts
export class ApiError extends Error {
  status: number
  body: unknown
  constructor(status: number, body: unknown) {
    super(`API error ${status}`)
    this.status = status
    this.body = body
  }
}

export async function apiFetch<T>(path: string, init: RequestInit = {}): Promise<T> {
  const res = await fetch(path, {
    credentials: 'include',
    headers: { 'Content-Type': 'application/json', ...(init.headers ?? {}) },
    ...init,
  })

  const text = await res.text()
  const body = text ? JSON.parse(text) : null

  if (!res.ok) {
    throw new ApiError(res.status, body)
  }
  return body as T
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd web && pnpm test src/lib/__tests__/api.spec.ts`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add web/src/lib/api.ts web/src/lib/__tests__/api.spec.ts
git commit -m "feat(web): add credentialed apiFetch wrapper"
```

### Task 11: Pinia auth store

**Files:**
- Create: `web/src/stores/auth.ts`
- Test: `web/src/stores/__tests__/auth.spec.ts`

- [ ] **Step 1: Write the failing test**

Create `web/src/stores/__tests__/auth.spec.ts`:

```ts
import { describe, it, expect, beforeEach, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useAuthStore } from '@/stores/auth'
import * as api from '@/lib/api'

beforeEach(() => { setActivePinia(createPinia()); vi.restoreAllMocks() })

describe('auth store', () => {
  it('hydrates user from /auth/me on init', async () => {
    vi.spyOn(api, 'apiFetch').mockResolvedValue({ id: '1', username: 'root', email: 'r@x.io', is_admin: true })
    const store = useAuthStore()
    await store.init()
    expect(store.user?.username).toBe('root')
    expect(store.isAuthenticated).toBe(true)
  })

  it('stays unauthenticated when /auth/me 401s', async () => {
    vi.spyOn(api, 'apiFetch').mockRejectedValue(new api.ApiError(401, null))
    const store = useAuthStore()
    await store.init()
    expect(store.user).toBeNull()
    expect(store.isAuthenticated).toBe(false)
  })

  it('clears user on logout', async () => {
    vi.spyOn(api, 'apiFetch').mockResolvedValue({} as never)
    const store = useAuthStore()
    store.user = { id: '1', username: 'root', email: 'r@x.io', is_admin: true }
    await store.logout()
    expect(store.user).toBeNull()
  })
})
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd web && pnpm test src/stores/__tests__/auth.spec.ts`
Expected: FAIL — module not found.

- [ ] **Step 3: Implement `web/src/stores/auth.ts`**

```ts
import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { apiFetch, ApiError } from '@/lib/api'

export interface User {
  id: string
  username: string
  email: string
  is_admin: boolean
}

export const useAuthStore = defineStore('auth', () => {
  const user = ref<User | null>(null)
  const ready = ref(false)
  const isAuthenticated = computed(() => user.value !== null)

  async function init() {
    try {
      user.value = await apiFetch<User>('/api/v1/auth/me')
    } catch (e) {
      if (e instanceof ApiError && e.status === 401) {
        user.value = null
      } else {
        user.value = null
      }
    } finally {
      ready.value = true
    }
  }

  async function login(username: string, password: string) {
    await apiFetch('/api/v1/auth/login', {
      method: 'POST',
      body: JSON.stringify({ username, password }),
    })
    user.value = await apiFetch<User>('/api/v1/auth/me')
  }

  async function logout() {
    try {
      await apiFetch('/api/v1/auth/logout', { method: 'POST' })
    } finally {
      user.value = null
    }
  }

  return { user, ready, isAuthenticated, init, login, logout }
})
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd web && pnpm test src/stores/__tests__/auth.spec.ts`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add web/src/stores/auth.ts web/src/stores/__tests__/auth.spec.ts
git commit -m "feat(web): add auth store with /auth/me hydration"
```

### Task 12: Router guards (setup + auth)

**Files:**
- Modify: `web/src/router/index.ts`
- Test: `web/src/router/__tests__/guards.spec.ts`

- [ ] **Step 1: Write the failing test**

Create `web/src/router/__tests__/guards.spec.ts`:

```ts
import { describe, it, expect, beforeEach, vi } from 'vitest'
import { resolveGuard } from '@/router/guards'

beforeEach(() => vi.restoreAllMocks())

describe('resolveGuard', () => {
  it('redirects to /setup when needs_setup', async () => {
    const r = await resolveGuard({ path: '/dashboard', needsAuth: true }, { needsSetup: true, authed: false })
    expect(r).toBe('/setup')
  })
  it('redirects to /login when auth required and not authed', async () => {
    const r = await resolveGuard({ path: '/dashboard', needsAuth: true }, { needsSetup: false, authed: false })
    expect(r).toBe('/login')
  })
  it('allows when authed', async () => {
    const r = await resolveGuard({ path: '/dashboard', needsAuth: true }, { needsSetup: false, authed: true })
    expect(r).toBeNull()
  })
  it('keeps public routes open', async () => {
    const r = await resolveGuard({ path: '/login', needsAuth: false }, { needsSetup: false, authed: false })
    expect(r).toBeNull()
  })
  it('bounces away from /setup once setup complete', async () => {
    const r = await resolveGuard({ path: '/setup', needsAuth: false }, { needsSetup: false, authed: false })
    expect(r).toBe('/login')
  })
})
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd web && pnpm test src/router/__tests__/guards.spec.ts`
Expected: FAIL — `@/router/guards` not found.

- [ ] **Step 3: Implement pure guard logic `web/src/router/guards.ts`**

```ts
export interface RouteInfo { path: string; needsAuth: boolean }
export interface AuthState { needsSetup: boolean; authed: boolean }

// Pure decision function so it is unit-testable without a live router.
export function resolveGuard(route: RouteInfo, state: AuthState): string | null {
  if (state.needsSetup) {
    return route.path === '/setup' ? null : '/setup'
  }
  if (route.path === '/setup') {
    return '/login' // setup already done
  }
  if (route.needsAuth && !state.authed) {
    return '/login'
  }
  return null
}
```

- [ ] **Step 4: Wire it into the real router in `web/src/router/index.ts`**

Add a cached setup-status fetch and a `beforeEach` that calls `resolveGuard`. Replace the existing `createRouter(...)` export so it includes the public routes and guard:

```ts
import { createRouter, createWebHistory } from 'vue-router'
import { apiFetch } from '@/lib/api'
import { useAuthStore } from '@/stores/auth'
import { resolveGuard } from '@/router/guards'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/login', component: () => import('@/views/LoginView.vue'), meta: { public: true } },
    { path: '/register', component: () => import('@/views/RegisterView.vue'), meta: { public: true } },
    { path: '/setup', component: () => import('@/views/SetupWizard.vue'), meta: { public: true } },
    {
      path: '/',
      component: () => import('@/layouts/AppLayout.vue'),
      children: [
        { path: '', redirect: '/dashboard' },
        { path: 'dashboard', component: () => import('@/views/DashboardView.vue') },
        { path: 'explorer', component: () => import('@/views/RegistryExplorer.vue') },
        { path: 'admin', component: () => import('@/views/RegistryExplorer.vue') },
      ],
    },
  ],
})

let cachedNeedsSetup: boolean | null = null
async function needsSetup(): Promise<boolean> {
  if (cachedNeedsSetup !== null) return cachedNeedsSetup
  try {
    const { needs_setup } = await apiFetch<{ needs_setup: boolean }>('/api/v1/auth/setup-status')
    cachedNeedsSetup = needs_setup
  } catch {
    cachedNeedsSetup = false
  }
  return cachedNeedsSetup
}

router.beforeEach(async (to) => {
  const auth = useAuthStore()
  if (!auth.ready) await auth.init()
  const redirect = resolveGuard(
    { path: to.path, needsAuth: !to.meta.public },
    { needsSetup: await needsSetup(), authed: auth.isAuthenticated },
  )
  return redirect ?? true
})

export default router
```

> `SetupWizard.vue`, `DashboardView.vue` are created in Phases 4–5. Until then, create one-line stubs so the router imports resolve: `<template><div>stub</div></template>`. Note this stub in the commit message.

- [ ] **Step 5: Run guard tests + build**

Run: `cd web && pnpm test src/router/__tests__/guards.spec.ts && pnpm build`
Expected: PASS + clean build (with stubs present).

- [ ] **Step 6: Commit**

```bash
git add web/src/router/guards.ts web/src/router/index.ts web/src/router/__tests__/guards.spec.ts web/src/views/SetupWizard.vue web/src/views/DashboardView.vue
git commit -m "feat(web): add setup + auth router guards (with view stubs)"
```

### Task 13: Auth layout (split brand panel)

**Files:**
- Create: `web/src/layouts/AuthLayout.vue`

- [ ] **Step 1: Implement `web/src/layouts/AuthLayout.vue`**

```vue
<script setup lang="ts">
defineProps<{ title: string; subtitle?: string }>()
</script>

<template>
  <div class="grid min-h-screen grid-cols-1 md:grid-cols-2">
    <!-- Brand panel -->
    <div class="relative hidden flex-col justify-between bg-linear-to-br from-primary to-[#1e40af] p-10 text-primary-foreground md:flex">
      <div class="text-2xl font-bold tracking-tight">Lodestone</div>
      <div>
        <h2 class="text-3xl font-bold leading-tight">Self-hosted artifact registry</h2>
        <p class="mt-3 max-w-sm text-sm text-primary-foreground/80">
          One registry for npm, OCI, Maven, Cargo, Go, Helm, NuGet, RubyGems and OPA.
        </p>
        <p class="mt-8 font-mono text-xs text-primary-foreground/60">npm · oci · maven · cargo · go · helm</p>
      </div>
      <div class="text-xs text-primary-foreground/60">Secure. Local. Yours.</div>
    </div>

    <!-- Form panel -->
    <div class="flex items-center justify-center bg-background p-8">
      <div class="w-full max-w-sm">
        <h1 class="text-2xl font-bold tracking-tight text-foreground">{{ title }}</h1>
        <p v-if="subtitle" class="mt-1 text-sm text-muted-foreground">{{ subtitle }}</p>
        <div class="mt-6">
          <slot />
        </div>
      </div>
    </div>
  </div>
</template>
```

- [ ] **Step 2: Build to verify it compiles**

Run: `cd web && pnpm build`
Expected: clean build.

- [ ] **Step 3: Commit**

```bash
git add web/src/layouts/AuthLayout.vue
git commit -m "feat(web): add split-brand auth layout"
```

### Task 14: Login view

**Files:**
- Create: `web/src/views/LoginView.vue`

- [ ] **Step 1: Implement `web/src/views/LoginView.vue`**

```vue
<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import AuthLayout from '@/layouts/AuthLayout.vue'
import Button from '@/components/ui/Button.vue'
import Input from '@/components/ui/Input.vue'
import { useAuthStore } from '@/stores/auth'
import { ApiError } from '@/lib/api'

const router = useRouter()
const auth = useAuthStore()
const username = ref('')
const password = ref('')
const error = ref('')
const loading = ref(false)

async function submit() {
  error.value = ''
  loading.value = true
  try {
    await auth.login(username.value, password.value)
    router.push('/dashboard')
  } catch (e) {
    error.value = e instanceof ApiError && e.status === 401 ? 'Invalid username or password.' : 'Login failed. Try again.'
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <AuthLayout title="Sign in" subtitle="Use your Lodestone account">
    <form class="flex flex-col gap-4" @submit.prevent="submit">
      <label class="text-sm font-medium text-foreground">
        Username
        <Input v-model="username" class="mt-1" autocomplete="username" required />
      </label>
      <label class="text-sm font-medium text-foreground">
        Password
        <Input v-model="password" type="password" class="mt-1" autocomplete="current-password" required />
      </label>
      <p v-if="error" class="text-sm text-destructive">{{ error }}</p>
      <Button type="submit" :disabled="loading">{{ loading ? 'Signing in…' : 'Sign in' }}</Button>
      <Button type="button" variant="secondary" disabled title="Coming soon">SSO (coming soon)</Button>
      <p class="text-center text-sm text-muted-foreground">
        No account?
        <RouterLink to="/register" class="font-medium text-primary hover:underline">Register</RouterLink>
      </p>
    </form>
  </AuthLayout>
</template>
```

> Verify `Input.vue` supports `v-model` and a `type` prop. It does (created in the earlier UI work). If `type` isn't forwarded, add `type?: string` to its props and bind `:type`.

- [ ] **Step 2: Build to verify it compiles**

Run: `cd web && pnpm build`
Expected: clean build.

- [ ] **Step 3: Commit**

```bash
git add web/src/views/LoginView.vue
git commit -m "feat(web): add login view"
```

### Task 15: Register view

**Files:**
- Create: `web/src/views/RegisterView.vue`

- [ ] **Step 1: Implement `web/src/views/RegisterView.vue`**

```vue
<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import AuthLayout from '@/layouts/AuthLayout.vue'
import Button from '@/components/ui/Button.vue'
import Input from '@/components/ui/Input.vue'
import { apiFetch, ApiError } from '@/lib/api'
import { useAuthStore } from '@/stores/auth'

const router = useRouter()
const auth = useAuthStore()
const username = ref('')
const email = ref('')
const password = ref('')
const error = ref('')
const loading = ref(false)

async function submit() {
  error.value = ''
  loading.value = true
  try {
    await apiFetch('/api/v1/auth/register', {
      method: 'POST',
      body: JSON.stringify({ username: username.value, email: email.value, password: password.value }),
    })
    await auth.login(username.value, password.value)
    router.push('/dashboard')
  } catch (e) {
    error.value = e instanceof ApiError && e.status === 400
      ? 'Check your details (username 3–50 chars, valid email, password ≥8).'
      : 'Registration failed. Try again.'
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <AuthLayout title="Create account" subtitle="Register a local Lodestone account">
    <form class="flex flex-col gap-4" @submit.prevent="submit">
      <label class="text-sm font-medium text-foreground">
        Username
        <Input v-model="username" class="mt-1" autocomplete="username" required />
      </label>
      <label class="text-sm font-medium text-foreground">
        Email
        <Input v-model="email" type="email" class="mt-1" autocomplete="email" required />
      </label>
      <label class="text-sm font-medium text-foreground">
        Password
        <Input v-model="password" type="password" class="mt-1" autocomplete="new-password" required />
      </label>
      <p v-if="error" class="text-sm text-destructive">{{ error }}</p>
      <Button type="submit" :disabled="loading">{{ loading ? 'Creating…' : 'Create account' }}</Button>
      <p class="text-center text-sm text-muted-foreground">
        Already have an account?
        <RouterLink to="/login" class="font-medium text-primary hover:underline">Sign in</RouterLink>
      </p>
    </form>
  </AuthLayout>
</template>
```

- [ ] **Step 2: Build + full web tests**

Run: `cd web && pnpm build && pnpm test`
Expected: clean build, all tests pass.

- [ ] **Step 3: Commit + PR**

```bash
git add web/src/views/RegisterView.vue
git commit -m "feat(web): add register view"
git push -u origin feat/web-auth-layer
gh pr create --title "feat(web): auth layer — store, guards, login/register" --body "Phase 3: vitest, apiFetch, auth store (/auth/me hydration), router guards, login + register views. SetupWizard/Dashboard are stubs filled in Phases 4–5."
```

---

## Phase 4 — Setup wizard (PR: `feat/web-setup-wizard`)

Replaces the `SetupWizard.vue` stub with the 3-step sidebar wizard.

Files in play:
- Modify: `web/src/views/SetupWizard.vue`
- Create: `web/src/components/setup/WizardSidebar.vue`
- Create: `web/src/components/setup/StepAdmin.vue`, `StepRegistries.vue`, `StepInstance.vue`
- Create: `web/src/components/setup/__tests__/wizard.spec.ts`

### Task 16: Wizard shell + step state

**Files:**
- Modify: `web/src/views/SetupWizard.vue`
- Create: `web/src/components/setup/WizardSidebar.vue`
- Test: `web/src/components/setup/__tests__/wizard.spec.ts`

- [ ] **Step 1: Write the failing test (step advance logic)**

Create `web/src/components/setup/__tests__/wizard.spec.ts`:

```ts
import { describe, it, expect } from 'vitest'
import { nextStep, prevStep, STEPS } from '@/components/setup/steps'

describe('wizard steps', () => {
  it('has three ordered steps', () => {
    expect(STEPS.map((s) => s.key)).toEqual(['admin', 'registries', 'instance'])
  })
  it('advances and clamps at the end', () => {
    expect(nextStep(0)).toBe(1)
    expect(nextStep(2)).toBe(2)
  })
  it('goes back and clamps at the start', () => {
    expect(prevStep(1)).toBe(0)
    expect(prevStep(0)).toBe(0)
  })
})
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd web && pnpm test src/components/setup/__tests__/wizard.spec.ts`
Expected: FAIL — `@/components/setup/steps` not found.

- [ ] **Step 3: Implement step model `web/src/components/setup/steps.ts`**

```ts
export interface Step { key: 'admin' | 'registries' | 'instance'; label: string }

export const STEPS: Step[] = [
  { key: 'admin', label: 'Admin account' },
  { key: 'registries', label: 'Registries' },
  { key: 'instance', label: 'Instance' },
]

export const nextStep = (i: number) => Math.min(i + 1, STEPS.length - 1)
export const prevStep = (i: number) => Math.max(i - 1, 0)
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd web && pnpm test src/components/setup/__tests__/wizard.spec.ts`
Expected: PASS.

- [ ] **Step 5: Implement `web/src/components/setup/WizardSidebar.vue`**

```vue
<script setup lang="ts">
import { STEPS } from './steps'
defineProps<{ current: number }>()
</script>

<template>
  <aside class="flex w-64 flex-col gap-6 bg-linear-to-b from-primary to-[#1e40af] p-8 text-primary-foreground">
    <div class="text-xl font-bold">Lodestone setup</div>
    <ol class="flex flex-col gap-4">
      <li v-for="(s, i) in STEPS" :key="s.key" class="flex items-center gap-3 text-sm" :class="i > current ? 'opacity-60' : ''">
        <span
          class="flex size-6 items-center justify-center rounded-full text-xs font-semibold"
          :class="i <= current ? 'bg-white text-primary' : 'border border-white/60'"
        >{{ i + 1 }}</span>
        {{ s.label }}
      </li>
    </ol>
  </aside>
</template>
```

- [ ] **Step 6: Implement the wizard shell `web/src/views/SetupWizard.vue`**

```vue
<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import WizardSidebar from '@/components/setup/WizardSidebar.vue'
import StepAdmin from '@/components/setup/StepAdmin.vue'
import StepRegistries from '@/components/setup/StepRegistries.vue'
import StepInstance from '@/components/setup/StepInstance.vue'
import { nextStep, prevStep } from '@/components/setup/steps'

const router = useRouter()
const current = ref(0)

function advance() { current.value = nextStep(current.value) }
function back() { current.value = prevStep(current.value) }
function finish() { router.push('/dashboard') }
</script>

<template>
  <div class="flex min-h-screen">
    <WizardSidebar :current="current" />
    <main class="flex flex-1 items-center justify-center bg-background p-10">
      <div class="w-full max-w-md">
        <StepAdmin v-if="current === 0" @done="advance" />
        <StepRegistries v-else-if="current === 1" @next="advance" @back="back" />
        <StepInstance v-else @finish="finish" @back="back" />
      </div>
    </main>
  </div>
</template>
```

- [ ] **Step 7: Commit (step components stubbed next)**

Create one-line stubs for `StepAdmin.vue`, `StepRegistries.vue`, `StepInstance.vue` (`<template><div>step</div></template>` with the emit declarations) so the build passes, then:

```bash
git add web/src/components/setup/ web/src/views/SetupWizard.vue
git commit -m "feat(web): wizard shell + sidebar + step model"
```

### Task 17: Step 1 — create admin

**Files:**
- Modify: `web/src/components/setup/StepAdmin.vue`

- [ ] **Step 1: Implement `StepAdmin.vue`**

```vue
<script setup lang="ts">
import { ref } from 'vue'
import Button from '@/components/ui/Button.vue'
import Input from '@/components/ui/Input.vue'
import { apiFetch, ApiError } from '@/lib/api'
import { useAuthStore } from '@/stores/auth'

const emit = defineEmits<{ done: [] }>()
const auth = useAuthStore()
const username = ref('')
const email = ref('')
const password = ref('')
const error = ref('')
const loading = ref(false)

async function submit() {
  error.value = ''
  loading.value = true
  try {
    const admin = await apiFetch<{ username: string; email: string; is_admin: boolean; id: string }>(
      '/api/v1/auth/setup',
      { method: 'POST', body: JSON.stringify({ username: username.value, email: email.value, password: password.value }) },
    )
    // setup sets the cookie; reflect the admin in the store without another round-trip
    auth.user = { ...admin }
    emit('done')
  } catch (e) {
    error.value = e instanceof ApiError && e.status === 409
      ? 'Setup is already complete.'
      : 'Could not create admin. Check your details.'
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div>
    <h1 class="text-2xl font-bold tracking-tight text-foreground">Create admin account</h1>
    <p class="mt-1 text-sm text-muted-foreground">This is the first account and has full admin rights.</p>
    <form class="mt-6 flex flex-col gap-4" @submit.prevent="submit">
      <label class="text-sm font-medium">Username<Input v-model="username" class="mt-1" required /></label>
      <label class="text-sm font-medium">Email<Input v-model="email" type="email" class="mt-1" required /></label>
      <label class="text-sm font-medium">Password<Input v-model="password" type="password" class="mt-1" required /></label>
      <p v-if="error" class="text-sm text-destructive">{{ error }}</p>
      <Button type="submit" :disabled="loading">{{ loading ? 'Creating…' : 'Create & continue' }}</Button>
    </form>
  </div>
</template>
```

- [ ] **Step 2: Build + commit**

Run: `cd web && pnpm build`
Expected: clean build.

```bash
git add web/src/components/setup/StepAdmin.vue
git commit -m "feat(web): setup wizard step 1 — create admin"
```

### Task 18: Step 2 — enable registries

**Files:**
- Modify: `web/src/components/setup/StepRegistries.vue`

- [ ] **Step 1: Implement `StepRegistries.vue`**

Reads current settings from `GET /api/v1/admin/registries/` and toggles via the existing `PUT /api/v1/admin/registries/:registry/enable|disable` (admin-guarded; the just-created admin's cookie authorizes it):

```vue
<script setup lang="ts">
import { ref, onMounted } from 'vue'
import Button from '@/components/ui/Button.vue'
import Switch from '@/components/ui/Switch.vue'
import { apiFetch } from '@/lib/api'

const emit = defineEmits<{ next: []; back: [] }>()

interface Setting { registry_name: string; enabled: boolean }
const settings = ref<Setting[]>([])
const loading = ref(true)

onMounted(async () => {
  try {
    const res = await apiFetch<{ data: Setting[] }>('/api/v1/admin/registries/')
    settings.value = res.data
  } finally {
    loading.value = false
  }
})

async function toggle(s: Setting) {
  const action = s.enabled ? 'disable' : 'enable'
  await apiFetch(`/api/v1/admin/registries/${s.registry_name}/${action}`, { method: 'PUT' })
  s.enabled = !s.enabled
}
</script>

<template>
  <div>
    <h1 class="text-2xl font-bold tracking-tight text-foreground">Enable registries</h1>
    <p class="mt-1 text-sm text-muted-foreground">Turn on the package formats you plan to use. You can change this later in Admin.</p>
    <div v-if="loading" class="mt-6 text-sm text-muted-foreground">Loading…</div>
    <ul v-else class="mt-6 flex flex-col divide-y divide-border rounded border border-border">
      <li v-for="s in settings" :key="s.registry_name" class="flex items-center justify-between px-4 py-3">
        <span class="font-mono text-sm uppercase">{{ s.registry_name }}</span>
        <Switch :model-value="s.enabled" @update:model-value="toggle(s)" />
      </li>
    </ul>
    <div class="mt-6 flex justify-between">
      <Button variant="secondary" @click="emit('back')">Back</Button>
      <Button @click="emit('next')">Continue</Button>
    </div>
  </div>
</template>
```

> Confirm the GET response shape: `admin.go`'s `getRegistrySettings` returns `types.APIResponse{ data: [...] }` and `types.RegistrySetting` exposes `registry_name` + `enabled` JSON fields. Adjust property names to match `pkg/types` if they differ.

- [ ] **Step 2: Build + commit**

Run: `cd web && pnpm build`

```bash
git add web/src/components/setup/StepRegistries.vue
git commit -m "feat(web): setup wizard step 2 — enable registries"
```

### Task 19: Step 3 — instance info + finish

**Files:**
- Modify: `web/src/components/setup/StepInstance.vue`

- [ ] **Step 1: Implement `StepInstance.vue`**

Storage/base-URL are env-configured, so this step is informational/confirmation and pulls the live version from `/health` (reuse the existing `useHealth` composable):

```vue
<script setup lang="ts">
import Button from '@/components/ui/Button.vue'
import { useHealth } from '@/composables/useHealth'

const emit = defineEmits<{ finish: []; back: [] }>()
const { data: health } = useHealth()
</script>

<template>
  <div>
    <h1 class="text-2xl font-bold tracking-tight text-foreground">Instance ready</h1>
    <p class="mt-1 text-sm text-muted-foreground">
      Storage and base URL are configured via environment variables on the server. Review and finish.
    </p>
    <dl class="mt-6 flex flex-col gap-3 rounded border border-border p-4 text-sm">
      <div class="flex justify-between"><dt class="text-muted-foreground">Service</dt><dd class="font-mono">{{ health?.service ?? '—' }}</dd></div>
      <div class="flex justify-between"><dt class="text-muted-foreground">Version</dt><dd class="font-mono">{{ health?.version ?? '—' }}</dd></div>
      <div class="flex justify-between"><dt class="text-muted-foreground">Status</dt><dd class="font-mono">{{ health?.status ?? '—' }}</dd></div>
    </dl>
    <div class="mt-6 flex justify-between">
      <Button variant="secondary" @click="emit('back')">Back</Button>
      <Button @click="emit('finish')">Finish setup</Button>
    </div>
  </div>
</template>
```

- [ ] **Step 2: Build + full web tests**

Run: `cd web && pnpm build && pnpm test`
Expected: clean build, tests pass.

- [ ] **Step 3: Commit + PR**

```bash
git add web/src/components/setup/StepInstance.vue
git commit -m "feat(web): setup wizard step 3 — instance review + finish"
git push -u origin feat/web-setup-wizard
gh pr create --title "feat(web): first-run setup wizard" --body "Phase 4: 3-step sidebar wizard (admin → registries → instance) wired to /auth/setup and the admin registry-settings endpoints."
```

---

## Phase 5 — Personalized dashboard (PR: `feat/web-dashboard`)

Replaces the `DashboardView.vue` stub with the stat-row + 2-col grid layout.

Files in play:
- Modify: `web/src/views/DashboardView.vue`
- Create: `web/src/composables/useDashboard.ts`
- Create: `web/src/components/dashboard/MyArtifacts.vue`, `MyApiKeys.vue`, `RecentActivity.vue`
- Create: `web/src/composables/__tests__/useDashboard.spec.ts`

### Task 20: Dashboard data composables

**Files:**
- Create: `web/src/composables/useDashboard.ts`
- Test: `web/src/composables/__tests__/useDashboard.spec.ts`

- [ ] **Step 1: Write the failing test (mock activity generator is pure + testable)**

Create `web/src/composables/__tests__/useDashboard.spec.ts`:

```ts
import { describe, it, expect } from 'vitest'
import { mockRecentActivity } from '@/composables/useDashboard'

describe('mockRecentActivity', () => {
  it('returns a non-empty, shaped, deterministic list', () => {
    const a = mockRecentActivity()
    expect(a.length).toBeGreaterThan(0)
    expect(a[0]).toHaveProperty('artifact')
    expect(a[0]).toHaveProperty('action')
    expect(a[0]).toHaveProperty('when')
    expect(mockRecentActivity()).toEqual(a) // deterministic
  })
})
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd web && pnpm test src/composables/__tests__/useDashboard.spec.ts`
Expected: FAIL — module not found.

- [ ] **Step 3: Implement `web/src/composables/useDashboard.ts`**

```ts
import { useQuery } from '@tanstack/vue-query'
import { apiFetch } from '@/lib/api'

export interface ApiKey { id: string; name: string; created_at: string; last_used_at: string | null }
export interface ActivityItem { artifact: string; action: 'pull' | 'publish'; when: string }

export function useApiKeys() {
  return useQuery({
    queryKey: ['api-keys'],
    queryFn: () => apiFetch<{ api_keys: ApiKey[] }>('/api/v1/auth/api-keys').then((r) => r.api_keys),
  })
}

// TODO(activity): replace with GET /api/v1/me/activity once the endpoint exists.
// Mocked for v1 so the dashboard ships without an analytics subsystem.
export function mockRecentActivity(): ActivityItem[] {
  return [
    { artifact: 'lodestone-core-runtime', action: 'pull', when: '2h ago' },
    { artifact: 'lodestone-ui-kit', action: 'publish', when: 'Yesterday' },
    { artifact: 'artifact-scanner-py', action: 'pull', when: '3 days ago' },
  ]
}
```

> "My artifacts" reuses the existing package-ownership endpoint. Confirm its path under `cmd/api-gateway/routes/` (PackageOwnershipRoutes) and add a `useMyArtifacts` query mirroring `useApiKeys`. If no per-user listing endpoint exists, list the user's owned packages via the ownership route and note any gap in the PR description.

- [ ] **Step 4: Run test to verify it passes**

Run: `cd web && pnpm test src/composables/__tests__/useDashboard.spec.ts`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add web/src/composables/useDashboard.ts web/src/composables/__tests__/useDashboard.spec.ts
git commit -m "feat(web): dashboard data composables (api keys + mock activity)"
```

### Task 21: Dashboard cards

**Files:**
- Create: `web/src/components/dashboard/MyApiKeys.vue`, `RecentActivity.vue`, `MyArtifacts.vue`

- [ ] **Step 1: Implement `MyApiKeys.vue`**

```vue
<script setup lang="ts">
import Card from '@/components/ui/Card.vue'
import { useApiKeys } from '@/composables/useDashboard'
const { data: keys, isLoading } = useApiKeys()
</script>

<template>
  <Card class="p-5">
    <h3 class="text-sm font-semibold text-foreground">API Keys</h3>
    <p v-if="isLoading" class="mt-3 text-sm text-muted-foreground">Loading…</p>
    <ul v-else-if="keys?.length" class="mt-3 flex flex-col divide-y divide-border">
      <li v-for="k in keys" :key="k.id" class="flex items-center justify-between py-2 text-sm">
        <span class="font-medium">{{ k.name }}</span>
        <span class="font-mono text-xs text-muted-foreground">{{ k.last_used_at ?? 'never used' }}</span>
      </li>
    </ul>
    <p v-else class="mt-3 text-sm text-muted-foreground">No API keys yet.</p>
  </Card>
</template>
```

- [ ] **Step 2: Implement `RecentActivity.vue` (mock + badge)**

```vue
<script setup lang="ts">
import Card from '@/components/ui/Card.vue'
import Badge from '@/components/ui/Badge.vue'
import { mockRecentActivity } from '@/composables/useDashboard'
const items = mockRecentActivity()
</script>

<template>
  <Card class="p-5">
    <div class="flex items-center justify-between">
      <h3 class="text-sm font-semibold text-foreground">Recent activity</h3>
      <Badge tone="neutral">mock</Badge>
    </div>
    <ul class="mt-3 flex flex-col divide-y divide-border">
      <li v-for="(it, i) in items" :key="i" class="flex items-center justify-between py-2 text-sm">
        <span class="font-mono text-xs">{{ it.artifact }}</span>
        <span class="text-muted-foreground">{{ it.action }} · {{ it.when }}</span>
      </li>
    </ul>
  </Card>
</template>
```

> Confirm `Badge.vue` accepts a `tone` prop with a `neutral` value (it does, from the earlier UI work). If the prop name differs, match it.

- [ ] **Step 3: Implement `MyArtifacts.vue`**

```vue
<script setup lang="ts">
import Card from '@/components/ui/Card.vue'
import { artifacts } from '@/data/mock'
// TODO(my-artifacts): swap mock for useMyArtifacts() once the per-user listing query is wired.
</script>

<template>
  <Card class="p-5">
    <h3 class="text-sm font-semibold text-foreground">My Artifacts</h3>
    <ul class="mt-3 flex flex-col divide-y divide-border">
      <li v-for="a in artifacts.slice(0, 5)" :key="a.name" class="flex items-center justify-between py-2 text-sm">
        <span class="font-medium">{{ a.name }}</span>
        <span class="font-mono text-xs text-muted-foreground">{{ a.version }}</span>
      </li>
    </ul>
  </Card>
</template>
```

- [ ] **Step 4: Build + commit**

Run: `cd web && pnpm build`

```bash
git add web/src/components/dashboard/
git commit -m "feat(web): dashboard cards (artifacts, api keys, activity)"
```

### Task 22: Dashboard view layout + nav

**Files:**
- Modify: `web/src/views/DashboardView.vue`
- Modify: `web/src/components/AppTopbar.vue` (wire logout) — only if a user menu exists; otherwise add a logout button.

- [ ] **Step 1: Implement `web/src/views/DashboardView.vue` (stat row + 2-col grid)**

```vue
<script setup lang="ts">
import { computed } from 'vue'
import StatCard from '@/components/StatCard.vue'
import MyArtifacts from '@/components/dashboard/MyArtifacts.vue'
import MyApiKeys from '@/components/dashboard/MyApiKeys.vue'
import RecentActivity from '@/components/dashboard/RecentActivity.vue'
import Button from '@/components/ui/Button.vue'
import { useAuthStore } from '@/stores/auth'
import { useApiKeys } from '@/composables/useDashboard'

const auth = useAuthStore()
const { data: keys } = useApiKeys()
const keyCount = computed(() => keys.value?.length ?? 0)
</script>

<template>
  <div class="mx-auto max-w-[1440px] px-8 py-7">
    <div class="flex items-center justify-between">
      <h1 class="text-3xl font-bold tracking-tight text-foreground">
        Welcome back, {{ auth.user?.username ?? 'there' }}
      </h1>
      <div class="flex gap-2">
        <Button>Publish artifact</Button>
        <Button variant="secondary">New API key</Button>
      </div>
    </div>

    <div class="mt-6 grid grid-cols-1 gap-4 sm:grid-cols-3">
      <StatCard label="My Artifacts" :value="'12'" />
      <StatCard label="API Keys" :value="String(keyCount)" />
      <StatCard label="My Pulls (24h)" :value="'1.2k'" />
    </div>

    <div class="mt-5 grid grid-cols-1 gap-5 lg:grid-cols-2">
      <MyArtifacts />
      <div class="flex flex-col gap-5">
        <MyApiKeys />
        <RecentActivity />
      </div>
    </div>
  </div>
</template>
```

> Confirm `StatCard.vue`'s prop names (`label`, `value`). It was built with `v-bind="s"` over `{ label, value, ... }` objects in `data/mock.ts`; pass the same prop names. Adjust if it expects different keys.

- [ ] **Step 2: Wire logout**

In `web/src/components/AppTopbar.vue`, add a logout action:

```ts
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
const router = useRouter()
const auth = useAuthStore()
async function logout() { await auth.logout(); router.push('/login') }
```

Add a button/menu item bound to `@click="logout"`.

- [ ] **Step 3: Build + full web tests**

Run: `cd web && pnpm build && pnpm test`
Expected: clean build, all tests pass.

- [ ] **Step 4: Manual verification (golden path)**

Run the backend (with a fresh DB) and `cd web && pnpm dev`. Verify:
- Empty DB → app redirects to `/setup`; complete the 3 steps → lands on `/dashboard`.
- Logout → `/login`; login → `/dashboard`.
- Dashboard shows username, API key count, and the mock activity card with its badge.

If the UI can't be exercised (no DB), say so explicitly rather than claiming success.

- [ ] **Step 5: Commit + PR**

```bash
git add web/src/views/DashboardView.vue web/src/components/AppTopbar.vue
git commit -m "feat(web): personalized dashboard + logout"
git push -u origin feat/web-dashboard
gh pr create --title "feat(web): personalized dashboard" --body "Phase 5: stat-row + 2-col dashboard (my artifacts, api keys, mock activity) and logout wiring."
```

---

## Self-Review

**Spec coverage:**
- Cookie auth (set/read/precedence) → Tasks 1–2. ✓
- Logout → Task 3. ✓
- `/auth/me` → Task 4. ✓
- `setup-status` / `setup` + first-admin guard → Tasks 5–8. ✓
- Wizard reuses existing admin registry-settings endpoints → noted after Task 8; Task 18. ✓
- Frontend auth store (user-only, /auth/me boot, no token in JS) → Task 11. ✓
- Router guards (setup redirect + auth) → Task 12. ✓
- Login (split brand) + register → Tasks 13–15. ✓
- Setup wizard (sidebar steps, 3 steps) → Tasks 16–19. ✓
- Dashboard (stat row + 2-col grid, mock activity + badge) → Tasks 20–22. ✓
- CSRF via SameSite=Strict; no refresh token → reflected in cookie attributes (Tasks 1, 8); no token storage in JS (Task 11). ✓

**Out-of-scope honored:** No SSO (disabled button, Task 14). Activity mocked + TODO (Tasks 20–21). No `/me/activity` endpoint built.

**Known confirmations to make during execution (flagged inline, not placeholders):**
- `mockAuthService` field names in `middleware/auth_test.go` (Task 2).
- `setupTestService` helper name in `internal/auth/service_test.go` (Tasks 5–6).
- `types.RegistrySetting` JSON field names (Task 18).
- `Input.vue` `type` prop, `Badge.vue` `tone` prop, `StatCard.vue` prop names (Tasks 14, 21, 22).
- Package-ownership per-user listing endpoint path (Task 20).

**Type consistency:** `User` shape (`id/username/email/is_admin`) consistent across `/auth/me` (Task 4), store (Task 11), StepAdmin (Task 17). Cookie name `lodestone_token` consistent across Tasks 1, 2, 3, 8. `apiFetch`/`ApiError` signatures consistent across all frontend tasks.
