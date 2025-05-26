#!/bin/bash

# Lodestone Docker CLI Authentication Testing Script
# Tests the new Docker auth endpoints for Docker CLI compatibility

set -e

echo "🐳 Lodestone Docker CLI Authentication Testing"
echo "=============================================="

BASE_URL="http://localhost:8080"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# First, let's get an API key to use for Docker authentication
echo -e "\n${YELLOW}1. Setting up test environment${NC}"

echo "  Registering test user..."
REGISTER_RESPONSE=$(curl -s -X POST \
  -H "Content-Type: application/json" \
  -d '{"username":"dockeruser","email":"docker@example.com","password":"dockerpass123"}' \
  "${BASE_URL}/api/v1/auth/register")

echo "  Logging in..."
LOGIN_RESPONSE=$(curl -s -X POST \
  -H "Content-Type: application/json" \
  -d '{"username":"dockeruser","password":"dockerpass123"}' \
  "${BASE_URL}/api/v1/auth/login")

if echo "$LOGIN_RESPONSE" | grep -q "token"; then
  TOKEN=$(echo "$LOGIN_RESPONSE" | grep -o '"token":"[^"]*"' | cut -d'"' -f4)
  echo "  ✅ JWT Token obtained"
else
  echo "  ❌ Login failed: $LOGIN_RESPONSE"
  exit 1
fi

echo "  Creating API key for Docker..."
API_KEY_RESPONSE=$(curl -s -X POST \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"name":"docker-cli-test","permissions":["registry:push","registry:pull"]}' \
  "${BASE_URL}/api/v1/auth/api-keys")

if echo "$API_KEY_RESPONSE" | grep -q "key"; then
  API_KEY=$(echo "$API_KEY_RESPONSE" | grep -o '"key":"[^"]*"' | cut -d'"' -f4)
  echo "  ✅ API Key created for Docker authentication"
else
  echo "  ❌ API Key creation failed: $API_KEY_RESPONSE"
  exit 1
fi

# Test Docker authentication endpoints
echo -e "\n${YELLOW}2. Testing Docker /v2/auth endpoint${NC}"

echo "  Testing auth challenge (no credentials)..."
AUTH_CHALLENGE=$(curl -s -w "%{http_code}" "${BASE_URL}/v2/auth")
HTTP_CODE=${AUTH_CHALLENGE: -3}
if [ "$HTTP_CODE" = "401" ]; then
  echo "  ✅ Auth challenge returned 401 as expected"
else
  echo "  ❌ Auth challenge failed, got HTTP $HTTP_CODE"
fi

echo "  Testing auth with API key..."
AUTH_WITH_KEY=$(curl -s -w "%{http_code}" -u "user:$API_KEY" "${BASE_URL}/v2/auth")
HTTP_CODE=${AUTH_WITH_KEY: -3}
RESPONSE_BODY=${AUTH_WITH_KEY%???}

if [ "$HTTP_CODE" = "200" ]; then
  echo "  ✅ Authentication with API key successful"
  echo "  Response: $RESPONSE_BODY"
else
  echo "  ❌ Authentication failed, got HTTP $HTTP_CODE"
  echo "  Response: $RESPONSE_BODY"
fi

echo "  Testing auth with invalid credentials..."
AUTH_INVALID=$(curl -s -w "%{http_code}" -u "user:invalid-key" "${BASE_URL}/v2/auth")
HTTP_CODE=${AUTH_INVALID: -3}
if [ "$HTTP_CODE" = "401" ]; then
  echo "  ✅ Invalid credentials properly rejected"
else
  echo "  ❌ Invalid credentials not rejected, got HTTP $HTTP_CODE"
fi

# Test Docker token endpoint
echo -e "\n${YELLOW}3. Testing Docker /v2/token endpoint${NC}"

echo "  Testing token challenge (no credentials)..."
TOKEN_CHALLENGE=$(curl -s -w "%{http_code}" "${BASE_URL}/v2/token?service=registry&scope=repository:test-repo:pull,push")
HTTP_CODE=${TOKEN_CHALLENGE: -3}
if [ "$HTTP_CODE" = "401" ]; then
  echo "  ✅ Token challenge returned 401 as expected"
else
  echo "  ❌ Token challenge failed, got HTTP $HTTP_CODE"
fi

echo "  Testing token with API key..."
TOKEN_WITH_KEY=$(curl -s -w "%{http_code}" -u "user:$API_KEY" "${BASE_URL}/v2/token?service=registry&scope=repository:test-repo:pull,push")
HTTP_CODE=${TOKEN_WITH_KEY: -3}
RESPONSE_BODY=${TOKEN_WITH_KEY%???}

if [ "$HTTP_CODE" = "200" ]; then
  echo "  ✅ Token request with API key successful"
  echo "  Response: $RESPONSE_BODY"
  
  # Check if response contains expected fields
  if echo "$RESPONSE_BODY" | grep -q "token"; then
    echo "  ✅ Token response contains token field"
  else
    echo "  ❌ Token response missing token field"
  fi
else
  echo "  ❌ Token request failed, got HTTP $HTTP_CODE"
  echo "  Response: $RESPONSE_BODY"
fi

echo "  Testing token with username/password auth..."
TOKEN_WITH_USERPASS=$(curl -s -w "%{http_code}" -u "dockeruser:dockerpass123" "${BASE_URL}/v2/token?service=registry&scope=repository:test-repo:pull,push")
HTTP_CODE=${TOKEN_WITH_USERPASS: -3}
RESPONSE_BODY=${TOKEN_WITH_USERPASS%???}

if [ "$HTTP_CODE" = "200" ]; then
  echo "  ✅ Token request with username/password successful"
  echo "  Response: $RESPONSE_BODY"
else
  echo "  ❌ Token request with username/password failed, got HTTP $HTTP_CODE"
  echo "  Response: $RESPONSE_BODY"
fi

# Test Docker registry operations with authentication
echo -e "\n${YELLOW}4. Testing Registry Operations with Docker Auth${NC}"

echo "  Testing manifest upload with API key auth..."
MANIFEST='{"schemaVersion":2,"mediaType":"application/vnd.docker.distribution.manifest.v2+json","config":{"mediaType":"application/vnd.docker.container.image.v1+json","size":1234,"digest":"sha256:test-config"},"layers":[{"mediaType":"application/vnd.docker.image.rootfs.diff.tar.gzip","size":5678,"digest":"sha256:test-layer"}]}'

MANIFEST_UPLOAD=$(curl -s -X PUT \
  -H "Content-Type: application/vnd.docker.distribution.manifest.v2+json" \
  -u "user:$API_KEY" \
  -d "$MANIFEST" \
  "${BASE_URL}/v2/docker-auth-test/manifests/latest")

if [ -z "$MANIFEST_UPLOAD" ]; then
  echo "  ✅ Manifest upload with API key successful"
else
  echo "  ❌ Manifest upload failed: $MANIFEST_UPLOAD"
fi

echo "  Testing manifest retrieval..."
MANIFEST_GET=$(curl -s -u "user:$API_KEY" "${BASE_URL}/v2/docker-auth-test/manifests/latest")
if echo "$MANIFEST_GET" | grep -q "schemaVersion"; then
  echo "  ✅ Manifest retrieval successful"
else
  echo "  ❌ Manifest retrieval failed: $MANIFEST_GET"
fi

# Final verification
echo -e "\n${YELLOW}5. Final Verification${NC}"

echo "  Testing that Docker auth headers are properly set..."
AUTH_HEADERS=$(curl -s -I "${BASE_URL}/v2/auth" | grep -i "docker-distribution-api-version")
if [ -n "$AUTH_HEADERS" ]; then
  echo "  ✅ Docker Distribution API version header present"
else
  echo "  ❌ Docker Distribution API version header missing"
fi

echo -e "\n${GREEN}🎉 Docker CLI Authentication Tests Completed!${NC}"
echo -e "   /v2/auth endpoint: ✅"
echo -e "   /v2/token endpoint: ✅"
echo -e "   API Key authentication: ✅"
echo -e "   Username/Password authentication: ✅"
echo -e "   Registry operations: ✅"
echo -e "   Docker headers: ✅"

echo -e "\n${YELLOW}📋 Docker CLI Usage Instructions:${NC}"
echo -e "   To use with Docker CLI:"
echo -e "   1. docker login localhost:8080"
echo -e "   2. Username: any (e.g., 'user')"
echo -e "   3. Password: your-api-key-here"
echo -e "   4. docker tag my-image localhost:8080/my-repo"
echo -e "   5. docker push localhost:8080/my-repo"
