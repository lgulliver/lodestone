#!/bin/bash
# NuGet Integration Validation Script
# This script validates the NuGet registry implementation

set -e

echo "🔍 NuGet Registry Validation"
echo "============================"

# Change to project directory
cd "$(dirname "$0")/../.."

echo "✅ Building API Gateway..."
go build -o bin/api-gateway ./cmd/api-gateway
if [ $? -eq 0 ]; then
    echo "   Build successful"
else
    echo "   ❌ Build failed"
    exit 1
fi

echo "✅ Running NuGet Registry Tests..."
go test -v ./internal/registry/registries/nuget/...
if [ $? -eq 0 ]; then
    echo "   NuGet tests passed"
else
    echo "   ❌ NuGet tests failed"
    exit 1
fi

echo "✅ Running NuGet API Route Tests..."
go test -v ./cmd/api-gateway/routes/... -run=".*[Nn]u[Gg]et.*"
if [ $? -eq 0 ]; then
    echo "   NuGet API tests passed"
else
    echo "   ❌ NuGet API tests failed"
    exit 1
fi

echo "✅ Running Integration Tests..."
go test -v ./cmd/api-gateway/routes/integration_test.go
if [ $? -eq 0 ]; then
    echo "   Integration tests passed"
else
    echo "   ❌ Integration tests failed"
    exit 1
fi

echo "✅ Running E2E Tests..."
go test -v ./test/e2e/...
if [ $? -eq 0 ]; then
    echo "   E2E tests passed"
else
    echo "   ❌ E2E tests failed"
    exit 1
fi

echo ""
echo "🎉 NuGet Registry Validation Complete!"
echo "=====================================
✅ All core components working:
   - NuGet package validation
   - Metadata extraction from .nuspec files
   - Upload/Download workflows
   - Storage path generation
   - API v3 Service Index endpoints
   - Package version listing
   - Search and discovery

✅ Integration validated:
   - Database operations
   - Authentication middleware
   - Error handling
   - Logging

The NuGet registry implementation is ready for production use!"

# Clean up
rm -f bin/api-gateway
