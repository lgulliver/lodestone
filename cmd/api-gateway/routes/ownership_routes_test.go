package routes

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lgulliver/lodestone/pkg/types"
)

func registerUser(t *testing.T, h *testHarness, username, email string) *types.User {
	t.Helper()
	u, err := h.auth.Register(context.Background(), &types.RegisterRequest{
		Username: username, Email: email, Password: "testpassword123",
	})
	require.NoError(t, err)
	return u
}

func TestOwnershipGetAddRemove(t *testing.T) {
	h := newHarness(t)
	PackageOwnershipRoutes(h.api, h.registry, h.auth)
	// seedArtifact makes h.user the initial owner of npm/mypkg.
	h.seedArtifact(t, "npm", "mypkg", "1.0.0", []byte("x"), nil)
	second := registerUser(t, h, "second", "second@example.com")

	// Get owners.
	w := h.do(http.MethodGet, "/api/packages/npm/mypkg/owners", nil, "")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), h.user.ID.String())

	// Add second user as owner (so there are 2 owners and removal is permitted).
	w = h.do(http.MethodPost, "/api/packages/npm/mypkg/owners",
		[]byte(`{"user_id":"`+second.ID.String()+`","role":"owner"}`), "application/json")
	assert.Equal(t, http.StatusCreated, w.Code)

	// Remove second user.
	w = h.do(http.MethodDelete, "/api/packages/npm/mypkg/owners/"+second.ID.String(), nil, "")
	assert.Equal(t, http.StatusOK, w.Code)
}

// Removing a maintainer from a single-owner package must succeed: the last-owner
// guard only applies when the target being removed is itself an owner.
func TestOwnershipRemoveMaintainer_Allowed(t *testing.T) {
	h := newHarness(t)
	PackageOwnershipRoutes(h.api, h.registry, h.auth)
	h.seedArtifact(t, "npm", "mypkg", "1.0.0", []byte("x"), nil)
	second := registerUser(t, h, "second", "second@example.com")

	w := h.do(http.MethodPost, "/api/packages/npm/mypkg/owners",
		[]byte(`{"user_id":"`+second.ID.String()+`","role":"maintainer"}`), "application/json")
	assert.Equal(t, http.StatusCreated, w.Code)

	w = h.do(http.MethodDelete, "/api/packages/npm/mypkg/owners/"+second.ID.String(), nil, "")
	assert.Equal(t, http.StatusOK, w.Code)
}

// The last *owner* still cannot be removed.
func TestOwnershipRemoveLastOwner_Blocked(t *testing.T) {
	h := newHarness(t)
	PackageOwnershipRoutes(h.api, h.registry, h.auth)
	h.seedArtifact(t, "npm", "mypkg", "1.0.0", []byte("x"), nil)

	w := h.do(http.MethodDelete, "/api/packages/npm/mypkg/owners/"+h.user.ID.String(), nil, "")
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "cannot remove the last owner")
}

func TestOwnershipAdd_BadInput(t *testing.T) {
	h := newHarness(t)
	PackageOwnershipRoutes(h.api, h.registry, h.auth)
	h.seedArtifact(t, "npm", "mypkg", "1.0.0", []byte("x"), nil)
	second := registerUser(t, h, "second", "second@example.com")

	// Malformed body.
	w := h.do(http.MethodPost, "/api/packages/npm/mypkg/owners", []byte("{"), "application/json")
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// Invalid role.
	w = h.do(http.MethodPost, "/api/packages/npm/mypkg/owners",
		[]byte(`{"user_id":"`+second.ID.String()+`","role":"god"}`), "application/json")
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// Invalid target user id on remove.
	w = h.do(http.MethodDelete, "/api/packages/npm/mypkg/owners/not-a-uuid", nil, "")
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestOwnershipMyPackages(t *testing.T) {
	h := newHarness(t)
	PackageOwnershipRoutes(h.api, h.registry, h.auth)
	h.seedArtifact(t, "npm", "mypkg", "1.0.0", []byte("x"), nil)

	w := h.do(http.MethodGet, "/api/packages/my-packages", nil, "")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "mypkg")

	// Pagination params accepted.
	w = h.do(http.MethodGet, "/api/packages/my-packages?page=1&per_page=10", nil, "")
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestOwnershipUnauthorized(t *testing.T) {
	h := newHarness(t)
	PackageOwnershipRoutes(h.api, h.registry, h.auth)

	w := h.doNoAuth(http.MethodGet, "/api/packages/my-packages")
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
