#!/bin/bash
set -e

# Health check script for Lodestone deployment
# This script verifies that all services are running and healthy

echo "🏥 Lodestone Health Check"
echo "========================"

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to check if a URL is responding
check_url() {
    local url=$1
    local service_name=$2
    local expected_code=${3:-200}
    
    echo -n "Checking $service_name... "
    
    if command -v curl >/dev/null 2>&1; then
        response=$(curl -s -o /dev/null -w "%{http_code}" "$url" 2>/dev/null || echo "000")
    elif command -v wget >/dev/null 2>&1; then
        response=$(wget --spider -S "$url" 2>&1 | grep "HTTP/" | awk '{print $2}' | tail -1 || echo "000")
    else
        echo -e "${RED}✗ No curl or wget available${NC}"
        return 1
    fi
    
    if [ "$response" = "$expected_code" ]; then
        echo -e "${GREEN}✓ OK (HTTP $response)${NC}"
        return 0
    else
        echo -e "${RED}✗ Failed (HTTP $response)${NC}"
        return 1
    fi
}

# Function to check Docker service status
check_docker_service() {
    local service_name=$1
    echo -n "Checking Docker service $service_name... "
    
    if docker-compose ps "$service_name" | grep -q "Up"; then
        echo -e "${GREEN}✓ Running${NC}"
        return 0
    else
        echo -e "${RED}✗ Not running${NC}"
        return 1
    fi
}

# Function to check database connectivity
check_database() {
    echo -n "Checking PostgreSQL connectivity... "
    
    if docker-compose exec -T postgres pg_isready -U lodestone >/dev/null 2>&1; then
        echo -e "${GREEN}✓ Connected${NC}"
        return 0
    else
        echo -e "${RED}✗ Connection failed${NC}"
        return 1
    fi
}

# Function to check Redis connectivity
check_redis() {
    echo -n "Checking Redis connectivity... "
    
    if docker-compose exec -T redis redis-cli ping >/dev/null 2>&1; then
        echo -e "${GREEN}✓ Connected${NC}"
        return 0
    else
        echo -e "${RED}✗ Connection failed${NC}"
        return 1
    fi
}

# Main health check
main() {
    local overall_status=0
    
    echo "📊 Service Status"
    echo "-----------------"
    
    # Check Docker services
    check_docker_service "postgres" || overall_status=1
    check_docker_service "redis" || overall_status=1
    check_docker_service "api-gateway" || overall_status=1
    
    # Check if nginx is running (production mode)
    if docker-compose ps nginx 2>/dev/null | grep -q "Up"; then
        check_docker_service "nginx" || overall_status=1
    fi
    
    echo ""
    echo "🔌 Connectivity Tests"
    echo "--------------------"
    
    # Check database and Redis connectivity
    check_database || overall_status=1
    check_redis || overall_status=1
    
    echo ""
    echo "🌐 HTTP Endpoint Tests"
    echo "---------------------"
    
    # Wait a moment for services to be ready
    sleep 2
    
    # Check API Gateway health endpoint
    check_url "http://localhost:8080/health" "API Gateway Health" || overall_status=1
    
    # Check if Nginx is running and test endpoints
    if docker-compose ps nginx 2>/dev/null | grep -q "Up"; then
        check_url "http://localhost:80/health" "Nginx -> API Gateway" || overall_status=1
    fi
    
    # Test basic API endpoints
    check_url "http://localhost:8080/api/v1/auth/health" "Auth Service" || overall_status=1
    
    echo ""
    echo "🏗️  Registry Endpoints"
    echo "---------------------"
    
    # Test registry endpoints (these might return 404 but should be reachable)
    check_url "http://localhost:8080/npm/" "NPM Registry" "404" || overall_status=1
    check_url "http://localhost:8080/nuget/" "NuGet Registry" "404" || overall_status=1
    check_url "http://localhost:8080/maven2/" "Maven Registry" "404" || overall_status=1
    
    echo ""
    echo "📈 Resource Usage"
    echo "----------------"
    
    # Show container resource usage
    if command -v docker >/dev/null 2>&1; then
        echo "Container resource usage:"
        docker stats --no-stream --format "table {{.Name}}\t{{.CPUPerc}}\t{{.MemUsage}}" $(docker-compose ps -q) 2>/dev/null || echo "Unable to get stats"
    fi
    
    echo ""
    echo "📝 Summary"
    echo "----------"
    
    if [ $overall_status -eq 0 ]; then
        echo -e "${GREEN}✅ All health checks passed! Lodestone is running properly.${NC}"
        echo ""
        echo "Available endpoints:"
        echo "  • API Gateway: http://localhost:8080"
        echo "  • Health Check: http://localhost:8080/health"
        echo "  • NPM Registry: http://localhost:8080/npm/"
        echo "  • NuGet Registry: http://localhost:8080/nuget/"
        echo "  • Maven Registry: http://localhost:8080/maven2/"
        
        if docker-compose ps nginx 2>/dev/null | grep -q "Up"; then
            echo "  • Nginx Proxy: http://localhost:80"
        fi
    else
        echo -e "${RED}❌ Some health checks failed. Please check the logs:${NC}"
        echo "  docker-compose logs -f"
        echo ""
        echo -e "${YELLOW}Common troubleshooting steps:${NC}"
        echo "  1. Check if all services are running: docker-compose ps"
        echo "  2. Restart services: docker-compose restart"
        echo "  3. Check environment variables in .env file"
        echo "  4. Verify database migrations: make db-migrate"
    fi
    
    exit $overall_status
}

# Run health check
main "$@"
