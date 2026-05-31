package routes

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuthRegister(t *testing.T) {
	h := newHarness(t)
	AuthRoutes(h.api, h.auth)

	// New user.
	w := h.do(http.MethodPost, "/api/auth/register",
		[]byte(`{"username":"newuser","email":"new@example.com","password":"password12345"}`),
		"application/json")
	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Contains(t, w.Body.String(), "newuser")

	// Duplicate username (harness already registered "routeuser").
	w = h.do(http.MethodPost, "/api/auth/register",
		[]byte(`{"username":"routeuser","email":"dup@example.com","password":"password12345"}`),
		"application/json")
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// Malformed body.
	w = h.do(http.MethodPost, "/api/auth/register", []byte("not-json"), "application/json")
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAuthLogin(t *testing.T) {
	h := newHarness(t)
	AuthRoutes(h.api, h.auth)

	// Valid creds (seeded by harness).
	w := h.do(http.MethodPost, "/api/auth/login",
		[]byte(`{"username":"routeuser","password":"testpassword123"}`), "application/json")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "token")

	// Wrong password.
	w = h.do(http.MethodPost, "/api/auth/login",
		[]byte(`{"username":"routeuser","password":"wrong"}`), "application/json")
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	// Malformed body.
	w = h.do(http.MethodPost, "/api/auth/login", []byte("{"), "application/json")
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAuthAPIKeyCreateListRevoke(t *testing.T) {
	h := newHarness(t)
	AuthRoutes(h.api, h.auth)

	// Create.
	w := h.do(http.MethodPost, "/api/auth/api-keys",
		[]byte(`{"name":"ci-key","permissions":["read"]}`), "application/json")
	require.Equal(t, http.StatusCreated, w.Code)

	var created struct {
		APIKey struct {
			ID string `json:"id"`
		} `json:"api_key"`
		Key string `json:"key"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))
	require.NotEmpty(t, created.APIKey.ID)

	// List.
	w = h.do(http.MethodGet, "/api/auth/api-keys", nil, "")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "ci-key")

	// Revoke.
	w = h.do(http.MethodDelete, "/api/auth/api-keys/"+created.APIKey.ID, nil, "")
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAuthAPIKey_BadInput(t *testing.T) {
	h := newHarness(t)
	AuthRoutes(h.api, h.auth)

	// Missing required name.
	w := h.do(http.MethodPost, "/api/auth/api-keys", []byte(`{}`), "application/json")
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// Revoke with non-UUID id.
	w = h.do(http.MethodDelete, "/api/auth/api-keys/not-a-uuid", nil, "")
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAuthProtected_Unauthorized(t *testing.T) {
	h := newHarness(t)
	AuthRoutes(h.api, h.auth)

	w := h.doNoAuth(http.MethodGet, "/api/auth/api-keys")
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
