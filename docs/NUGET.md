# NuGet Package Management

Lodestone provides full support for NuGet packages, including regular packages (.nupkg) and symbol packages (.snupkg) for debugging. This guide covers how to configure and use the NuGet feed with Lodestone.

## Features

- ✅ **NuGet v3 Protocol** - Full compatibility with modern NuGet clients
- ✅ **Regular Packages** - Upload and download .nupkg files
- ✅ **Symbol Packages** - Upload and download .snupkg files with debug symbols
- ✅ **Private Registry** - API key authentication for all operations
- ✅ **Package Search** - Find packages by name and metadata
- ✅ **Package Metadata** - Rich package information and versioning
- ✅ **CLI Support** - Works with `dotnet` CLI and NuGet Package Manager

## Configuration

### 1. Add Lodestone as a NuGet Source

```bash
# Add the Lodestone NuGet feed
dotnet nuget add source "http://localhost:8080/api/v1/nuget/v3/index.json" \
    --name "Lodestone" \
    --username "your-username" \
    --password "your-api-key" \
    --store-password-in-clear-text

# Or using nuget.exe
nuget sources add -Name "Lodestone" \
    -Source "http://localhost:8080/api/v1/nuget/v3/index.json" \
    -Username "your-username" \
    -Password "your-api-key"
```

### 2. Configure NuGet.Config

Create or update your `NuGet.Config` file:

```xml
<?xml version="1.0" encoding="utf-8"?>
<configuration>
  <packageSources>
    <add key="Lodestone" value="http://localhost:8080/api/v1/nuget/v3/index.json" />
    <add key="nuget.org" value="https://api.nuget.org/v3/index.json" protocolVersion="3" />
  </packageSources>
  <packageSourceCredentials>
    <Lodestone>
      <add key="Username" value="your-username" />
      <add key="ClearTextPassword" value="your-api-key" />
    </Lodestone>
  </packageSourceCredentials>
  <apikeys>
    <add key="http://localhost:8080/api/v1/nuget" value="your-api-key" />
  </apikeys>
</configuration>
```

## Publishing Packages

### Regular Packages (.nupkg)

#### Using dotnet CLI

```bash
# Pack your project
dotnet pack MyProject.csproj -c Release

# Push to Lodestone
dotnet nuget push ./bin/Release/MyProject.1.0.0.nupkg \
    --source "Lodestone" \
    --api-key "your-api-key"
```

#### Using nuget.exe

```bash
# Pack using nuspec
nuget pack MyProject.nuspec

# Push to Lodestone
nuget push MyProject.1.0.0.nupkg \
    -Source "http://localhost:8080/api/v1/nuget/v2/package" \
    -ApiKey "your-api-key"
```

### Symbol Packages (.snupkg)

Symbol packages contain debugging information (.pdb files) and are essential for debugging applications that consume your packages.

#### Generating Symbol Packages

Configure your project to generate symbol packages:

```xml
<!-- In your .csproj file -->
<PropertyGroup>
  <IncludeSymbols>true</IncludeSymbols>
  <SymbolPackageFormat>snupkg</SymbolPackageFormat>
  <PublishRepositoryUrl>true</PublishRepositoryUrl>
  <EmbedUntrackedSources>true</EmbedUntrackedSources>
  <DebugType>embedded</DebugType>
</PropertyGroup>
```

```bash
# Pack with symbols
dotnet pack MyProject.csproj -c Release --include-symbols -p:SymbolPackageFormat=snupkg

# This will generate both:
# - MyProject.1.0.0.nupkg (regular package)
# - MyProject.1.0.0.snupkg (symbol package)
```

#### Publishing Symbol Packages

**⚠️ Important Caveat**: Due to limitations in the `dotnet` CLI, symbol packages must be uploaded using curl or other HTTP clients. The `dotnet nuget push` command doesn't work directly with .snupkg files.

```bash
# Upload regular package first
dotnet nuget push ./bin/Release/MyProject.1.0.0.nupkg \
    --source "Lodestone" \
    --api-key "your-api-key"

# Upload symbol package using curl
curl -X PUT "http://localhost:8080/api/v1/nuget/v2/symbolpackage" \
    -H "X-NuGet-ApiKey: your-api-key" \
    -H "Content-Type: application/octet-stream" \
    --data-binary @"./bin/Release/MyProject.1.0.0.snupkg"
```

#### Alternative: Using NuGet Package Explorer

You can also upload symbol packages using the NuGet Package Explorer UI or any tool that supports direct HTTP uploads to the symbol package endpoint.

## Consuming Packages

### Installing Packages

```bash
# Install from Lodestone feed
dotnet add package MyProject --source "Lodestone"

# Install specific version
dotnet add package MyProject --version 1.0.0 --source "Lodestone"

# Using Package Manager Console in Visual Studio
Install-Package MyProject -Source "Lodestone"
```

### Debugging with Symbol Packages

When symbol packages are available, Visual Studio and other debuggers can automatically download and use them for debugging:

1. **Visual Studio**: Go to Debug → Windows → Modules, right-click on your assembly, and select "Load Symbols"
2. **VS Code**: The C# extension will automatically use available symbols
3. **Command Line**: Use `dotnet-symbol` tool to download symbols manually

```bash
# Install dotnet-symbol tool
dotnet tool install --global dotnet-symbol

# Download symbols for debugging
dotnet-symbol --server-path "http://localhost:8080/api/v1/nuget/symbols/" MyAssembly.dll
```

## Package Management

### Searching Packages

```bash
# Search packages in Lodestone
dotnet package search "MyProject" --source "Lodestone"

# List all versions of a package
nuget list "MyProject" -Source "Lodestone" -AllVersions
```

### Deleting Packages

```bash
# Delete a specific version (requires authentication)
nuget delete MyProject 1.0.0 \
    -Source "http://localhost:8080/api/v1/nuget/v2/package" \
    -ApiKey "your-api-key"
```

## API Endpoints

Lodestone implements the NuGet v3 protocol with the following endpoints:

### Discovery
- `GET /api/v1/nuget/v3/index.json` - Service index (entry point)

### Package Content
- `GET /api/v1/nuget/v3-flatcontainer/{id}/index.json` - Package versions
- `GET /api/v1/nuget/v3-flatcontainer/{id}/{version}/{filename}` - Download package

### Package Publishing (v2 API)
- `PUT /api/v1/nuget/v2/package` - Upload regular packages
- `PUT /api/v1/nuget/v2/symbolpackage` - Upload symbol packages
- `DELETE /api/v1/nuget/v2/package/{id}/{version}` - Delete packages

### Symbol Server
- `GET /api/v1/nuget/symbols/{id}/{version}/{filename}` - Download symbol packages

### Search and Metadata
- `GET /api/v1/nuget/v3/search` - Search packages
- `GET /api/v1/nuget/v3/registration/{id}/index.json` - Package metadata

## Authentication

All operations require authentication using API keys:

1. **Header-based**: Include `X-NuGet-ApiKey: your-api-key` in HTTP requests
2. **Basic Auth**: Use username and API key for package sources
3. **NuGet.Config**: Store credentials in configuration files

## Troubleshooting

### Common Issues

**Symbol Package Upload Fails with dotnet CLI**
- This is a known limitation. Use curl or HTTP client instead.

**Package Not Found After Upload**
- Verify API key permissions
- Check that the package was uploaded to the correct feed
- Ensure proper authentication is configured

**Authentication Errors**
- Verify API key is correct and active
- Check that the key has appropriate permissions
- Ensure the source URL includes authentication

**Symbol Debugging Not Working**
- Verify symbol package was uploaded successfully
- Check that symbol server URL is configured correctly
- Ensure debugging tools are configured to use the symbol server

### Logs and Debugging

Check Lodestone logs for detailed error information:

```bash
# View API gateway logs
docker logs lodestone-api-gateway

# View specific service logs
docker logs lodestone-registry
```

## Best Practices

1. **Version Management**: Use semantic versioning (SemVer) for consistent package versioning
2. **Symbol Packages**: Always publish symbol packages for libraries to enable debugging
3. **Metadata**: Include comprehensive package metadata (description, authors, tags)
4. **Security**: Use separate API keys for different projects or environments
5. **CI/CD**: Automate package publishing in your build pipelines
6. **Testing**: Test packages in staging environments before publishing to production feeds

## Example Project Setup

Here's a complete example of a project configured for Lodestone:

```xml
<!-- MyProject.csproj -->
<Project Sdk="Microsoft.NET.Sdk">

  <PropertyGroup>
    <TargetFramework>net8.0</TargetFramework>
    <PackageId>MyCompany.MyProject</PackageId>
    <Version>1.0.0</Version>
    <Authors>Your Name</Authors>
    <Description>A sample package for Lodestone</Description>
    <PackageTags>sample;lodestone;nuget</PackageTags>
    
    <!-- Symbol package configuration -->
    <IncludeSymbols>true</IncludeSymbols>
    <SymbolPackageFormat>snupkg</SymbolPackageFormat>
    <PublishRepositoryUrl>true</PublishRepositoryUrl>
    <EmbedUntrackedSources>true</EmbedUntrackedSources>
    <DebugType>embedded</DebugType>
  </PropertyGroup>

  <ItemGroup>
    <PackageReference Include="Microsoft.SourceLink.GitHub" Version="1.1.1" PrivateAssets="All"/>
  </ItemGroup>

</Project>
```

```bash
#!/bin/bash
# publish.sh - Build and publish script

# Build and pack
dotnet pack -c Release

# Publish regular package
dotnet nuget push ./bin/Release/MyCompany.MyProject.1.0.0.nupkg \
    --source "Lodestone" \
    --api-key "$NUGET_API_KEY"

# Publish symbol package
curl -X PUT "http://localhost:8080/api/v1/nuget/v2/symbolpackage" \
    -H "X-NuGet-ApiKey: $NUGET_API_KEY" \
    -H "Content-Type: application/octet-stream" \
    --data-binary @"./bin/Release/MyCompany.MyProject.1.0.0.snupkg"
```

This comprehensive setup ensures your packages are properly configured for both consumption and debugging in Lodestone.
