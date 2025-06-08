> **⚠️ Lodestone is a work in progress and not yet feature complete. Expect breaking changes and incomplete functionality. Contributions and feedback are welcome!**

# Lodestone - Self-Hosted Artifact Feed

Lodestone is an open-source, self-hosted artifact feed that supports multiple package formats with a modern web UI. It's designed to run on Kubernetes and provides secure authentication for both publishing and consuming artifacts.

## Supported Artifact Types

- **NuGet** - .NET packages
- **OCI** - Container images
- **OPA** - Open Policy Agent bundles
- **Maven** - Java packages
- **npm** - Node.js packages
- **Cargo** - Rust packages
- **Go** - Go modules
- **Helm** - Kubernetes charts
- **RubyGems** - Ruby packages

## Architecture

Lodestone follows a microservices architecture with the following components:

- **API Gateway** - Single entry point, routing, and load balancing
- **Authentication Service** - JWT tokens, RBAC, API keys
- **Registry Service** - Pluggable artifact handlers
- **Metadata Service** - Search, indexing, and package metadata
- **Storage Backend** - Configurable blob storage (S3, GCS, Azure, local)

## Quick Start

### Using Deployment Scripts (Recommended)

```bash
# First-time setup for local development
./deploy/scripts/setup.sh local

# Start the deployment
./deploy/scripts/deploy.sh up local

# Check health
./deploy/scripts/health-check.sh
```

### Using Make Commands

```bash
# Build all services
make build

# Run locally with Docker Compose
make dev

# Deploy to Kubernetes
make deploy
```

For detailed deployment options, see [deploy/README.md](deploy/README.md).

## Package Format Documentation

Detailed guides for using Lodestone with specific package formats:

- **[NuGet Documentation](docs/NUGET.md)** - Complete guide for .NET packages, including symbol packages
- **Maven** - Java package management (coming soon)
- **npm** - Node.js package management (coming soon)
- **Cargo** - Rust package management (coming soon)
- **OCI** - Container image management (coming soon)
- **Helm** - Kubernetes chart management (coming soon)

## Authentication

All package operations require API key authentication. Generate API keys through the web interface or CLI:

```bash
# Generate a new API key
curl -X POST "http://localhost:8080/api/v1/auth/api-keys" \
    -H "Authorization: Bearer your-jwt-token" \
    -H "Content-Type: application/json" \
    -d '{"name": "my-build-key", "scopes": ["read", "write"]}'
```

API keys should be included in requests using the `X-NuGet-ApiKey` header for NuGet operations, or similar format for other package types.

## Development

For development setup and guidelines:

```bash
# Install dependencies
go mod download

# Run tests
make test

# Build locally
make build

# Run with hot reload (requires air)
air
```

See the code structure and architecture documentation in the codebase for development guidelines.

## License

This project is licensed under the GNU General Public License v3.0 - see the [LICENSE](LICENSE) file for details.
