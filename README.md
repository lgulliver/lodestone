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

```bash
# Build all services
make build

# Run locally with Docker Compose
make dev

# Deploy to Kubernetes
make deploy
```

## Development

See [CONTRIBUTING.md](CONTRIBUTING.md) for development guidelines.

## License

This project is licensed under the GNU General Public License v3.0 - see the [LICENSE](LICENSE) file for details.
