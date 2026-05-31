package upstream

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lgulliver/lodestone/pkg/config"
)

func enabledCfg(registry, upstream string) config.ProxyConfig {
	cfg := config.ProxyConfig{Enabled: true, TimeoutSeconds: 5}
	rc := config.ProxyRegistryConfig{Enabled: true, Upstream: upstream}
	switch registry {
	case "npm":
		cfg.Registries.NPM = rc
	case "nuget":
		cfg.Registries.NuGet = rc
	case "maven":
		cfg.Registries.Maven = rc
	case "go":
		cfg.Registries.Go = rc
	case "helm":
		cfg.Registries.Helm = rc
	case "cargo":
		cfg.Registries.Cargo = rc
	case "rubygems":
		cfg.Registries.RubyGems = rc
	case "opa":
		cfg.Registries.OPA = rc
	case "oci":
		cfg.Registries.OCI = rc
	}
	return cfg
}

func TestIsEnabled(t *testing.T) {
	t.Run("disabled globally", func(t *testing.T) {
		s := NewService(config.ProxyConfig{Enabled: false})
		assert.False(t, s.IsEnabled("npm"))
	})
	t.Run("unknown registry", func(t *testing.T) {
		s := NewService(config.ProxyConfig{Enabled: true})
		assert.False(t, s.IsEnabled("bogus"))
	})
	t.Run("registry disabled", func(t *testing.T) {
		cfg := config.ProxyConfig{Enabled: true}
		cfg.Registries.NPM = config.ProxyRegistryConfig{Enabled: false, Upstream: "https://x"}
		assert.False(t, NewService(cfg).IsEnabled("npm"))
	})
	t.Run("empty upstream", func(t *testing.T) {
		cfg := config.ProxyConfig{Enabled: true}
		cfg.Registries.NPM = config.ProxyRegistryConfig{Enabled: true, Upstream: "  "}
		assert.False(t, NewService(cfg).IsEnabled("npm"))
	})
	t.Run("enabled", func(t *testing.T) {
		assert.True(t, NewService(enabledCfg("npm", "https://registry.npmjs.org")).IsEnabled("npm"))
	})
}

func TestNewServiceDefaultTimeout(t *testing.T) {
	s := NewService(config.ProxyConfig{TimeoutSeconds: 0})
	assert.NotNil(t, s.client)
}

func TestAdapterBuildURL(t *testing.T) {
	s := NewService(config.ProxyConfig{})
	tests := []struct {
		registry string
		req      ProxyRequest
		want     string
		wantErr  bool
	}{
		{"npm", ProxyRequest{Name: "lodash", Version: "4.17.21"}, "https://up/lodash/-/lodash-4.17.21.tgz", false},
		{"npm", ProxyRequest{Name: "@scope/pkg", Version: "1.0.0"}, "https://up/@scope%2fpkg/-/pkg-1.0.0.tgz", false},
		{"nuget", ProxyRequest{Name: "Newtonsoft.Json", Version: "13.0.1"}, "https://up/v3-flatcontainer/newtonsoft.json/13.0.1/newtonsoft.json.13.0.1.nupkg", false},
		{"maven", ProxyRequest{Name: "com.google.guava:guava", Version: "32.0"}, "https://up/com/google/guava/guava/32.0/guava-32.0.jar", false},
		{"maven", ProxyRequest{Name: "bad-coords", Version: "1.0"}, "", true},
		{"go", ProxyRequest{Name: "github.com/foo/bar", Version: "v1.0.0"}, "https://up/github.com/foo/bar/@v/v1.0.0.zip", false},
		{"helm", ProxyRequest{Name: "nginx", Version: "1.0.0"}, "https://up/nginx-1.0.0.tgz", false},
		{"cargo", ProxyRequest{Name: "serde", Version: "1.0.0"}, "https://up/api/v1/crates/serde/1.0.0/download", false},
		{"rubygems", ProxyRequest{Name: "rails", Version: "7.0.0"}, "https://up/downloads/rails-7.0.0.gem", false},
		{"opa", ProxyRequest{Name: "policy", Version: "1.0.0"}, "https://up/bundles/policy/1.0.0.tar.gz", false},
		{"oci", ProxyRequest{Name: "app", Version: "latest", Resource: "manifest"}, "https://up/v2/app/manifests/latest", false},
		{"oci", ProxyRequest{Name: "app", Version: "sha256:x", Resource: "blob"}, "https://up/v2/app/blobs/sha256:x", false},
		{"oci", ProxyRequest{Name: "app", Version: "latest", Resource: "bogus"}, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.registry+"/"+tt.req.Resource, func(t *testing.T) {
			a := s.adapters[tt.registry]
			require.NotNil(t, a)
			got, err := a.BuildURL("https://up/", tt.req)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestAdapterAcceptHeaders(t *testing.T) {
	s := NewService(config.ProxyConfig{})
	assert.Equal(t, "application/zip", s.adapters["go"].AcceptHeader(ProxyRequest{}))
	assert.Contains(t, s.adapters["oci"].AcceptHeader(ProxyRequest{Resource: "manifest"}), "manifest.v1+json")
	assert.Equal(t, "application/octet-stream", s.adapters["oci"].AcceptHeader(ProxyRequest{Resource: "blob"}))
}

func TestFetchDisabled(t *testing.T) {
	s := NewService(config.ProxyConfig{Enabled: false})
	_, err := s.Fetch(context.Background(), ProxyRequest{Registry: "npm"})
	assert.ErrorIs(t, err, ErrProxyDisabled)
}

func TestFetchHappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/gzip")
		_, _ = w.Write([]byte("payload"))
	}))
	defer srv.Close()

	s := NewService(enabledCfg("helm", srv.URL))
	got, err := s.Fetch(context.Background(), ProxyRequest{Registry: "helm", Name: "nginx", Version: "1.0.0"})
	require.NoError(t, err)
	assert.Equal(t, []byte("payload"), got.Content)
	assert.Equal(t, "application/gzip", got.ContentType)
}

func TestFetchNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	s := NewService(enabledCfg("helm", srv.URL))
	_, err := s.Fetch(context.Background(), ProxyRequest{Registry: "helm", Name: "x", Version: "1"})
	assert.ErrorIs(t, err, ErrUpstreamNotFound)
}

func TestFetchUnauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()
	s := NewService(enabledCfg("helm", srv.URL))
	_, err := s.Fetch(context.Background(), ProxyRequest{Registry: "helm", Name: "x", Version: "1"})
	assert.ErrorIs(t, err, ErrUpstreamUnauthorized)
}

func TestFetchUnexpectedStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	s := NewService(enabledCfg("helm", srv.URL))
	_, err := s.Fetch(context.Background(), ProxyRequest{Registry: "helm", Name: "x", Version: "1"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "status 500")
}

func TestFetchTooLarge(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("0123456789"))
	}))
	defer srv.Close()
	cfg := enabledCfg("helm", srv.URL)
	cfg.MaxArtifactBytes = 5
	s := NewService(cfg)
	_, err := s.Fetch(context.Background(), ProxyRequest{Registry: "helm", Name: "x", Version: "1"})
	assert.ErrorIs(t, err, ErrUpstreamTooLarge)
}

func TestFetchDefaultContentType(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header()["Content-Type"] = nil // suppress net/http content sniffing
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("x"))
	}))
	defer srv.Close()
	s := NewService(enabledCfg("helm", srv.URL))
	got, err := s.Fetch(context.Background(), ProxyRequest{Registry: "helm", Name: "x", Version: "1"})
	require.NoError(t, err)
	assert.Equal(t, "application/octet-stream", got.ContentType)
}

func TestFetchBuildURLError(t *testing.T) {
	s := NewService(enabledCfg("maven", "https://up"))
	_, err := s.Fetch(context.Background(), ProxyRequest{Registry: "maven", Name: "no-colon", Version: "1"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid maven coordinates")
}

func TestFetchOCIBearerFlow(t *testing.T) {
	var tokenSrv *httptest.Server
	tokenSrv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "myservice", r.URL.Query().Get("service"))
		_, _ = w.Write([]byte(`{"token":"abc123"}`))
	}))
	defer tokenSrv.Close()

	registrySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "Bearer abc123" {
			_, _ = w.Write([]byte("manifest-bytes"))
			return
		}
		w.Header().Set("WWW-Authenticate", `Bearer realm="`+tokenSrv.URL+`",service="myservice",scope="repository:app:pull"`)
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer registrySrv.Close()

	s := NewService(enabledCfg("oci", registrySrv.URL))
	got, err := s.Fetch(context.Background(), ProxyRequest{Registry: "oci", Name: "app", Version: "latest", Resource: "manifest"})
	require.NoError(t, err)
	assert.Equal(t, []byte("manifest-bytes"), got.Content)
}

func TestFetchOCIUnauthorizedNoChallenge(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()
	s := NewService(enabledCfg("oci", srv.URL))
	_, err := s.Fetch(context.Background(), ProxyRequest{Registry: "oci", Name: "app", Version: "latest", Resource: "manifest"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bearer token")
}

func TestParseBearerChallenge(t *testing.T) {
	got := parseBearerChallenge(`realm="https://auth.example.com/token",service="reg",scope="pull"`)
	assert.Equal(t, "https://auth.example.com/token", got["realm"])
	assert.Equal(t, "reg", got["service"])
	assert.Equal(t, "pull", got["scope"])
	// Malformed entries are skipped.
	got2 := parseBearerChallenge(`bad,realm="r"`)
	assert.Equal(t, "r", got2["realm"])
	assert.Len(t, got2, 1)
}

func TestGetBearerTokenErrors(t *testing.T) {
	s := NewService(config.ProxyConfig{})
	ctx := context.Background()

	_, err := s.getBearerToken(ctx, "Basic xyz")
	assert.Contains(t, err.Error(), "unsupported auth challenge")

	_, err = s.getBearerToken(ctx, "Bearer service=\"x\"")
	assert.Contains(t, err.Error(), "missing bearer realm")

	// Token endpoint returns non-200.
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer bad.Close()
	_, err = s.getBearerToken(ctx, `Bearer realm="`+bad.URL+`"`)
	assert.Contains(t, err.Error(), "token endpoint returned")

	// access_token fallback.
	at := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"access_token":"fallback"}`))
	}))
	defer at.Close()
	tok, err := s.getBearerToken(ctx, `Bearer realm="`+at.URL+`"`)
	require.NoError(t, err)
	assert.Equal(t, "fallback", tok)

	// Missing token in response.
	empty := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{}`))
	}))
	defer empty.Close()
	_, err = s.getBearerToken(ctx, `Bearer realm="`+empty.URL+`"`)
	assert.Contains(t, err.Error(), "token not present")
}

func TestRegistryConfigMapping(t *testing.T) {
	s := NewService(config.ProxyConfig{})
	for _, reg := range []string{"npm", "nuget", "maven", "go", "helm", "cargo", "rubygems", "opa", "oci"} {
		_, ok := s.registryConfig(reg)
		assert.True(t, ok, reg)
	}
	_, ok := s.registryConfig("unknown")
	assert.False(t, ok)
}

func TestReadContentWithLimitUnbounded(t *testing.T) {
	s := NewService(config.ProxyConfig{MaxArtifactBytes: 0})
	data, err := s.readContentWithLimit(strings.NewReader("hello"))
	require.NoError(t, err)
	assert.Equal(t, []byte("hello"), data)
}
