# Lodestone NuGet Test Package

This directory contains a sample .NET library and console application for testing NuGet package functionality with the Lodestone registry.

## Structure

```
test/nuget-sample/
├── TestLibrary/              # The library that gets packaged
│   ├── TestLibrary.csproj    # Project file with package configuration
│   ├── Calculator.cs         # Sample calculator class
│   └── StringUtils.cs        # Sample string utility class
├── TestConsole/              # Console app that uses the library
│   ├── TestConsole.csproj    # Console project file
│   └── Program.cs            # Main program
├── TestLibrary.sln           # Solution file
├── test-nuget-packages.sh    # Build and test script
└── README.md                 # This file
```

## Features

The test library includes:
- **Calculator class**: Basic arithmetic operations with logging
- **StringUtils class**: String manipulation utilities
- **Symbol generation**: Configured to generate .snupkg symbol packages
- **Proper metadata**: Package ID, version, authors, description, etc.

## Building Packages

### Prerequisites

- .NET 6.0 SDK or later
- (Optional) Running Lodestone registry for upload testing

### Build Only

To build the packages without testing against a registry:

```bash
./test-nuget-packages.sh
```

This will:
1. Build the solution
2. Generate `.nupkg` and `.snupkg` packages
3. Run the test console application
4. Place packages in the `build/` directory

### Build and Test with Registry

To build and test against a running Lodestone registry:

```bash
# First, start the Lodestone registry
cd ../..
make dev-up

# Then build and test packages
cd test/nuget-sample
./test-nuget-packages.sh --with-registry
```

This will additionally:
1. Configure NuGet to use the local registry
2. Attempt to push both regular and symbol packages
3. Test package download functionality

## Manual Testing

You can also manually test with standard NuGet CLI commands:

```bash
# Build packages
cd TestLibrary
dotnet pack -c Release -o ../build --include-symbols

# Configure source (adjust URL as needed)
dotnet nuget add source http://localhost:8080/api/v2 --name "lodestone-local"

# Push regular package
dotnet nuget push build/Lodestone.TestLibrary.1.0.0.nupkg --source "lodestone-local"

# Push symbol package
dotnet nuget push build/Lodestone.TestLibrary.1.0.0.snupkg --source "http://localhost:8080/api/v2/symbolpackage"

# Test installation in a new project
mkdir /tmp/test-install && cd /tmp/test-install
dotnet new console
dotnet add package Lodestone.TestLibrary --source "lodestone-local"
```

## Expected Outputs

### Regular Package (.nupkg)
- Contains compiled assemblies
- Includes package metadata
- Standard NuGet package format

### Symbol Package (.snupkg)
- Contains PDB symbol files
- Enables debugging support
- Separate from regular package

### Console Output
The test console should display operations from both Calculator and StringUtils classes, demonstrating that the library works correctly.

## Troubleshooting

### Common Issues

1. **Build failures**: Ensure .NET 6.0+ SDK is installed
2. **Push failures**: Verify Lodestone registry is running and accessible
3. **Authentication errors**: Check if API key is required for your registry setup
4. **Package not found**: Verify the registry URL and source configuration

### Debugging

- Check package contents: `unzip -l build/Lodestone.TestLibrary.1.0.0.nupkg`
- View symbol files: `unzip -l build/Lodestone.TestLibrary.1.0.0.snupkg`
- Verify NuGet sources: `dotnet nuget list source`
- Test registry connectivity: `curl http://localhost:8080/api/v2`

## Testing Symbol Support

The generated `.snupkg` file contains debugging symbols that should:
1. Be detected as a symbol package by Lodestone
2. Be stored in the symbols path: `nuget/symbols/lodestone.testlibrary/1.0.0/`
3. Be downloadable via the symbols endpoint
4. Enable debugging when the package is consumed

This tests the complete symbol package workflow implemented in Lodestone's NuGet registry.
