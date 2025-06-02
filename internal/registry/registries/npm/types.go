package npm

// PackageManifest represents package.json structure
type PackageManifest struct {
	Name             string                 `json:"name"`
	Version          string                 `json:"version"`
	Description      string                 `json:"description,omitempty"`
	Author           interface{}            `json:"author,omitempty"` // Can be string or object
	License          string                 `json:"license,omitempty"`
	Dependencies     map[string]string      `json:"dependencies,omitempty"`
	DevDependencies  map[string]string      `json:"devDependencies,omitempty"`
	Keywords         []string               `json:"keywords,omitempty"`
	Repository       interface{}            `json:"repository,omitempty"`       // Can be string or object
	DistTags         map[string]string      `json:"dist-tags,omitempty"`        // For npm dist-tags like latest, beta, etc.
	PublishConfig    map[string]interface{} `json:"publishConfig,omitempty"`    // For npm publish configuration
	Time             interface{}            `json:"time,omitempty"`             // For version timestamps
	Homepage         string                 `json:"homepage,omitempty"`         // Project homepage URL
	Bugs             interface{}            `json:"bugs,omitempty"`             // Issue tracker details
	Scripts          map[string]string      `json:"scripts,omitempty"`          // NPM scripts
	Contributors     interface{}            `json:"contributors,omitempty"`     // Can be array of strings or objects
	Engines          map[string]string      `json:"engines,omitempty"`          // Engine compatibility
	PeerDependencies map[string]string      `json:"peerDependencies,omitempty"` // Peer dependencies
	Deprecated       string                 `json:"deprecated,omitempty"`       // Deprecation message
}

// NPMRegistryResponse represents the npm registry API response format
type NPMRegistryResponse struct {
	ID           string                    `json:"_id"`
	Rev          string                    `json:"_rev,omitempty"`
	Name         string                    `json:"name"`
	Description  string                    `json:"description,omitempty"`
	DistTags     map[string]string         `json:"dist-tags"`
	Versions     map[string]VersionDetails `json:"versions"`
	Time         map[string]string         `json:"time,omitempty"`
	Author       interface{}               `json:"author,omitempty"`
	Repository   interface{}               `json:"repository,omitempty"`
	Homepage     string                    `json:"homepage,omitempty"`
	Keywords     []string                  `json:"keywords,omitempty"`
	License      string                    `json:"license,omitempty"`
	ReadmeFile   string                    `json:"readme,omitempty"`
	Maintainers  []interface{}             `json:"maintainers,omitempty"`
	Contributors []interface{}             `json:"contributors,omitempty"`
}

// VersionDetails represents detailed version information in npm registry responses
type VersionDetails struct {
	Name             string                 `json:"name"`
	Version          string                 `json:"version"`
	Description      string                 `json:"description,omitempty"`
	Main             string                 `json:"main,omitempty"`
	Scripts          map[string]string      `json:"scripts,omitempty"`
	Author           interface{}            `json:"author,omitempty"`
	License          string                 `json:"license,omitempty"`
	Dependencies     map[string]string      `json:"dependencies,omitempty"`
	DevDependencies  map[string]string      `json:"devDependencies,omitempty"`
	PeerDependencies map[string]string      `json:"peerDependencies,omitempty"`
	Keywords         []string               `json:"keywords,omitempty"`
	Repository       interface{}            `json:"repository,omitempty"`
	Homepage         string                 `json:"homepage,omitempty"`
	Bugs             interface{}            `json:"bugs,omitempty"`
	Contributors     []interface{}          `json:"contributors,omitempty"`
	Engines          map[string]string      `json:"engines,omitempty"`
	PublishConfig    map[string]interface{} `json:"publishConfig,omitempty"`
	Dist             DistInfo               `json:"dist"`
	Deprecated       string                 `json:"deprecated,omitempty"`
	HasShrinkwrap    bool                   `json:"_hasShrinkwrap,omitempty"`
	ID               string                 `json:"_id"`
	NodeVersion      string                 `json:"_nodeVersion,omitempty"`
	NPMVersion       string                 `json:"_npmVersion,omitempty"`
}

// DistInfo represents distribution information for a package version
type DistInfo struct {
	Integrity    string `json:"integrity,omitempty"`
	Shasum       string `json:"shasum"`
	Tarball      string `json:"tarball"`
	FileCount    int    `json:"fileCount,omitempty"`
	UnpackedSize int64  `json:"unpackedSize,omitempty"`
}

// NPMSearchResult represents search results from npm registry
type NPMSearchResult struct {
	Package     SearchPackage `json:"package"`
	Score       SearchScore   `json:"score"`
	SearchScore float64       `json:"searchScore"`
}

// SearchPackage represents package information in search results
type SearchPackage struct {
	Name        string            `json:"name"`
	Scope       string            `json:"scope,omitempty"`
	Version     string            `json:"version"`
	Description string            `json:"description,omitempty"`
	Keywords    []string          `json:"keywords,omitempty"`
	Date        string            `json:"date"`
	Links       map[string]string `json:"links,omitempty"`
	Author      interface{}       `json:"author,omitempty"`
	Publisher   interface{}       `json:"publisher,omitempty"`
	Maintainers []interface{}     `json:"maintainers,omitempty"`
}

// SearchScore represents scoring information in search results
type SearchScore struct {
	Final  float64            `json:"final"`
	Detail map[string]float64 `json:"detail"`
}

// NPMSearchResponse represents the response from npm search API
type NPMSearchResponse struct {
	Objects []NPMSearchResult `json:"objects"`
	Total   int               `json:"total"`
	Time    string            `json:"time"`
}
