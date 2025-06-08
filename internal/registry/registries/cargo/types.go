package cargo

// CargoManifest represents Cargo.toml structure
type CargoManifest struct {
	Package struct {
		Name          string   `toml:"name"`
		Version       string   `toml:"version"`
		Authors       []string `toml:"authors"`
		Description   string   `toml:"description"`
		Keywords      []string `toml:"keywords"`
		Categories    []string `toml:"categories"`
		License       string   `toml:"license"`
		Repository    string   `toml:"repository"`
		Homepage      string   `toml:"homepage"`
		Documentation string   `toml:"documentation"`
		ReadmeFile    string   `toml:"readme"`
		Edition       string   `toml:"edition"`
	} `toml:"package"`
	Dependencies      map[string]interface{} `toml:"dependencies,omitempty"`
	DevDependencies   map[string]interface{} `toml:"dev-dependencies,omitempty"`
	BuildDependencies map[string]interface{} `toml:"build-dependencies,omitempty"`
	Features          map[string][]string    `toml:"features,omitempty"`
	Target            map[string]interface{} `toml:"target,omitempty"`
	Bin               []CargoTarget          `toml:"bin,omitempty"`
	Lib               CargoTarget            `toml:"lib,omitempty"`
	Example           []CargoTarget          `toml:"example,omitempty"`
	Test              []CargoTarget          `toml:"test,omitempty"`
	Bench             []CargoTarget          `toml:"bench,omitempty"`
}

// CargoTarget represents a build target in Cargo.toml
type CargoTarget struct {
	Name             string   `toml:"name,omitempty"`
	Path             string   `toml:"path,omitempty"`
	Test             bool     `toml:"test,omitempty"`
	DocTest          bool     `toml:"doctest,omitempty"`
	Bench            bool     `toml:"bench,omitempty"`
	Doc              bool     `toml:"doc,omitempty"`
	Plugin           bool     `toml:"plugin,omitempty"`
	ProcMacro        bool     `toml:"proc-macro,omitempty"`
	Harness          bool     `toml:"harness,omitempty"`
	Edition          string   `toml:"edition,omitempty"`
	CrateType        []string `toml:"crate-type,omitempty"`
	RequiredFeatures []string `toml:"required-features,omitempty"`
}

// CargoDependency represents a dependency specification
type CargoDependency struct {
	Version         string   `toml:"version,omitempty"`
	Path            string   `toml:"path,omitempty"`
	Git             string   `toml:"git,omitempty"`
	Branch          string   `toml:"branch,omitempty"`
	Tag             string   `toml:"tag,omitempty"`
	Rev             string   `toml:"rev,omitempty"`
	Features        []string `toml:"features,omitempty"`
	Optional        bool     `toml:"optional,omitempty"`
	DefaultFeatures bool     `toml:"default-features,omitempty"`
	Package         string   `toml:"package,omitempty"`
	Registry        string   `toml:"registry,omitempty"`
}

// CargoIndexEntry represents an entry in the Cargo registry index
type CargoIndexEntry struct {
	Name     string              `json:"name"`
	Vers     string              `json:"vers"`
	Deps     []CargoIndexDep     `json:"deps"`
	Cksum    string              `json:"cksum"`
	Features map[string][]string `json:"features"`
	Yanked   bool                `json:"yanked"`
	Links    string              `json:"links,omitempty"`
}

// CargoIndexDep represents a dependency in the Cargo index
type CargoIndexDep struct {
	Name               string   `json:"name"`
	Req                string   `json:"req"`
	Features           []string `json:"features"`
	Optional           bool     `json:"optional"`
	DefaultFeatures    bool     `json:"default_features"`
	Target             string   `json:"target,omitempty"`
	Kind               string   `json:"kind"`
	Registry           string   `json:"registry,omitempty"`
	ExplicitNameInToml string   `json:"explicit_name_in_toml,omitempty"`
}

// CargoRegistryConfig represents the config.json for a Cargo registry
type CargoRegistryConfig struct {
	DL  string `json:"dl"`
	API string `json:"api"`
}

// CargoPublishRequest represents a publish request to Cargo registry
type CargoPublishRequest struct {
	Name          string                 `json:"name"`
	Vers          string                 `json:"vers"`
	Deps          []CargoIndexDep        `json:"deps"`
	Features      map[string][]string    `json:"features"`
	Authors       []string               `json:"authors"`
	Description   string                 `json:"description,omitempty"`
	Homepage      string                 `json:"homepage,omitempty"`
	Documentation string                 `json:"documentation,omitempty"`
	Keywords      []string               `json:"keywords,omitempty"`
	Categories    []string               `json:"categories,omitempty"`
	License       string                 `json:"license,omitempty"`
	LicenseFile   string                 `json:"license_file,omitempty"`
	Repository    string                 `json:"repository,omitempty"`
	Badges        map[string]interface{} `json:"badges,omitempty"`
	Links         string                 `json:"links,omitempty"`
}

// CargoSearchResponse represents the response from cargo search API
type CargoSearchResponse struct {
	Crates []CargoSearchResult `json:"crates"`
	Meta   CargoSearchMeta     `json:"meta"`
}

// CargoSearchResult represents a single crate in search results
type CargoSearchResult struct {
	Name             string     `json:"name"`
	MaxVersion       string     `json:"max_version"`
	Description      string     `json:"description"`
	Homepage         string     `json:"homepage,omitempty"`
	Repository       string     `json:"repository,omitempty"`
	Documentation    string     `json:"documentation,omitempty"`
	Keywords         []string   `json:"keywords"`
	Categories       []string   `json:"categories"`
	MaxStableVersion string     `json:"max_stable_version,omitempty"`
	Links            CargoLinks `json:"links"`
	ExactMatch       bool       `json:"exact_match"`
	CreatedAt        string     `json:"created_at"`
	UpdatedAt        string     `json:"updated_at"`
	Downloads        int64      `json:"downloads"`
	RecentDownloads  int64      `json:"recent_downloads,omitempty"`
}

// CargoLinks represents links associated with a crate
type CargoLinks struct {
	VersionDownloads     string `json:"version_downloads"`
	Versions             string `json:"versions"`
	Owners               string `json:"owners,omitempty"`
	OwnerTeam            string `json:"owner_team,omitempty"`
	OwnerUser            string `json:"owner_user,omitempty"`
	ReverseDepdendencies string `json:"reverse_dependencies"`
}

// CargoSearchMeta represents metadata in search response
type CargoSearchMeta struct {
	Total int `json:"total"`
}
