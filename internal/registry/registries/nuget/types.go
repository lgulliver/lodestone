package nuget

import (
	"encoding/xml"
	"time"
)

// XML parsing types for .nuspec files
// NuSpecMetadata represents the metadata section of a .nuspec file
type NuSpecMetadata struct {
	ID                       string              `xml:"id"`
	Version                  string              `xml:"version"`
	Title                    string              `xml:"title,omitempty"`
	Authors                  string              `xml:"authors"`
	Owners                   string              `xml:"owners,omitempty"`
	Description              string              `xml:"description"`
	Summary                  string              `xml:"summary,omitempty"`
	ReleaseNotes             string              `xml:"releaseNotes,omitempty"`
	Copyright                string              `xml:"copyright,omitempty"`
	Language                 string              `xml:"language,omitempty"`
	ProjectURL               string              `xml:"projectUrl,omitempty"`
	IconURL                  string              `xml:"iconUrl,omitempty"`
	LicenseURL               string              `xml:"licenseUrl,omitempty"`
	Tags                     string              `xml:"tags,omitempty"`
	RequireLicenseAcceptance bool                `xml:"requireLicenseAcceptance,omitempty"`
	DevelopmentDependency    bool                `xml:"developmentDependency,omitempty"`
	Dependencies             *Dependencies       `xml:"dependencies,omitempty"`
	FrameworkAssemblies      []FrameworkAssembly `xml:"frameworkAssemblies>frameworkAssembly,omitempty"`
	References               []Reference         `xml:"references>reference,omitempty"`
	MinClientVersion         string              `xml:"minClientVersion,omitempty"`
	PackageTypes             []PackageType       `xml:"packageTypes>packageType,omitempty"`
	Repository               *Repository         `xml:"repository,omitempty"`
	License                  *License            `xml:"license,omitempty"`
	Icon                     string              `xml:"icon,omitempty"`
	Readme                   string              `xml:"readme,omitempty"`
	TargetFrameworks         []TargetFramework   `xml:"metadata>targetFramework,omitempty"`
}

type NuSpec struct {
	XMLName  xml.Name       `xml:"package"`
	Metadata NuSpecMetadata `xml:"metadata"`
	Files    []File         `xml:"files>file,omitempty"`
}

// Dependencies represents package dependencies
type Dependencies struct {
	Groups []DependencyGroup `xml:"group,omitempty"`
	// For packages without groups, dependencies are direct children
	Dependencies []Dependency `xml:"dependency,omitempty"`
}

// DependencyGroup represents a group of dependencies for a specific framework
type DependencyGroup struct {
	TargetFramework string       `xml:"targetFramework,attr,omitempty"`
	Dependencies    []Dependency `xml:"dependency,omitempty"`
}

// Dependency represents a single package dependency
type Dependency struct {
	ID      string `xml:"id,attr"`
	Version string `xml:"version,attr,omitempty"`
	Include string `xml:"include,attr,omitempty"`
	Exclude string `xml:"exclude,attr,omitempty"`
}

// FrameworkAssembly represents a framework assembly reference
type FrameworkAssembly struct {
	AssemblyName    string `xml:"assemblyName,attr"`
	TargetFramework string `xml:"targetFramework,attr,omitempty"`
}

// Reference represents a reference
type Reference struct {
	File string `xml:"file,attr"`
}

// PackageType represents the type of package
type PackageType struct {
	Name    string `xml:"name,attr"`
	Version string `xml:"version,attr,omitempty"`
}

// Repository represents repository information
type Repository struct {
	Type   string `xml:"type,attr,omitempty"`
	URL    string `xml:"url,attr,omitempty"`
	Branch string `xml:"branch,attr,omitempty"`
	Commit string `xml:"commit,attr,omitempty"`
}

// License represents license information
type License struct {
	Type       string `xml:"type,attr,omitempty"`
	Version    string `xml:"version,attr,omitempty"`
	Expression string `xml:",chardata"`
}

// File represents a file in the package
type File struct {
	Src     string `xml:"src,attr"`
	Target  string `xml:"target,attr,omitempty"`
	Exclude string `xml:"exclude,attr,omitempty"`
}

// TargetFramework represents target framework information
type TargetFramework struct {
	Moniker string `xml:",chardata"`
}

// Service Index types for NuGet v3 API
type ServiceIndex struct {
	Version   string     `json:"version"`
	Resources []Resource `json:"resources"`
}

type Resource struct {
	ID      string `json:"@id"`
	Type    string `json:"@type"`
	Comment string `json:"comment"`
}

// Search response types
type SearchResult struct {
	ID             string   `json:"id"`
	Version        string   `json:"version"`
	Description    string   `json:"description"`
	Authors        []string `json:"authors"`
	TotalDownloads int64    `json:"totalDownloads"`
	Verified       bool     `json:"verified"`
	Tags           []string `json:"tags,omitempty"`
}

type SearchResponse struct {
	TotalHits int            `json:"totalHits"`
	Data      []SearchResult `json:"data"`
}

// Registration/metadata types
type CatalogEntry struct {
	ID             string                 `json:"@id"`
	Type           string                 `json:"@type"`
	Authors        string                 `json:"authors"`
	Description    string                 `json:"description"`
	PackageID      string                 `json:"id"`
	Version        string                 `json:"version"`
	Published      time.Time              `json:"published"`
	PackageContent string                 `json:"packageContent"`
	Dependencies   map[string]interface{} `json:"dependencies,omitempty"`
}

type RegistrationPageItem struct {
	ID              string       `json:"@id"`
	Type            string       `json:"@type"`
	CommitID        string       `json:"commitId"`
	CommitTimeStamp time.Time    `json:"commitTimeStamp"`
	CatalogEntry    CatalogEntry `json:"catalogEntry"`
	PackageContent  string       `json:"packageContent"`
	Registration    string       `json:"registration"`
}

type RegistrationPage struct {
	ID     string                 `json:"@id"`
	Type   string                 `json:"@type"`
	Count  int                    `json:"count"`
	Items  []RegistrationPageItem `json:"items"`
	Lower  string                 `json:"lower"`
	Upper  string                 `json:"upper"`
	Parent string                 `json:"parent"`
}

type RegistrationIndex struct {
	ID    string             `json:"@id"`
	Type  []string           `json:"@type"`
	Count int                `json:"count"`
	Items []RegistrationPage `json:"items"`
}

// Package versions response
type PackageVersionsResponse struct {
	Versions []string `json:"versions"`
}

// Symbol package types
type SymbolPackageInfo struct {
	PackageName   string
	Version       string
	HasSymbols    bool
	PdbFiles      []string
	SourceFiles   []string
	NuSpecContent []byte
}

// Upload request types
type UploadRequest struct {
	PackageName string
	Version     string
	Content     []byte
	ContentType string
	IsSymbol    bool
}

// Validation result types
type ValidationResult struct {
	Valid     bool
	Errors    []string
	Warnings  []string
	Metadata  *NuSpecMetadata
	IsSymbol  bool
	FileCount int
	Size      int64
}
