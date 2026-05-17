package upstream

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/lgulliver/lodestone/pkg/config"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/singleflight"
)

var (
	ErrProxyDisabled       = errors.New("upstream proxy disabled")
	ErrUpstreamNotFound    = errors.New("upstream artifact not found")
	ErrUpstreamTooLarge    = errors.New("upstream artifact exceeds configured size limit")
	ErrUpstreamUnauthorized = errors.New("upstream request unauthorized")
)

type ProxyRequest struct {
	Registry string
	Resource string
	Name     string
	Version  string
}

type FetchedArtifact struct {
	Content     []byte
	ContentType string
	SourceURL   string
}

type adapter interface {
	Registry() string
	BuildURL(upstream string, req ProxyRequest) (string, error)
	AcceptHeader(req ProxyRequest) string
}

type Service struct {
	cfg      config.ProxyConfig
	client   *http.Client
	adapters map[string]adapter
	group    singleflight.Group
}

func NewService(cfg config.ProxyConfig) *Service {
	timeout := cfg.TimeoutSeconds
	if timeout <= 0 {
		timeout = 30
	}

	service := &Service{
		cfg: cfg,
		client: &http.Client{
			Timeout: time.Duration(timeout) * time.Second,
		},
		adapters: make(map[string]adapter),
	}
	service.registerAdapters()
	return service
}

func (s *Service) IsEnabled(registry string) bool {
	if !s.cfg.Enabled {
		return false
	}

	registryCfg, ok := s.registryConfig(registry)
	if !ok {
		return false
	}

	return registryCfg.Enabled && strings.TrimSpace(registryCfg.Upstream) != ""
}

func (s *Service) Fetch(ctx context.Context, req ProxyRequest) (*FetchedArtifact, error) {
	if !s.IsEnabled(req.Registry) {
		return nil, ErrProxyDisabled
	}

	adapter, ok := s.adapters[req.Registry]
	if !ok {
		return nil, fmt.Errorf("no upstream adapter for registry %s", req.Registry)
	}

	registryCfg, _ := s.registryConfig(req.Registry)
	key := fmt.Sprintf("%s|%s|%s|%s", req.Registry, req.Resource, req.Name, req.Version)

	result, err, _ := s.group.Do(key, func() (interface{}, error) {
		fetchCtx := ctx
		var cancel context.CancelFunc
		if s.cfg.TimeoutSeconds > 0 {
			fetchCtx, cancel = context.WithTimeout(ctx, time.Duration(s.cfg.TimeoutSeconds)*time.Second)
			defer cancel()
		}

		fetchURL, err := adapter.BuildURL(registryCfg.Upstream, req)
		if err != nil {
			return nil, err
		}

		if req.Registry == "oci" {
			return s.fetchOCI(fetchCtx, fetchURL, adapter.AcceptHeader(req))
		}
		return s.fetchURL(fetchCtx, fetchURL, adapter.AcceptHeader(req))
	})
	if err != nil {
		return nil, err
	}

	fetched, ok := result.(*FetchedArtifact)
	if !ok {
		return nil, fmt.Errorf("invalid upstream fetch result type")
	}
	return fetched, nil
}

func (s *Service) fetchOCI(ctx context.Context, fetchURL, accept string) (*FetchedArtifact, error) {
	result, statusCode, headers, err := s.doFetch(ctx, fetchURL, accept, "")
	if err == nil {
		return result, nil
	}

	if statusCode != http.StatusUnauthorized {
		return nil, err
	}

	wwwAuthenticate := headers.Get("WWW-Authenticate")
	token, tokenErr := s.getBearerToken(ctx, wwwAuthenticate)
	if tokenErr != nil {
		return nil, err
	}

	return s.fetchURLWithAuth(ctx, fetchURL, accept, "Bearer "+token)
}

func (s *Service) fetchURL(ctx context.Context, fetchURL, accept string) (*FetchedArtifact, error) {
	result, _, _, err := s.doFetch(ctx, fetchURL, accept, "")
	return result, err
}

func (s *Service) fetchURLWithAuth(ctx context.Context, fetchURL, accept, authz string) (*FetchedArtifact, error) {
	result, _, _, err := s.doFetch(ctx, fetchURL, accept, authz)
	return result, err
}

func (s *Service) doFetch(ctx context.Context, fetchURL, accept, authz string) (*FetchedArtifact, int, http.Header, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, fetchURL, nil)
	if err != nil {
		return nil, 0, nil, err
	}
	if accept != "" {
		request.Header.Set("Accept", accept)
	}
	if authz != "" {
		request.Header.Set("Authorization", authz)
	}

	response, err := s.client.Do(request)
	if err != nil {
		return nil, 0, nil, fmt.Errorf("failed upstream request: %w", err)
	}
	defer response.Body.Close()

	switch response.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		return nil, response.StatusCode, response.Header, ErrUpstreamNotFound
	case http.StatusUnauthorized, http.StatusForbidden:
		return nil, response.StatusCode, response.Header, ErrUpstreamUnauthorized
	default:
		return nil, response.StatusCode, response.Header, fmt.Errorf("upstream request failed with status %d", response.StatusCode)
	}

	content, err := s.readContentWithLimit(response.Body)
	if err != nil {
		return nil, response.StatusCode, response.Header, err
	}

	contentType := response.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	log.Debug().
		Str("url", fetchURL).
		Int("size", len(content)).
		Str("content_type", contentType).
		Msg("fetched artifact from upstream")

	return &FetchedArtifact{
		Content:     content,
		ContentType: contentType,
		SourceURL:   fetchURL,
	}, response.StatusCode, response.Header, nil
}

func (s *Service) readContentWithLimit(reader io.Reader) ([]byte, error) {
	if s.cfg.MaxArtifactBytes <= 0 {
		return io.ReadAll(reader)
	}

	limitedReader := io.LimitReader(reader, s.cfg.MaxArtifactBytes+1)
	data, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > s.cfg.MaxArtifactBytes {
		return nil, ErrUpstreamTooLarge
	}
	return data, nil
}

func (s *Service) getBearerToken(ctx context.Context, wwwAuthenticate string) (string, error) {
	if !strings.HasPrefix(wwwAuthenticate, "Bearer ") {
		return "", fmt.Errorf("unsupported auth challenge")
	}

	authValues := parseBearerChallenge(strings.TrimPrefix(wwwAuthenticate, "Bearer "))
	realm := authValues["realm"]
	if realm == "" {
		return "", fmt.Errorf("missing bearer realm")
	}

	query := url.Values{}
	if service := authValues["service"]; service != "" {
		query.Set("service", service)
	}
	if scope := authValues["scope"]; scope != "" {
		query.Set("scope", scope)
	}

	tokenURL := realm
	if encoded := query.Encode(); encoded != "" {
		tokenURL = tokenURL + "?" + encoded
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, tokenURL, nil)
	if err != nil {
		return "", err
	}

	response, err := s.client.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("token endpoint returned %d", response.StatusCode)
	}

	var payload struct {
		Token       string `json:"token"`
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return "", err
	}

	if payload.Token != "" {
		return payload.Token, nil
	}
	if payload.AccessToken != "" {
		return payload.AccessToken, nil
	}

	return "", fmt.Errorf("token not present in upstream auth response")
}

func parseBearerChallenge(raw string) map[string]string {
	result := make(map[string]string)
	parts := strings.Split(raw, ",")
	for _, part := range parts {
		piece := strings.TrimSpace(part)
		kv := strings.SplitN(piece, "=", 2)
		if len(kv) != 2 {
			continue
		}
		key := strings.TrimSpace(kv[0])
		value := strings.Trim(strings.TrimSpace(kv[1]), "\"")
		result[key] = value
	}
	return result
}

func (s *Service) registryConfig(registry string) (config.ProxyRegistryConfig, bool) {
	switch registry {
	case "npm":
		return s.cfg.Registries.NPM, true
	case "nuget":
		return s.cfg.Registries.NuGet, true
	case "maven":
		return s.cfg.Registries.Maven, true
	case "go":
		return s.cfg.Registries.Go, true
	case "helm":
		return s.cfg.Registries.Helm, true
	case "cargo":
		return s.cfg.Registries.Cargo, true
	case "rubygems":
		return s.cfg.Registries.RubyGems, true
	case "opa":
		return s.cfg.Registries.OPA, true
	case "oci":
		return s.cfg.Registries.OCI, true
	default:
		return config.ProxyRegistryConfig{}, false
	}
}
