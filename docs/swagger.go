// Package docs contains the OpenAPI documentation for Lodestone
//
//	@title			Lodestone Artifact Registry API
//	@version		1.0
//	@description	A multi-format artifact registry supporting NuGet, npm, Maven, Cargo, OCI/Docker, Helm, RubyGems, OPA, and Go modules.
//	@termsOfService	http://swagger.io/terms/
//
//	@contact.name	Lodestone API Support
//	@contact.url	http://www.swagger.io/support
//	@contact.email	support@swagger.io
//
//	@license.name	MIT
//	@license.url	https://opensource.org/licenses/MIT
//
//	@host		localhost:8080
//	@BasePath	/api/v1
//	@schemes	http https
//
//	@securityDefinitions.apikey	BearerAuth
//	@in							header
//	@name						Authorization
//	@description				Type "Bearer" followed by a space and JWT token.
//
//	@securityDefinitions.apikey	ApiKeyAuth
//	@in							header
//	@name						X-API-Key
//	@description				API key for authentication
//
//	@tag.name			Authentication
//	@tag.description	Authentication and authorization operations
//
//	@tag.name			Admin
//	@tag.description	Administrative operations for registry management
//
//	@tag.name			Package Ownership
//	@tag.description	Package ownership management operations
//
//	@tag.name			NuGet
//	@tag.description	NuGet package manager operations
//
//	@tag.name			npm
//	@tag.description	npm package manager operations
//
//	@tag.name			Maven
//	@tag.description	Maven repository operations
//
//	@tag.name			Go
//	@tag.description	Go module proxy operations
//
//	@tag.name			Helm
//	@tag.description	Helm chart repository operations
//
//	@tag.name			Cargo
//	@tag.description	Rust Cargo registry operations
//
//	@tag.name			RubyGems
//	@tag.description	RubyGems repository operations
//
//	@tag.name			OPA
//	@tag.description	Open Policy Agent bundle repository operations
//
//	@tag.name			OCI/Docker
//	@tag.description	OCI/Docker registry operations
package docs
