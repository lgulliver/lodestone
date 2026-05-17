package upstream

import (
	"fmt"
	"net/url"
	"strings"
)

type npmAdapter struct{}
type nugetAdapter struct{}
type mavenAdapter struct{}
type goAdapter struct{}
type helmAdapter struct{}
type cargoAdapter struct{}
type rubyGemsAdapter struct{}
type opaAdapter struct{}
type ociAdapter struct{}

func (s *Service) registerAdapters() {
	adapters := []adapter{
		npmAdapter{},
		nugetAdapter{},
		mavenAdapter{},
		goAdapter{},
		helmAdapter{},
		cargoAdapter{},
		rubyGemsAdapter{},
		opaAdapter{},
		ociAdapter{},
	}
	for _, current := range adapters {
		s.adapters[current.Registry()] = current
	}
}

func (a npmAdapter) Registry() string { return "npm" }
func (a npmAdapter) BuildURL(upstream string, req ProxyRequest) (string, error) {
	trimmed := strings.TrimSuffix(upstream, "/")
	pkgPath := strings.ReplaceAll(req.Name, "/", "%2f")
	tarballName := req.Name
	if strings.Contains(tarballName, "/") {
		parts := strings.Split(tarballName, "/")
		tarballName = parts[len(parts)-1]
	}
	return fmt.Sprintf("%s/%s/-/%s-%s.tgz", trimmed, pkgPath, tarballName, req.Version), nil
}
func (a npmAdapter) AcceptHeader(req ProxyRequest) string { return "application/octet-stream" }

func (a nugetAdapter) Registry() string { return "nuget" }
func (a nugetAdapter) BuildURL(upstream string, req ProxyRequest) (string, error) {
	base := strings.TrimSuffix(upstream, "/")
	base = strings.TrimSuffix(base, "/v3/index.json")
	base = strings.TrimSuffix(base, "/index.json")
	packageID := strings.ToLower(req.Name)
	version := strings.ToLower(req.Version)
	return fmt.Sprintf("%s/v3-flatcontainer/%s/%s/%s.%s.nupkg", base, packageID, version, packageID, version), nil
}
func (a nugetAdapter) AcceptHeader(req ProxyRequest) string { return "application/octet-stream" }

func (a mavenAdapter) Registry() string { return "maven" }
func (a mavenAdapter) BuildURL(upstream string, req ProxyRequest) (string, error) {
	parts := strings.Split(req.Name, ":")
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid maven coordinates: %s", req.Name)
	}
	group := strings.ReplaceAll(parts[0], ".", "/")
	artifact := parts[1]
	base := strings.TrimSuffix(upstream, "/")
	return fmt.Sprintf("%s/%s/%s/%s/%s-%s.jar", base, group, artifact, req.Version, artifact, req.Version), nil
}
func (a mavenAdapter) AcceptHeader(req ProxyRequest) string { return "application/java-archive" }

func (a goAdapter) Registry() string { return "go" }
func (a goAdapter) BuildURL(upstream string, req ProxyRequest) (string, error) {
	base := strings.TrimSuffix(upstream, "/")
	escapedModule := url.PathEscape(req.Name)
	escapedModule = strings.ReplaceAll(escapedModule, "%2F", "/")
	return fmt.Sprintf("%s/%s/@v/%s.zip", base, escapedModule, req.Version), nil
}
func (a goAdapter) AcceptHeader(req ProxyRequest) string { return "application/zip" }

func (a helmAdapter) Registry() string { return "helm" }
func (a helmAdapter) BuildURL(upstream string, req ProxyRequest) (string, error) {
	base := strings.TrimSuffix(upstream, "/")
	return fmt.Sprintf("%s/%s-%s.tgz", base, req.Name, req.Version), nil
}
func (a helmAdapter) AcceptHeader(req ProxyRequest) string { return "application/gzip" }

func (a cargoAdapter) Registry() string { return "cargo" }
func (a cargoAdapter) BuildURL(upstream string, req ProxyRequest) (string, error) {
	base := strings.TrimSuffix(upstream, "/")
	return fmt.Sprintf("%s/api/v1/crates/%s/%s/download", base, req.Name, req.Version), nil
}
func (a cargoAdapter) AcceptHeader(req ProxyRequest) string { return "application/octet-stream" }

func (a rubyGemsAdapter) Registry() string { return "rubygems" }
func (a rubyGemsAdapter) BuildURL(upstream string, req ProxyRequest) (string, error) {
	base := strings.TrimSuffix(upstream, "/")
	return fmt.Sprintf("%s/downloads/%s-%s.gem", base, req.Name, req.Version), nil
}
func (a rubyGemsAdapter) AcceptHeader(req ProxyRequest) string { return "application/octet-stream" }

func (a opaAdapter) Registry() string { return "opa" }
func (a opaAdapter) BuildURL(upstream string, req ProxyRequest) (string, error) {
	base := strings.TrimSuffix(upstream, "/")
	return fmt.Sprintf("%s/bundles/%s/%s.tar.gz", base, req.Name, req.Version), nil
}
func (a opaAdapter) AcceptHeader(req ProxyRequest) string { return "application/gzip" }

func (a ociAdapter) Registry() string { return "oci" }
func (a ociAdapter) BuildURL(upstream string, req ProxyRequest) (string, error) {
	base := strings.TrimSuffix(upstream, "/")
	switch req.Resource {
	case "manifest":
		return fmt.Sprintf("%s/v2/%s/manifests/%s", base, req.Name, req.Version), nil
	case "blob":
		return fmt.Sprintf("%s/v2/%s/blobs/%s", base, req.Name, req.Version), nil
	default:
		return "", fmt.Errorf("unsupported oci upstream resource: %s", req.Resource)
	}
}
func (a ociAdapter) AcceptHeader(req ProxyRequest) string {
	if req.Resource == "manifest" {
		return strings.Join([]string{
			"application/vnd.oci.image.manifest.v1+json",
			"application/vnd.docker.distribution.manifest.v2+json",
			"application/vnd.docker.distribution.manifest.list.v2+json",
			"application/vnd.oci.image.index.v1+json",
		}, ", ")
	}
	return "application/octet-stream"
}
