# Package Format Quick Reference

Quick reference guides for working with different package formats in Lodestone.

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

Coming soon...

## npm (Node.js Packages)

Coming soon...

## Cargo (Rust Packages)

Coming soon...

## OCI (Container Images)

Coming soon...

## Helm (Kubernetes Charts)

Coming soon...
