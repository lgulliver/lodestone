#!/bin/bash
# Lodestone Health Check Script
# Comprehensive health monitoring for all deployment environments

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

# Default values
ENVIRONMENT="auto"
VERBOSE=false

# Function to print colored output
print_info() {
    echo -e "${BLUE}ℹ️  $1${NC}"
}

print_success() {
    echo -e "${GREEN}✅ $1${NC}"
}

print_warning() {
    echo -e "${YELLOW}⚠️  $1${NC}"
}

print_error() {
    echo -e "${RED}❌ $1${NC}"
}

# Function to detect current environment
detect_environment() {
    cd "$PROJECT_ROOT"
    
    # Check which compose files are being used
    if docker-compose -p lodestone -f deploy/compose/docker-compose.yml -f deploy/compose/docker-compose.prod.yml ps >/dev/null 2>&1; then
        if docker-compose -p lodestone -f deploy/compose/docker-compose.yml -f deploy/compose/docker-compose.prod.yml ps | grep -q "nginx"; then
            echo "prod"
            return
        fi
    fi
    
    if docker-compose -p lodestone -f deploy/compose/docker-compose.yml -f deploy/compose/docker-compose.dev.yml ps >/dev/null 2>&1; then
        if docker-compose -p lodestone -f deploy/compose/docker-compose.yml -f deploy/compose/docker-compose.dev.yml ps | grep -q "minio"; then
            echo "dev"
            return
        fi
    fi
    
    if docker-compose -p lodestone -f deploy/compose/docker-compose.yml ps >/dev/null 2>&1; then
        echo "local"
        return
    fi
    
    echo "none"
}

# Function to check service health
check_service_health() {
    local service="$1"
    local url="$2"
    local expected_status="${3:-200}"
    
    if [ "$VERBOSE" = true ]; then
        print_info "Checking $service at $url"
    fi
    
    local response
    response=$(curl -s -o /dev/null -w "%{http_code}" "$url" 2>/dev/null || echo "000")
    
    if [ "$response" = "$expected_status" ]; then
        print_success "$service is healthy (HTTP $response)"
        return 0
    else
        print_error "$service is unhealthy (HTTP $response, expected $expected_status)"
        return 1
    fi
}

# Function to check database connectivity
check_database() {
    local env="$1"
    
    if [ "$VERBOSE" = true ]; then
        print_info "Checking PostgreSQL database connectivity"
    fi
    
    # Try to connect to database through the API health endpoint
    if check_service_health "Database (via API)" "http://localhost:8080/health" 200; then
        return 0
    fi
    
    # Direct database check if API is down
    if command -v psql >/dev/null 2>&1; then
        local db_host="localhost"
        local db_port="5432"
        local db_user="lodestone"
        local db_name="lodestone"
        
        if [ "$env" != "prod" ]; then
            if PGPASSWORD=password psql -h "$db_host" -p "$db_port" -U "$db_user" -d "$db_name" -c "SELECT 1;" >/dev/null 2>&1; then
                print_success "Database is accessible directly"
                return 0
            fi
        fi
    fi
    
    print_error "Database connectivity check failed"
    return 1
}

# Function to check Redis connectivity  
check_redis() {
    local env="$1"
    
    if [ "$VERBOSE" = true ]; then
        print_info "Checking Redis connectivity"
    fi
    
    if [ "$env" != "prod" ] && command -v redis-cli >/dev/null 2>&1; then
        if redis-cli -h localhost -p 6379 ping >/dev/null 2>&1; then
            print_success "Redis is accessible"
            return 0
        fi
    fi
    
    # Check through API health endpoint
    if curl -s http://localhost:8080/health | grep -q "redis.*healthy" 2>/dev/null; then
        print_success "Redis is healthy (via API)"
        return 0
    fi
    
    print_warning "Redis connectivity check inconclusive"
    return 1
}

# Function to check API endpoints
check_api_endpoints() {
    local env="$1"
    local base_url="http://localhost:8080"
    
    # Core endpoints
    local endpoints=(
        "/health:200"
        "/api/v1/auth/api-keys:401"  # GET requires auth, so returns 401 unauthorized
    )
    
    # Registry endpoints - test actual working endpoints
    local registry_endpoints=(
        "/api/v1/npm/test-package:404"        # npm package lookup
        "/api/v1/nuget/v3/search:400"         # nuget search (no query params)
        "/api/v1/maven/test:404"              # maven path
        "/api/v1/go/test.example.com/@latest:404"  # go module latest
        "/api/v1/helm/index.yaml:404"         # helm index
        "/api/v1/cargo/api/v1/crates:404"     # cargo crates
        "/api/v1/gems/api/v1/gems:404"        # gems search
        "/api/v1/opa/policies:404"            # opa policies (if exists)
    )
    
    local failed=0
    
    print_info "Checking core API endpoints..."
    for endpoint_spec in "${endpoints[@]}"; do
        local endpoint="${endpoint_spec%:*}"
        local expected="${endpoint_spec#*:}"
        
        if ! check_service_health "API${endpoint}" "${base_url}${endpoint}" "$expected"; then
            ((failed++))
        fi
    done
    
    print_info "Checking registry endpoints..."
    for endpoint_spec in "${registry_endpoints[@]}"; do
        local endpoint="${endpoint_spec%:*}"
        local expected="${endpoint_spec#*:}"
        
        local response
        response=$(curl -s -o /dev/null -w "%{http_code}" "${base_url}${endpoint}" 2>/dev/null || echo "000")
        
        # Accept both expected response and 200 (working endpoint), plus 400 for maven (invalid path format)
        if [ "$response" = "$expected" ] || [ "$response" = "200" ] || [ "$response" = "400" ]; then
            # Expected response code, 200, or 400 means the route exists and is working
            if [ "$VERBOSE" = true ]; then
                print_success "Registry endpoint $endpoint is responding correctly (HTTP $response)"
            fi
        else
            print_warning "Registry endpoint $endpoint unexpected response (HTTP $response, expected $expected, 200, or 400)"
            ((failed++))
        fi
    done
    
    if [ $failed -eq 0 ]; then
        print_success "All API endpoints are healthy"
        return 0
    else
        print_error "$failed API endpoints failed health checks"
        return 1
    fi
}

# Function to check container status
check_containers() {
    local env="$1"
    
    print_info "Checking container status for $env environment..."
    
    cd "$PROJECT_ROOT"
    
    local compose_cmd="docker-compose -p lodestone -f deploy/compose/docker-compose.yml"
    case "$env" in
        "dev")
            compose_cmd="$compose_cmd -f deploy/compose/docker-compose.dev.yml"
            ;;
        "prod")
            compose_cmd="$compose_cmd -f deploy/compose/docker-compose.prod.yml"
            ;;
    esac
    
    local failed=0
    
    # Get running containers
    local containers
    containers=$($compose_cmd ps --services 2>/dev/null || echo "")
    
    if [ -z "$containers" ]; then
        print_error "No containers found for $env environment"
        return 1
    fi
    
    for container in $containers; do
        local status
        # Get container status using docker-compose ps format
        status=$($compose_cmd ps --format "table {{.State}}" "$container" 2>/dev/null | tail -n +2 | tr -d '[:space:]' || echo "")
        
        if [[ "$status" == "running" ]] || [[ "$status" == "Up" ]]; then
            if [ "$VERBOSE" = true ]; then
                print_success "Container $container is running"
            fi
        else
            print_error "Container $container is not running (status: $status)"
            ((failed++))
        fi
    done
    
    if [ $failed -eq 0 ]; then
        print_success "All containers are running"
        return 0
    else
        print_error "$failed containers are not running properly"
        return 1
    fi
}

# Function to show system resources
show_resources() {
    if [ "$VERBOSE" = true ]; then
        print_info "System resource usage:"
        
        # Docker stats if available
        if command -v docker >/dev/null 2>&1; then
            echo ""
            docker stats --no-stream --format "table {{.Name}}\t{{.CPUPerc}}\t{{.MemUsage}}" $(docker ps -q) 2>/dev/null || echo "Unable to get container stats"
        fi
        
        # Disk usage
        echo ""
        print_info "Disk usage for Docker volumes:"
        docker system df 2>/dev/null || echo "Unable to get Docker disk usage"
    fi
}

# Main health check function
run_health_check() {
    local env="$1"
    
    echo "🏥 Lodestone Health Check - $(date)"
    echo "========================================"
    echo ""
    
    print_info "Environment: $env"
    echo ""
    
    local overall_status=0
    
    # Check containers
    if ! check_containers "$env"; then
        overall_status=1
    fi
    
    echo ""
    
    # Check database
    if ! check_database "$env"; then
        overall_status=1
    fi
    
    # Check Redis
    if ! check_redis "$env"; then
        # Redis failure is not critical, just warning
        print_warning "Redis check failed, but this may not be critical"
    fi
    
    echo ""
    
    # Check API endpoints
    if ! check_api_endpoints "$env"; then
        overall_status=1
    fi
    
    echo ""
    
    # Show resources if verbose
    show_resources
    
    echo ""
    echo "📝 Summary"
    echo "----------"
    
    if [ $overall_status -eq 0 ]; then
        print_success "All health checks passed! Lodestone is running properly."
        echo ""
        echo "Available endpoints:"
        echo "  • API Gateway: http://localhost:8080"
        echo "  • Health Check: http://localhost:8080/health"
        echo "  • NPM Registry: http://localhost:8080/api/v1/npm/"
        echo "  • NuGet Registry: http://localhost:8080/api/v1/nuget/"
        echo "  • Maven Registry: http://localhost:8080/api/v1/maven/"
        echo "  • Go Registry: http://localhost:8080/api/v1/go/"
        echo "  • Helm Registry: http://localhost:8080/api/v1/helm/"
        echo "  • Cargo Registry: http://localhost:8080/api/v1/cargo/"
        echo "  • RubyGems Registry: http://localhost:8080/api/v1/gems/"
        echo "  • OPA Registry: http://localhost:8080/api/v1/opa/"
        
        if [ "$env" = "prod" ]; then
            echo "  • Nginx Proxy: http://localhost:80"
        elif [ "$env" = "dev" ]; then
            echo "  • MinIO Console: http://localhost:9001"
        fi
        
        echo ""
        print_info "Use './deploy/scripts/deploy.sh logs $env' to view service logs"
        
    else
        print_error "Some health checks failed. Please check the logs for more details."
        echo ""
        print_info "Troubleshooting commands:"
        echo "  • View logs: ./deploy/scripts/deploy.sh logs $env"
        echo "  • Check containers: ./deploy/scripts/deploy.sh ps $env"
        echo "  • Restart services: ./deploy/scripts/deploy.sh restart $env"
    fi
    
    return $overall_status
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -e|--environment)
            ENVIRONMENT="$2"
            shift 2
            ;;
        -v|--verbose)
            VERBOSE=true
            shift
            ;;
        -h|--help)
            echo "Lodestone Health Check"
            echo ""
            echo "Usage: $0 [options]"
            echo ""
            echo "Options:"
            echo "  -e, --environment ENV    Specify environment (local/dev/prod/auto)"
            echo "  -v, --verbose           Show detailed output"
            echo "  -h, --help              Show this help message"
            echo ""
            echo "Examples:"
            echo "  $0                      # Auto-detect environment"
            echo "  $0 -e dev -v           # Check dev environment with verbose output"
            echo "  $0 --environment prod   # Check production environment"
            exit 0
            ;;
        *)
            print_error "Unknown option: $1"
            exit 1
            ;;
    esac
done

# Auto-detect environment if not specified
if [ "$ENVIRONMENT" = "auto" ]; then
    ENVIRONMENT=$(detect_environment)
    if [ "$ENVIRONMENT" = "none" ]; then
        print_error "No Lodestone deployment detected"
        print_info "Start a deployment first: ./deploy/scripts/deploy.sh up <environment>"
        exit 1
    fi
    print_info "Auto-detected environment: $ENVIRONMENT"
fi

# Validate environment
case "$ENVIRONMENT" in
    "local"|"dev"|"prod")
        ;;
    *)
        print_error "Invalid environment: $ENVIRONMENT"
        print_info "Valid environments: local, dev, prod, auto"
        exit 1
        ;;
esac

# Run the health check
run_health_check "$ENVIRONMENT"
