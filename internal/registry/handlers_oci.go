package registry

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/lgulliver/lodestone/pkg/types"
)

// OCIRegistry implements the OCI/container registry following the OCI Distribution Spec
type OCIRegistry struct {
	service *Service
}

// OCIManifest represents an OCI image manifest
type OCIManifest struct {
	SchemaVersion int    `json:"schemaVersion"`
	MediaType     string `json:"mediaType"`
	Config        struct {
		MediaType string `json:"mediaType"`
		Size      int64  `json:"size"`
		Digest    string `json:"digest"`
	} `json:"config"`
	Layers []struct {
		MediaType string `json:"mediaType"`
		Size      int64  `json:"size"`
		Digest    string `json:"digest"`
	} `json:"layers"`
}

// OCIImageConfig represents the configuration of an OCI image
type OCIImageConfig struct {
	Architecture string `json:"architecture"`
	OS           string `json:"os"`
	Config       struct {
		ExposedPorts map[string]struct{} `json:"ExposedPorts,omitempty"`
		Env          []string            `json:"Env,omitempty"`
		Entrypoint   []string            `json:"Entrypoint,omitempty"`
		Cmd          []string            `json:"Cmd,omitempty"`
		WorkingDir   string              `json:"WorkingDir,omitempty"`
	} `json:"config"`
	RootFS struct {
		Type    string   `json:"type"`
		DiffIDs []string `json:"diff_ids"`
	} `json:"rootfs"`
}

// Upload stores an OCI artifact (manifest, blob, or config)
func (r *OCIRegistry) Upload(artifact *types.Artifact, content []byte) error {
	ctx := context.Background()
	
	// Determine content type based on artifact type
	contentType := r.getContentType(artifact)
	
	// Store the content
	reader := bytes.NewReader(content)
	if err := r.service.storage.Store(ctx, artifact.StoragePath, reader, contentType); err != nil {
		return fmt.Errorf("failed to store OCI artifact: %w", err)
	}
	
	artifact.ContentType = contentType
	return nil
}

// Download retrieves an OCI artifact
func (r *OCIRegistry) Download(name, version string) (*types.Artifact, []byte, error) {
	return nil, nil, fmt.Errorf("use service.Download instead")
}

// List returns OCI artifacts matching the filter
func (r *OCIRegistry) List(filter *types.ArtifactFilter) ([]*types.Artifact, error) {
	return nil, fmt.Errorf("use service.List instead")
}

// Delete removes an OCI artifact
func (r *OCIRegistry) Delete(name, version string) error {
	return fmt.Errorf("use service.Delete instead")
}

// Validate checks if the artifact is a valid OCI artifact
func (r *OCIRegistry) Validate(artifact *types.Artifact, content []byte) error {
	if len(content) == 0 {
		return fmt.Errorf("empty OCI artifact content")
	}

	// Validate repository name format
	repoNameRegex := regexp.MustCompile(`^[a-z0-9]+(?:[._-][a-z0-9]+)*(?:/[a-z0-9]+(?:[._-][a-z0-9]+)*)*$`)
	if !repoNameRegex.MatchString(artifact.Name) {
		return fmt.Errorf("invalid repository name format")
	}

	// Validate tag or digest format
	if strings.HasPrefix(artifact.Version, "sha256:") {
		// It's a digest
		digestRegex := regexp.MustCompile(`^sha256:[a-f0-9]{64}$`)
		if !digestRegex.MatchString(artifact.Version) {
			return fmt.Errorf("invalid digest format")
		}
	} else {
		// It's a tag
		tagRegex := regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]*$`)
		if !tagRegex.MatchString(artifact.Version) {
			return fmt.Errorf("invalid tag format")
		}
	}

	// Validate JSON structure for manifests
	if strings.Contains(artifact.ContentType, "manifest") {
		var manifest OCIManifest
		if err := json.Unmarshal(content, &manifest); err != nil {
			return fmt.Errorf("invalid manifest JSON: %w", err)
		}
		if manifest.SchemaVersion != 2 {
			return fmt.Errorf("unsupported manifest schema version: %d", manifest.SchemaVersion)
		}
	}

	return nil
}

// GetMetadata extracts metadata from OCI artifact
func (r *OCIRegistry) GetMetadata(content []byte) (map[string]interface{}, error) {
	metadata := map[string]interface{}{
		"format": "oci",
		"type":   "unknown",
	}

	// Try to parse as manifest
	var manifest OCIManifest
	if err := json.Unmarshal(content, &manifest); err == nil {
		metadata["type"] = "manifest"
		metadata["schema_version"] = manifest.SchemaVersion
		metadata["media_type"] = manifest.MediaType
		metadata["layers_count"] = len(manifest.Layers)
		
		// Calculate total size
		var totalSize int64
		for _, layer := range manifest.Layers {
			totalSize += layer.Size
		}
		metadata["total_size"] = totalSize
		
		return metadata, nil
	}

	// Try to parse as image config
	var config OCIImageConfig
	if err := json.Unmarshal(content, &config); err == nil {
		metadata["type"] = "config"
		metadata["architecture"] = config.Architecture
		metadata["os"] = config.OS
		return metadata, nil
	}

	// If it's binary data, it's likely a blob
	metadata["type"] = "blob"
	metadata["size"] = len(content)
	
	return metadata, nil
}

// GenerateStoragePath creates the storage path for OCI artifacts
func (r *OCIRegistry) GenerateStoragePath(name, version string) string {
	if strings.HasPrefix(version, "sha256:") {
		// It's a blob/manifest referenced by digest
		digest := strings.TrimPrefix(version, "sha256:")
		return fmt.Sprintf("oci/blobs/sha256/%s", digest)
	}
	// It's a manifest referenced by tag
	return fmt.Sprintf("oci/manifests/%s/%s", name, version)
}

// getContentType determines the appropriate content type for OCI artifacts
func (r *OCIRegistry) getContentType(artifact *types.Artifact) string {
	// Determine based on storage path or metadata
	if strings.Contains(artifact.StoragePath, "/manifests/") {
		return "application/vnd.oci.image.manifest.v1+json"
	}
	if strings.Contains(artifact.StoragePath, "/blobs/") {
		// Could be a layer or config
		if artifact.Metadata != nil {
			if artifactType, ok := artifact.Metadata["type"].(string); ok {
				switch artifactType {
				case "config":
					return "application/vnd.oci.image.config.v1+json"
				case "layer":
					return "application/vnd.oci.image.layer.v1.tar+gzip"
				}
			}
		}
		return "application/octet-stream"
	}
	return "application/vnd.oci.image.manifest.v1+json"
}

// GetManifestResponse creates a proper OCI manifest response
func (r *OCIRegistry) GetManifestResponse(name, tag string, manifest []byte) (map[string]string, []byte) {
	headers := map[string]string{
		"Content-Type":                 "application/vnd.oci.image.manifest.v1+json",
		"Docker-Content-Digest":        r.calculateDigest(manifest),
		"Docker-Distribution-API-Version": "registry/2.0",
	}
	return headers, manifest
}

// calculateDigest calculates the SHA256 digest of content
func (r *OCIRegistry) calculateDigest(content []byte) string {
	// TODO: Implement SHA256 digest calculation
	return "sha256:placeholder"
}
