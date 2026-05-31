package routes

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lgulliver/lodestone/pkg/types"
)

// makeAdmin flips the harness user to admin. AuthMiddleware reloads the user from
// the DB on every request, so the existing token keeps working.
func makeAdmin(t *testing.T, h *testHarness) {
	t.Helper()
	require.NoError(t, h.db.DB.Model(&types.User{}).
		Where("id = ?", h.user.ID).Update("is_admin", true).Error)
}

func TestAdminRegistrySettings(t *testing.T) {
	h := newHarness(t)
	makeAdmin(t, h)
	AdminRoutes(h.api, h.registry, h.auth)

	// List all.
	w := h.do(http.MethodGet, "/api/admin/registries/", nil, "")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "npm")

	// Get one.
	w = h.do(http.MethodGet, "/api/admin/registries/npm", nil, "")
	assert.Equal(t, http.StatusOK, w.Code)

	// Unknown registry.
	w = h.do(http.MethodGet, "/api/admin/registries/ghost", nil, "")
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestAdminEnableDisableDescribe(t *testing.T) {
	h := newHarness(t)
	makeAdmin(t, h)
	AdminRoutes(h.api, h.registry, h.auth)

	w := h.do(http.MethodPut, "/api/admin/registries/npm/disable", nil, "")
	assert.Equal(t, http.StatusOK, w.Code)

	w = h.do(http.MethodPut, "/api/admin/registries/npm/enable", nil, "")
	assert.Equal(t, http.StatusOK, w.Code)

	w = h.do(http.MethodPut, "/api/admin/registries/npm/description",
		[]byte(`{"description":"node packages"}`), "application/json")
	assert.Equal(t, http.StatusOK, w.Code)

	// Missing required description.
	w = h.do(http.MethodPut, "/api/admin/registries/npm/description",
		[]byte(`{}`), "application/json")
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAdminForbiddenForNonAdmin(t *testing.T) {
	h := newHarness(t)
	AdminRoutes(h.api, h.registry, h.auth)

	w := h.do(http.MethodGet, "/api/admin/registries/", nil, "")
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestAdminUnauthorized(t *testing.T) {
	h := newHarness(t)
	AdminRoutes(h.api, h.registry, h.auth)

	w := h.doNoAuth(http.MethodGet, "/api/admin/registries/")
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
