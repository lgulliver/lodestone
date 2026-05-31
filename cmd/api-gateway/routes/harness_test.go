package routes

import (
	"bytes"
	"context"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/lgulliver/lodestone/internal/auth"
	"github.com/lgulliver/lodestone/internal/common"
	"github.com/lgulliver/lodestone/internal/registry"
	"github.com/lgulliver/lodestone/internal/storage"
	"github.com/lgulliver/lodestone/pkg/config"
	"github.com/lgulliver/lodestone/pkg/types"
)

// testHarness wires real auth + registry services over sqlite and local storage so
// route handlers can be exercised end-to-end through gin.
type testHarness struct {
	router   *gin.Engine
	api      *gin.RouterGroup
	registry *registry.Service
	auth     *auth.Service
	db       *common.Database
	store    storage.BlobStorage
	user     *types.User
	token    string
}

// seedArtifact inserts an artifact row and stores its content so Download works.
func (h *testHarness) seedArtifact(t *testing.T, reg, name, version string, content []byte, metadata types.JSONMap) {
	t.Helper()
	path := reg + "/" + name + "/" + version + "/" + name
	require.NoError(t, h.store.Store(context.Background(), path, bytes.NewReader(content), "application/octet-stream"))
	art := &types.Artifact{
		Name: name, Version: version, Registry: reg,
		StoragePath: path, Size: int64(len(content)),
		PublishedBy: h.user.ID, Metadata: metadata,
	}
	require.NoError(t, h.db.Create(art).Error)
	require.NoError(t, h.registry.Ownership.EstablishInitialOwnership(
		context.Background(), reg, name, h.user.ID))
}

func newHarness(t *testing.T) *testHarness {
	t.Helper()
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&types.User{}, &types.APIKey{}, &types.Artifact{},
		&types.PackageOwnership{}, &types.RegistrySetting{},
	))

	for _, name := range []string{"npm", "nuget", "maven", "go", "helm", "oci", "opa", "cargo", "rubygems"} {
		require.NoError(t, db.Create(&types.RegistrySetting{RegistryName: name, Enabled: true}).Error)
	}

	cdb := &common.Database{DB: db}

	store, err := storage.NewLocalStorage(t.TempDir())
	require.NoError(t, err)

	regSvc := registry.NewService(cdb, store, nil)
	authSvc := auth.NewService(cdb, nil, &config.AuthConfig{
		JWTSecret:     "test-secret-key-for-routes",
		JWTExpiration: time.Hour,
		BCryptCost:    4,
	})

	ctx := context.Background()
	user, err := authSvc.Register(ctx, &types.RegisterRequest{
		Username: "routeuser", Email: "route@example.com", Password: "testpassword123",
	})
	require.NoError(t, err)

	authToken, err := authSvc.Login(ctx, &types.LoginRequest{
		Username: "routeuser", Password: "testpassword123",
	})
	require.NoError(t, err)

	router := gin.New()
	api := router.Group("/api")

	return &testHarness{
		router: router, api: api, registry: regSvc, auth: authSvc,
		db: cdb, store: store, user: user, token: authToken.Token,
	}
}

func (h *testHarness) do(method, path string, body []byte, contentType string) *httptest.ResponseRecorder {
	var r *http.Request
	if body != nil {
		r = httptest.NewRequest(method, path, bytes.NewReader(body))
	} else {
		r = httptest.NewRequest(method, path, nil)
	}
	r.Header.Set("Authorization", "Bearer "+h.token)
	if contentType != "" {
		r.Header.Set("Content-Type", contentType)
	}
	w := httptest.NewRecorder()
	h.router.ServeHTTP(w, r)
	return w
}

func (h *testHarness) doNoAuth(method, path string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, nil)
	w := httptest.NewRecorder()
	h.router.ServeHTTP(w, r)
	return w
}

// multipartFile builds a multipart body with a single file field.
func multipartFile(t *testing.T, field, filename string, data []byte) ([]byte, string) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile(field, filename)
	require.NoError(t, err)
	_, err = fw.Write(data)
	require.NoError(t, err)
	require.NoError(t, mw.Close())
	return buf.Bytes(), mw.FormDataContentType()
}
