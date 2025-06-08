package registry

import (
	"github.com/lgulliver/lodestone/internal/registry/registries/cargo"
	goregistry "github.com/lgulliver/lodestone/internal/registry/registries/go"
	"github.com/lgulliver/lodestone/internal/registry/registries/helm"
	"github.com/lgulliver/lodestone/internal/registry/registries/maven"
	"github.com/lgulliver/lodestone/internal/registry/registries/npm"
	"github.com/lgulliver/lodestone/internal/registry/registries/nuget"
	"github.com/lgulliver/lodestone/internal/registry/registries/oci"
	"github.com/lgulliver/lodestone/internal/registry/registries/opa"
	"github.com/lgulliver/lodestone/internal/registry/registries/rubygems"
)

// Factory creates registry handlers based on package format
type Factory struct {
	service *Service
}

// NewFactory creates a new registry factory
func NewFactory(service *Service) *Factory {
	return &Factory{
		service: service,
	}
}

// GetRegistryHandler returns the appropriate registry handler for the given format
func (f *Factory) GetRegistryHandler(format string) Handler {
	switch format {
	case "npm":
		return npm.New(f.service.Storage, f.service.DB)
	case "nuget":
		return nuget.New(f.service.Storage, f.service.DB)
	case "oci":
		return oci.New(f.service.Storage, f.service.DB)
	case "maven":
		return maven.New(f.service.Storage, f.service.DB)
	case "go":
		return goregistry.New(f.service.Storage, f.service.DB)
	case "helm":
		return helm.New(f.service.Storage, f.service.DB)
	case "cargo":
		return cargo.New(f.service.Storage, f.service.DB)
	case "rubygems":
		return rubygems.New(f.service.Storage, f.service.DB)
	case "opa":
		return opa.New(f.service.Storage, f.service.DB)
	default:
		// Return a generic handler or null handler as fallback
		return nil
	}
}
