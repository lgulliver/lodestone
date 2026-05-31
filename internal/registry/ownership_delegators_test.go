package registry

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServiceGetPackageOwners(t *testing.T) {
	s, db, _ := setupTestService(t)
	ctx := context.Background()
	owner := createTestUserWithAdmin(t, db, false)

	require.NoError(t, s.Ownership.EstablishInitialOwnership(ctx, "npm", "pkg", owner.ID))

	owners, err := s.GetPackageOwners(ctx, "npm", "pkg")
	require.NoError(t, err)
	require.Len(t, owners, 1)
	assert.Equal(t, owner.ID, owners[0].UserID)
}

func TestServiceAddPackageOwner(t *testing.T) {
	s, db, _ := setupTestService(t)
	ctx := context.Background()
	owner := createTestUserWithAdmin(t, db, false)
	target := createTestUserWithAdmin(t, db, false)

	require.NoError(t, s.Ownership.EstablishInitialOwnership(ctx, "npm", "pkg", owner.ID))

	// Owner can add another owner.
	require.NoError(t, s.AddPackageOwner(ctx, "npm", "pkg", owner.ID, target.ID, "owner"))

	owners, err := s.GetPackageOwners(ctx, "npm", "pkg")
	require.NoError(t, err)
	assert.Len(t, owners, 2)

	// Non-owner cannot add.
	stranger := createTestUserWithAdmin(t, db, false)
	err = s.AddPackageOwner(ctx, "npm", "pkg", stranger.ID, uuid.New(), "owner")
	assert.Error(t, err)
}

func TestServiceRemovePackageOwner(t *testing.T) {
	s, db, _ := setupTestService(t)
	ctx := context.Background()
	owner := createTestUserWithAdmin(t, db, false)
	target := createTestUserWithAdmin(t, db, false)

	require.NoError(t, s.Ownership.EstablishInitialOwnership(ctx, "npm", "pkg", owner.ID))
	require.NoError(t, s.AddPackageOwner(ctx, "npm", "pkg", owner.ID, target.ID, "owner"))

	// Owner removes the added owner.
	require.NoError(t, s.RemovePackageOwner(ctx, "npm", "pkg", owner.ID, target.ID))

	owners, err := s.GetPackageOwners(ctx, "npm", "pkg")
	require.NoError(t, err)
	assert.Len(t, owners, 1)

	// Non-owner cannot remove.
	stranger := createTestUserWithAdmin(t, db, false)
	err = s.RemovePackageOwner(ctx, "npm", "pkg", stranger.ID, owner.ID)
	assert.Error(t, err)
}
