#!/bin/bash

# Script to build and test NuGet packages with Lodestone registry
set -e

echo "=== Lodestone NuGet Package Test Script ==="

# Configuration
REGISTRY_URL="http://localhost:8080"
API_KEY="your-api-key-here"
PROJECT_DIR="/Users/Liam.Gulliver/Repos/lodestone/test/nuget-sample"
BUILD_DIR="$PROJECT_DIR/build"

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if .NET is installed
check_dotnet() {
    if ! command -v dotnet &> /dev/null; then
        log_error ".NET SDK is not installed. Please install .NET 6.0 or later."
        exit 1
    fi
    
    log_info "Using .NET version: $(dotnet --version)"
}

# Clean and create build directory
setup_build_dir() {
    log_info "Setting up build directory..."
    rm -rf "$BUILD_DIR"
    mkdir -p "$BUILD_DIR"
}

# Build the library and generate packages
build_packages() {
    log_info "Building NuGet packages..."
    
    cd "$PROJECT_DIR/TestLibrary"
    
    # Build in Release mode to generate packages with symbols
    dotnet build -c Release -o "$BUILD_DIR"
    dotnet pack -c Release -o "$BUILD_DIR" --include-symbols --include-source
    
    log_info "Packages built successfully!"
    ls -la "$BUILD_DIR"/*.nupkg "$BUILD_DIR"/*.snupkg 2>/dev/null || log_warn "No packages found in build directory"
}

# Test the console application
test_console_app() {
    log_info "Testing console application..."
    
    cd "$PROJECT_DIR/TestConsole"
    dotnet run
}

# Configure NuGet to use local registry
configure_nuget() {
    log_info "Configuring NuGet sources..."
    
    # Remove existing source if it exists
    dotnet nuget remove source "lodestone-local" 2>/dev/null || true
    
    # Add our local registry
    dotnet nuget add source "$REGISTRY_URL/api/v2" --name "lodestone-local"
    
    # List sources to verify
    log_info "Current NuGet sources:"
    dotnet nuget list source
}

# Push packages to registry
push_packages() {
    log_info "Pushing packages to Lodestone registry..."
    
    # Find the regular package
    NUPKG_FILE=$(find "$BUILD_DIR" -name "*.nupkg" ! -name "*.symbols.nupkg" | head -1)
    SNUPKG_FILE=$(find "$BUILD_DIR" -name "*.snupkg" | head -1)
    
    if [ -f "$NUPKG_FILE" ]; then
        log_info "Pushing regular package: $(basename "$NUPKG_FILE")"
        
        # Push regular package
        if [ -n "$API_KEY" ] && [ "$API_KEY" != "your-api-key-here" ]; then
            dotnet nuget push "$NUPKG_FILE" --source "lodestone-local" --api-key "$API_KEY"
        else
            log_warn "No API key provided. Attempting push without authentication..."
            dotnet nuget push "$NUPKG_FILE" --source "lodestone-local" || log_warn "Push failed - this is expected if authentication is required"
        fi
    else
        log_error "No regular package (.nupkg) found in build directory"
        return 1
    fi
    
    if [ -f "$SNUPKG_FILE" ]; then
        log_info "Pushing symbol package: $(basename "$SNUPKG_FILE")"
        
        # Push symbol package to symbol endpoint
        SYMBOL_URL="$REGISTRY_URL/api/v2/symbolpackage"
        
        if [ -n "$API_KEY" ] && [ "$API_KEY" != "your-api-key-here" ]; then
            dotnet nuget push "$SNUPKG_FILE" --source "$SYMBOL_URL" --api-key "$API_KEY"
        else
            log_warn "No API key provided. Attempting symbol push without authentication..."
            dotnet nuget push "$SNUPKG_FILE" --source "$SYMBOL_URL" || log_warn "Symbol push failed - this is expected if authentication is required"
        fi
    else
        log_warn "No symbol package (.snupkg) found in build directory"
    fi
}

# Test package download
test_package_download() {
    log_info "Testing package download..."
    
    # Create a temporary test project
    TEMP_TEST_DIR="/tmp/lodestone-nuget-test"
    rm -rf "$TEMP_TEST_DIR"
    mkdir -p "$TEMP_TEST_DIR"
    
    cd "$TEMP_TEST_DIR"
    
    # Create a simple console app
    dotnet new console -n "DownloadTest"
    cd "DownloadTest"
    
    # Try to add our package
    log_info "Attempting to install package from registry..."
    dotnet add package Lodestone.TestLibrary --source "lodestone-local" || log_warn "Package install failed - this is expected if the registry is not running"
    
    # Clean up
    rm -rf "$TEMP_TEST_DIR"
}

# Main execution
main() {
    log_info "Starting NuGet package test workflow..."
    
    check_dotnet
    setup_build_dir
    build_packages
    test_console_app
    
    # Only configure and test registry if requested
    if [ "$1" == "--with-registry" ]; then
        configure_nuget
        push_packages
        test_package_download
    else
        log_info "Skipping registry tests. Use '--with-registry' flag to test against running registry."
    fi
    
    log_info "Test workflow completed!"
    log_info "Built packages are available in: $BUILD_DIR"
    log_info ""
    log_info "To test with registry:"
    log_info "1. Start Lodestone registry: make dev-up"
    log_info "2. Run this script with: ./test-nuget-packages.sh --with-registry"
}

# Run main function with all arguments
main "$@"
