# Lodestone Documentation

This directory contains detailed documentation for using and deploying Lodestone.

## Deployment and Operations

- **[DEPLOYMENT.md](DEPLOYMENT.md)** - Comprehensive deployment guide for production environments
- **[../deploy/README.md](../deploy/README.md)** - Quick deployment scripts and Docker Compose setup

## Package Format Guides

- **[NUGET.md](NUGET.md)** - Complete NuGet (.NET) package management guide
  - Regular packages (.nupkg)
  - Symbol packages (.snupkg) 
  - Authentication setup
  - Troubleshooting
- **[PACKAGE-FORMATS.md](PACKAGE-FORMATS.md)** - Quick reference for all package formats

## Key Features

### NuGet Highlights
✅ **Full NuGet v3 Protocol Support** - Compatible with dotnet CLI and Visual Studio  
✅ **Symbol Package Support** - Debug symbols (.snupkg) for better debugging experience  
✅ **Private Registry** - API key authentication for secure package management  
⚠️ **Symbol Upload Caveat** - Use curl for .snupkg uploads (dotnet CLI limitation)

### Multi-Format Support
- **NuGet** (.NET packages and symbols)
- **Maven** (Java packages) 
- **npm** (Node.js packages)
- **Cargo** (Rust packages)
- **OCI** (Container images)
- **Helm** (Kubernetes charts)
- **Go** (Go modules)
- **RubyGems** (Ruby packages)
- **OPA** (Open Policy Agent bundles)

## Quick Start

1. **Deploy**: Use `./deploy/scripts/setup.sh local` for quick local setup
2. **Configure**: Add Lodestone as a package source with your API key
3. **Publish**: Upload packages using standard tooling (with noted caveats)
4. **Consume**: Install packages from your private registry

For detailed instructions, see the format-specific guides above.
