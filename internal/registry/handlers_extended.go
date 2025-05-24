package registry

import (
	"bytes"
	"context"
	"fmt"

	"github.com/lgulliver/lodestone/pkg/types"
)

// MavenRegistry implements the Maven package registry
type MavenRegistry struct {
	service *Service
}

func (r *MavenRegistry) Upload(artifact *types.Artifact, content []byte) error {
	ctx := context.Background()
	reader := bytes.NewReader(content)
	if err := r.service.storage.Store(ctx, artifact.StoragePath, reader, "application/java-archive"); err != nil {
		return fmt.Errorf("failed to store Maven artifact: %w", err)
	}
	artifact.ContentType = "application/java-archive"
	return nil
}

func (r *MavenRegistry) Download(name, version string) (*types.Artifact, []byte, error) {
	return nil, nil, fmt.Errorf("use service.Download instead")
}

func (r *MavenRegistry) List(filter *types.ArtifactFilter) ([]*types.Artifact, error) {
	return nil, fmt.Errorf("use service.List instead")
}

func (r *MavenRegistry) Delete(name, version string) error {
	return fmt.Errorf("use service.Delete instead")
}

func (r *MavenRegistry) Validate(artifact *types.Artifact, content []byte) error {
	if len(content) == 0 {
		return fmt.Errorf("empty artifact content")
	}
	// TODO: Add Maven-specific validation
	return nil
}

func (r *MavenRegistry) GetMetadata(content []byte) (map[string]interface{}, error) {
	return map[string]interface{}{
		"format": "maven",
		"type": "jar",
	}, nil
}

// HelmRegistry implements the Helm chart registry
type HelmRegistry struct {
	service *Service
}

func (r *HelmRegistry) Upload(artifact *types.Artifact, content []byte) error {
	ctx := context.Background()
	reader := bytes.NewReader(content)
	if err := r.service.storage.Store(ctx, artifact.StoragePath, reader, "application/gzip"); err != nil {
		return fmt.Errorf("failed to store Helm chart: %w", err)
	}
	artifact.ContentType = "application/gzip"
	return nil
}

func (r *HelmRegistry) Download(name, version string) (*types.Artifact, []byte, error) {
	return nil, nil, fmt.Errorf("use service.Download instead")
}

func (r *HelmRegistry) List(filter *types.ArtifactFilter) ([]*types.Artifact, error) {
	return nil, fmt.Errorf("use service.List instead")
}

func (r *HelmRegistry) Delete(name, version string) error {
	return fmt.Errorf("use service.Delete instead")
}

func (r *HelmRegistry) Validate(artifact *types.Artifact, content []byte) error {
	if len(content) == 0 {
		return fmt.Errorf("empty chart content")
	}
	// TODO: Add Helm-specific validation (check for Chart.yaml, etc.)
	return nil
}

func (r *HelmRegistry) GetMetadata(content []byte) (map[string]interface{}, error) {
	return map[string]interface{}{
		"format": "helm",
		"type": "chart",
	}, nil
}

// OCIRegistry implements the OCI/container registry
type OCIRegistry struct {
	service *Service
}

func (r *OCIRegistry) Upload(artifact *types.Artifact, content []byte) error {
	ctx := context.Background()
	reader := bytes.NewReader(content)
	if err := r.service.storage.Store(ctx, artifact.StoragePath, reader, "application/vnd.oci.image.manifest.v1+json"); err != nil {
		return fmt.Errorf("failed to store OCI artifact: %w", err)
	}
	artifact.ContentType = "application/vnd.oci.image.manifest.v1+json"
	return nil
}

func (r *OCIRegistry) Download(name, version string) (*types.Artifact, []byte, error) {
	return nil, nil, fmt.Errorf("use service.Download instead")
}

func (r *OCIRegistry) List(filter *types.ArtifactFilter) ([]*types.Artifact, error) {
	return nil, fmt.Errorf("use service.List instead")
}

func (r *OCIRegistry) Delete(name, version string) error {
	return fmt.Errorf("use service.Delete instead")
}

func (r *OCIRegistry) Validate(artifact *types.Artifact, content []byte) error {
	if len(content) == 0 {
		return fmt.Errorf("empty OCI artifact content")
	}
	// TODO: Add OCI-specific validation
	return nil
}

func (r *OCIRegistry) GetMetadata(content []byte) (map[string]interface{}, error) {
	return map[string]interface{}{
		"format": "oci",
		"type": "image",
	}, nil
}

// OPARegistry implements the Open Policy Agent bundle registry
type OPARegistry struct {
	service *Service
}

func (r *OPARegistry) Upload(artifact *types.Artifact, content []byte) error {
	ctx := context.Background()
	reader := bytes.NewReader(content)
	if err := r.service.storage.Store(ctx, artifact.StoragePath, reader, "application/gzip"); err != nil {
		return fmt.Errorf("failed to store OPA bundle: %w", err)
	}
	artifact.ContentType = "application/gzip"
	return nil
}

func (r *OPARegistry) Download(name, version string) (*types.Artifact, []byte, error) {
	return nil, nil, fmt.Errorf("use service.Download instead")
}

func (r *OPARegistry) List(filter *types.ArtifactFilter) ([]*types.Artifact, error) {
	return nil, fmt.Errorf("use service.List instead")
}

func (r *OPARegistry) Delete(name, version string) error {
	return fmt.Errorf("use service.Delete instead")
}

func (r *OPARegistry) Validate(artifact *types.Artifact, content []byte) error {
	if len(content) == 0 {
		return fmt.Errorf("empty OPA bundle content")
	}
	// TODO: Add OPA-specific validation
	return nil
}

func (r *OPARegistry) GetMetadata(content []byte) (map[string]interface{}, error) {
	return map[string]interface{}{
		"format": "opa",
		"type": "bundle",
	}, nil
}

// CargoRegistry implements the Rust/Cargo package registry
type CargoRegistry struct {
	service *Service
}

func (r *CargoRegistry) Upload(artifact *types.Artifact, content []byte) error {
	ctx := context.Background()
	reader := bytes.NewReader(content)
	if err := r.service.storage.Store(ctx, artifact.StoragePath, reader, "application/gzip"); err != nil {
		return fmt.Errorf("failed to store Cargo crate: %w", err)
	}
	artifact.ContentType = "application/gzip"
	return nil
}

func (r *CargoRegistry) Download(name, version string) (*types.Artifact, []byte, error) {
	return nil, nil, fmt.Errorf("use service.Download instead")
}

func (r *CargoRegistry) List(filter *types.ArtifactFilter) ([]*types.Artifact, error) {
	return nil, fmt.Errorf("use service.List instead")
}

func (r *CargoRegistry) Delete(name, version string) error {
	return fmt.Errorf("use service.Delete instead")
}

func (r *CargoRegistry) Validate(artifact *types.Artifact, content []byte) error {
	if len(content) == 0 {
		return fmt.Errorf("empty crate content")
	}
	// TODO: Add Cargo-specific validation
	return nil
}

func (r *CargoRegistry) GetMetadata(content []byte) (map[string]interface{}, error) {
	return map[string]interface{}{
		"format": "cargo",
		"type": "crate",
	}, nil
}

// RubyGemsRegistry implements the RubyGems package registry
type RubyGemsRegistry struct {
	service *Service
}

func (r *RubyGemsRegistry) Upload(artifact *types.Artifact, content []byte) error {
	ctx := context.Background()
	reader := bytes.NewReader(content)
	if err := r.service.storage.Store(ctx, artifact.StoragePath, reader, "application/x-tar"); err != nil {
		return fmt.Errorf("failed to store Ruby gem: %w", err)
	}
	artifact.ContentType = "application/x-tar"
	return nil
}

func (r *RubyGemsRegistry) Download(name, version string) (*types.Artifact, []byte, error) {
	return nil, nil, fmt.Errorf("use service.Download instead")
}

func (r *RubyGemsRegistry) List(filter *types.ArtifactFilter) ([]*types.Artifact, error) {
	return nil, fmt.Errorf("use service.List instead")
}

func (r *RubyGemsRegistry) Delete(name, version string) error {
	return fmt.Errorf("use service.Delete instead")
}

func (r *RubyGemsRegistry) Validate(artifact *types.Artifact, content []byte) error {
	if len(content) == 0 {
		return fmt.Errorf("empty gem content")
	}
	// TODO: Add RubyGems-specific validation
	return nil
}

func (r *RubyGemsRegistry) GetMetadata(content []byte) (map[string]interface{}, error) {
	return map[string]interface{}{
		"format": "rubygems",
		"type": "gem",
	}, nil
}
