package registry

import (
	"github.com/lgulliver/lodestone/internal/registry/registries/cargo"
	"github.com/lgulliver/lodestone/internal/registry/registries/go_registry"
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
		return npm_registry.New(f.service)
	case "nuget":
		return nuget_registry.New(f.service)
	case "oci":
		return oci_registry.New(f.service)
	case "maven":
		return maven_registry.New(f.service)
	case "go":
		return go_registry.New(f.service)
	case "helm":
		return helm_registry.New(f.service)
	case "cargo":
		return cargo_registry.New(f.service)
	case "rubygems":
		return rubygems_registry.New(f.service)
	case "opa":
		return opa_registry.New(f.service)
	default:
		// Return a generic handler or null handler as fallback
		return nil
	}
}
