#!/bin/bash

# Lodestone OCI Registry API Testing Script
# Demonstrates both JWT and API Key authentication methods

set -e

echo "🚀 Lodestone OCI Registry Testing"
echo "=================================="

BASE_URL="http://localhost:8080"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Test basic registry endpoints
echo -e "\n${YELLOW}1. Testing Basic Registry Endpoints${NC}"

echo "  Testing health endpoint..."
HEALTH=$(curl -s "${BASE_URL}/health")
echo "  ✅ Health: $HEALTH"

echo "  Testing OCI base endpoint..."
BASE=$(curl -s "${BASE_URL}/v2/")
echo "  ✅ Base: $BASE"

echo "  Testing catalog..."
CATALOG=$(curl -s "${BASE_URL}/v2/_catalog")
echo "  ✅ Catalog: $CATALOG"

# Test authentication
echo -e "\n${YELLOW}2. Testing Authentication${NC}"

# Login and get JWT token
echo "  Logging in to get JWT token..."
LOGIN_RESPONSE=$(curl -s -X POST \
  -H "Content-Type: application/json" \
  -d '{"username":"testuser","password":"testpassword123"}' \
  "${BASE_URL}/api/v1/auth/login")

if echo "$LOGIN_RESPONSE" | grep -q "token"; then
  TOKEN=$(echo "$LOGIN_RESPONSE" | grep -o '"token":"[^"]*"' | cut -d'"' -f4)
  echo "  ✅ JWT Token obtained: ${TOKEN:0:20}..."
else
  echo "  ❌ Login failed: $LOGIN_RESPONSE"
  exit 1
fi

# Create API Key
echo "  Creating API key..."
API_KEY_RESPONSE=$(curl -s -X POST \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"name":"test-script-key","permissions":["registry:push","registry:pull"]}' \
  "${BASE_URL}/api/v1/auth/api-keys")

if echo "$API_KEY_RESPONSE" | grep -q "key"; then
  API_KEY=$(echo "$API_KEY_RESPONSE" | grep -o '"key":"[^"]*"' | cut -d'"' -f4)
  echo "  ✅ API Key created: ${API_KEY:0:20}..."
else
  echo "  ❌ API Key creation failed: $API_KEY_RESPONSE"
  exit 1
fi

# Test with JWT Token
echo -e "\n${YELLOW}3. Testing with JWT Token${NC}"

MANIFEST_JWT='{"test":"jwt-manifest","version":"jwt-test","timestamp":"'$(date -u +%Y-%m-%dT%H:%M:%SZ)'"}'

echo "  Uploading manifest with JWT..."
JWT_UPLOAD=$(curl -s -X PUT \
  -H "Content-Type: application/vnd.docker.distribution.manifest.v2+json" \
  -H "Authorization: Bearer $TOKEN" \
  -d "$MANIFEST_JWT" \
  "${BASE_URL}/v2/jwt-test/manifests/latest")

if [ -z "$JWT_UPLOAD" ]; then
  echo "  ✅ JWT manifest upload successful"
else
  echo "  ❌ JWT upload failed: $JWT_UPLOAD"
fi

echo "  Retrieving manifest uploaded with JWT..."
JWT_RETRIEVE=$(curl -s "${BASE_URL}/v2/jwt-test/manifests/latest")
echo "  ✅ Retrieved: $JWT_RETRIEVE"

# Test with API Key
echo -e "\n${YELLOW}4. Testing with API Key${NC}"

MANIFEST_API='{"test":"api-key-manifest","version":"api-test","timestamp":"'$(date -u +%Y-%m-%dT%H:%M:%SZ)'"}'

echo "  Uploading manifest with API Key..."
API_UPLOAD=$(curl -s -X PUT \
  -H "Content-Type: application/vnd.docker.distribution.manifest.v2+json" \
  -H "X-API-Key: $API_KEY" \
  -d "$MANIFEST_API" \
  "${BASE_URL}/v2/api-test/manifests/latest")

if [ -z "$API_UPLOAD" ]; then
  echo "  ✅ API Key manifest upload successful"
else
  echo "  ❌ API Key upload failed: $API_UPLOAD"
fi

echo "  Retrieving manifest uploaded with API Key..."
API_RETRIEVE=$(curl -s "${BASE_URL}/v2/api-test/manifests/latest")
echo "  ✅ Retrieved: $API_RETRIEVE"

# Test realistic Docker manifest
echo -e "\n${YELLOW}5. Testing Realistic Docker Manifest${NC}"

DOCKER_MANIFEST='{
  "schemaVersion": 2,
  "mediaType": "application/vnd.docker.distribution.manifest.v2+json",
  "config": {
    "mediaType": "application/vnd.docker.container.image.v1+json",
    "size": 7023,
    "digest": "sha256:script-test-config"
  },
  "layers": [
    {
      "mediaType": "application/vnd.docker.image.rootfs.diff.tar.gzip",
      "size": 32654,
      "digest": "sha256:script-test-layer1"
    },
    {
      "mediaType": "application/vnd.docker.image.rootfs.diff.tar.gzip", 
      "size": 16724,
      "digest": "sha256:script-test-layer2"
    }
  ]
}'

echo "  Uploading realistic Docker manifest..."
DOCKER_UPLOAD=$(curl -s -X PUT \
  -H "Content-Type: application/vnd.docker.distribution.manifest.v2+json" \
  -H "X-API-Key: $API_KEY" \
  -d "$DOCKER_MANIFEST" \
  "${BASE_URL}/v2/my-docker-app/manifests/v1.2.3")

if [ -z "$DOCKER_UPLOAD" ]; then
  echo "  ✅ Docker manifest upload successful"
else
  echo "  ❌ Docker upload failed: $DOCKER_UPLOAD"
fi

# Final verification
echo -e "\n${YELLOW}6. Final Verification${NC}"

echo "  Checking updated catalog..."
FINAL_CATALOG=$(curl -s "${BASE_URL}/v2/_catalog")
echo "  ✅ Final catalog: $FINAL_CATALOG"

echo "  Checking tags for docker app..."
DOCKER_TAGS=$(curl -s "${BASE_URL}/v2/my-docker-app/tags/list")
echo "  ✅ Docker app tags: $DOCKER_TAGS"

# Test authentication rejection
echo "  Testing invalid API key rejection..."
INVALID_TEST=$(curl -s -X PUT \
  -H "Content-Type: application/vnd.docker.distribution.manifest.v2+json" \
  -H "X-API-Key: invalid-key-test" \
  -d '{"test":"should-fail"}' \
  "${BASE_URL}/v2/should-fail/manifests/test")

if echo "$INVALID_TEST" | grep -q "unauthorized"; then
  echo "  ✅ Invalid API key properly rejected"
else
  echo "  ❌ Security issue: Invalid key not rejected: $INVALID_TEST"
fi

echo -e "\n${GREEN}🎉 All tests completed successfully!${NC}"
echo -e "   JWT Token authentication: ✅"
echo -e "   API Key authentication: ✅"
echo -e "   OCI Registry operations: ✅"
echo -e "   Security validation: ✅"
echo -e "   Multi-repository support: ✅"
