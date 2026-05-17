package config

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestLoadFromEnv_StorageAliases(t *testing.T) {
	t.Setenv("STORAGE_TYPE", "s3")
	t.Setenv("S3_BUCKET", "legacy-bucket")
	t.Setenv("S3_REGION", "us-west-2")
	t.Setenv("S3_ACCESS_KEY", "legacy-access")
	t.Setenv("S3_SECRET_KEY", "legacy-secret")
	t.Setenv("S3_ENDPOINT", "http://localhost:4566")

	cfg := LoadFromEnv()

	assert.Equal(t, "s3", cfg.Storage.Type)
	assert.Equal(t, "legacy-bucket", cfg.Storage.S3.Bucket)
	assert.Equal(t, "us-west-2", cfg.Storage.S3.Region)
	assert.Equal(t, "legacy-access", cfg.Storage.S3.AccessKey)
	assert.Equal(t, "legacy-secret", cfg.Storage.S3.SecretKey)
	assert.Equal(t, "http://localhost:4566", cfg.Storage.S3.Endpoint)
}

func TestLoadFromEnv_AzureStorageConfig(t *testing.T) {
	t.Setenv("STORAGE_TYPE", "azure")
	t.Setenv("STORAGE_AZURE_ACCOUNT_NAME", "lodestoneaccount")
	t.Setenv("STORAGE_AZURE_ACCOUNT_KEY", "secret")
	t.Setenv("STORAGE_AZURE_CONTAINER", "lodestone-artifacts")
	t.Setenv("STORAGE_AZURE_ENDPOINT", "http://127.0.0.1:10000/devstoreaccount1")

	cfg := LoadFromEnv()

	assert.Equal(t, "azure", cfg.Storage.Type)
	assert.Equal(t, "lodestoneaccount", cfg.Storage.Azure.AccountName)
	assert.Equal(t, "secret", cfg.Storage.Azure.AccountKey)
	assert.Equal(t, "lodestone-artifacts", cfg.Storage.Azure.Container)
	assert.Equal(t, "http://127.0.0.1:10000/devstoreaccount1", cfg.Storage.Azure.Endpoint)
}

func TestLoadFromEnv_ProxyConfig(t *testing.T) {
	t.Setenv("PROXY_ENABLED", "true")
	t.Setenv("PROXY_TIMEOUT_SECONDS", "45")
	t.Setenv("PROXY_MAX_ARTIFACT_BYTES", "1048576")
	t.Setenv("PROXY_NPM_ENABLED", "true")
	t.Setenv("PROXY_NPM_UPSTREAM", "https://registry.npmjs.org")
	t.Setenv("PROXY_OCI_ENABLED", "true")
	t.Setenv("PROXY_OCI_UPSTREAM", "https://registry-1.docker.io")

	cfg := LoadFromEnv()

	assert.True(t, cfg.Proxy.Enabled)
	assert.Equal(t, 45, cfg.Proxy.TimeoutSeconds)
	assert.Equal(t, int64(1048576), cfg.Proxy.MaxArtifactBytes)
	assert.True(t, cfg.Proxy.Registries.NPM.Enabled)
	assert.Equal(t, "https://registry.npmjs.org", cfg.Proxy.Registries.NPM.Upstream)
	assert.True(t, cfg.Proxy.Registries.OCI.Enabled)
	assert.Equal(t, "https://registry-1.docker.io", cfg.Proxy.Registries.OCI.Upstream)
}

func TestLoadFromEnv_UIAuthAndCORSConfig(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://ui.example.com, http://localhost:5173")
	t.Setenv("UI_SESSION_COOKIE_NAME", "lodestone_session")
	t.Setenv("UI_CSRF_COOKIE_NAME", "lodestone_csrf")
	t.Setenv("UI_SESSION_EXPIRATION", "2h")
	t.Setenv("UI_COOKIE_SECURE", "false")
	t.Setenv("UI_COOKIE_SAME_SITE", "Strict")

	cfg := LoadFromEnv()

	assert.Equal(t, []string{"https://ui.example.com", "http://localhost:5173"}, cfg.Server.CORSAllowedOrigins)
	assert.Equal(t, "lodestone_session", cfg.Auth.UISessionCookieName)
	assert.Equal(t, "lodestone_csrf", cfg.Auth.UICSRFCookieName)
	assert.Equal(t, 2*time.Hour, cfg.Auth.UISessionExpiration)
	assert.False(t, cfg.Auth.UICookieSecure)
	assert.Equal(t, http.SameSiteStrictMode, cfg.Auth.UISameSite())
}
