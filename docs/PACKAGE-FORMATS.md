# Package Format Quick Reference

Quick reference guides for working with different package formats in Lodestone.

## Current Implementation Status (validated)

| Feed | Workflow Status | Notes |
|---|---|---|
| [NuGet](NUGET.md) | ✅ Upload + Download | `.nupkg` upload/download verified; `.snupkg` upload supported via curl |
| [npm](package-feeds/npm.md) | ✅ Upload + Download | npm publish/download verified |
| [Helm](package-feeds/helm.md) | ✅ Upload + Download | multipart chart upload and chart download verified |
| [Cargo](package-feeds/cargo.md) | ✅ Upload + Download | upload/download verified with route-friendly crate name (`lodestone-cargo`) |
| [Go Modules](package-feeds/go.md) | ✅ Upload + Download (simple module path) | works for module names without path separators (e.g. `lodestone`) |
| [OPA](package-feeds/opa.md) | ✅ Upload + Download | bundle upload/download verified |
| [Maven](package-feeds/maven.md) | ⚠️ Upload only | upload works; download currently fails due upload/download path-to-package mapping mismatch |
| [RubyGems](package-feeds/rubygems.md) | ⚠️ Not stable | upload currently returns `500`; download not available for uploaded artifact |
| [OCI/Docker](package-feeds/oci.md) | ⚠️ Blocked in current runtime path | `/v2/*` currently returns `503` registry disabled on OCI route in current environment |

---

## NuGet (.NET Packages)

### Basic Usage
```bash
# Add source
dotnet nuget add source "http://localhost:8080/api/v1/nuget/v3/index.json" --name "Lodestone"

# Push package
dotnet nuget push package.nupkg --source "Lodestone" --api-key "your-key"
```

### ⚠️ Symbol Package Caveat
**Important**: `dotnet nuget push` doesn't work with `.snupkg` files. Use curl instead:

```bash
# Regular package - works with dotnet CLI
dotnet nuget push MyPackage.1.0.0.nupkg --source "Lodestone" --api-key "your-key"

# Symbol package - use curl
curl -X PUT "http://localhost:8080/api/v1/nuget/v2/symbolpackage" \
    -H "X-NuGet-ApiKey: your-key" \
    -H "Content-Type: application/octet-stream" \
    --data-binary @"MyPackage.1.0.0.snupkg"
```

**Full Documentation**: [NuGet Guide](NUGET.md)

---

## Maven (Java Packages)
Current note: upload endpoint is active, but download currently fails due package key/path parsing mismatch.

## npm (Node.js Packages)
npm publish/download flows are implemented and validated.

## Cargo (Rust Packages)
Cargo upload/download flows are implemented and validated.

## OCI (Container Images)
Endpoints are implemented, but the runtime path is currently blocked by OCI registry enablement behavior (`503` on `/v2/*`).

## Helm (Kubernetes Charts)
Helm upload/download flows are implemented and validated.

---

## Feed Guides

See **[package-feeds/README.md](package-feeds/README.md)** for concise, route-accurate examples for npm, Maven, Cargo, Go, Helm, RubyGems, OPA, and OCI.
