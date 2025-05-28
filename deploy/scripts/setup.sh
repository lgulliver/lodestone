#!/bin/bash
# Lodestone Setup Script
# First-time setup for Lodestone deployments

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
ENV_DIR="$PROJECT_ROOT/deploy/environments"

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

# Function to check prerequisites
check_prerequisites() {
    local missing=0
    
    print_info "Checking prerequisites..."
    
    # Check Docker
    if ! command -v docker >/dev/null 2>&1; then
        print_error "Docker is not installed"
        echo "Please install Docker: https://docs.docker.com/get-docker/"
        ((missing++))
    else
        print_success "Docker is installed"
    fi
    
    # Check Docker Compose
    if ! command -v docker-compose >/dev/null 2>&1; then
        print_error "Docker Compose is not installed"
        echo "Please install Docker Compose: https://docs.docker.com/compose/install/"
        ((missing++))
    else
        print_success "Docker Compose is installed"
    fi
    
    # Check if Docker daemon is running
    if ! docker info >/dev/null 2>&1; then
        print_error "Docker daemon is not running"
        echo "Please start Docker daemon"
        ((missing++))
    else
        print_success "Docker daemon is running"
    fi
    
    # Check available disk space (warn if less than 2GB)
    local available_space
    available_space=$(df "$PROJECT_ROOT" | awk 'NR==2 {print $4}')
    local available_gb=$((available_space / 1024 / 1024))
    
    if [ $available_gb -lt 2 ]; then
        print_warning "Low disk space: ${available_gb}GB available (recommend 2GB+)"
    else
        print_success "Sufficient disk space: ${available_gb}GB available"
    fi
    
    return $missing
}

# Function to create default local environment file
create_local_env() {
    local env_file="$1"
    
    print_info "Creating default local environment file..."
    
    cat > "$env_file" << 'EOF'
# Lodestone Local Development Environment
# This file is auto-generated for local development

# Application
ENVIRONMENT=local
LOG_LEVEL=debug
API_PORT=8080
CORS_ORIGINS=http://localhost:3000,http://localhost:8080

# Database
POSTGRES_HOST=localhost
POSTGRES_PORT=5432
POSTGRES_DB=lodestone
POSTGRES_USER=lodestone
POSTGRES_PASSWORD=lodestone
DATABASE_URL=postgres://lodestone:lodestone@localhost:5432/lodestone?sslmode=disable

# Redis
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PASSWORD=
REDIS_DB=0

# Storage (Local filesystem for development)
STORAGE_TYPE=local
LOCAL_STORAGE_PATH=./data/artifacts

# Authentication
JWT_SECRET=local-development-secret-key-not-for-production-use-only
JWT_EXPIRY=24h
API_KEY_PREFIX=lk_

# Metrics and Monitoring
METRICS_ENABLED=true
HEALTH_CHECK_ENABLED=true

# File Upload Limits
MAX_UPLOAD_SIZE=100MB
TEMP_DIR=/tmp/lodestone

# Development Features
DEBUG_MODE=true
ENABLE_SWAGGER=true
ENABLE_PROFILING=true
EOF
    
    print_success "Created local environment file with development defaults"
}

# Function to setup environment
setup_environment() {
    local env="$1"
    local env_file="$PROJECT_ROOT/.env"
    
    print_info "Setting up $env environment..."
    
    # Copy appropriate environment template
    case "$env" in
        "local")
            # Check if template exists, otherwise create default
            if [ -f "$ENV_DIR/.env.local" ]; then
                cp "$ENV_DIR/.env.local" "$env_file"
                print_success "Copied local environment configuration from template"
            else
                create_local_env "$env_file"
            fi
            ;;
        "dev")
            cp "$ENV_DIR/.env.dev" "$env_file"
            print_success "Copied development environment configuration"
            ;;
        "prod")
            cp "$ENV_DIR/.env.prod.template" "$env_file"
            print_warning "Copied production template - YOU MUST EDIT THIS FILE!"
            echo ""
            print_info "Required production configuration changes:"
            echo "  1. Set strong passwords for POSTGRES_PASSWORD and REDIS_PASSWORD"
            echo "  2. Set a secure JWT_SECRET (64+ characters)"
            echo "  3. Configure S3 storage settings (recommended for production)"
            echo "  4. Set your domain name in CORS_ORIGINS"
            echo "  5. Set CERTBOT_EMAIL for SSL certificates"
            echo ""
            print_warning "Edit .env file before starting production deployment!"
            ;;
    esac
    
    print_info "Environment file created: $env_file"
}

# Function to build images
build_images() {
    print_info "Building Lodestone images..."
    
    cd "$PROJECT_ROOT"
    
    # Build the API gateway image
    docker build -f deploy/configs/docker/Dockerfile.api-gateway -t lodestone/api-gateway:latest .
    
    print_success "Images built successfully"
}

# Function to create directories
create_directories() {
    print_info "Creating required directories..."
    
    # SSL certificates directory (for production)
    mkdir -p "$PROJECT_ROOT/ssl/certs"
    mkdir -p "$PROJECT_ROOT/ssl/private"
    
    # Data directories
    mkdir -p "$PROJECT_ROOT/data/postgres"
    mkdir -p "$PROJECT_ROOT/data/redis" 
    mkdir -p "$PROJECT_ROOT/data/artifacts"
    
    print_success "Directories created"
}

# Function to show next steps
show_next_steps() {
    local env="$1"
    
    echo ""
    echo "🎉 Setup completed successfully!"
    echo "================================"
    echo ""
    
    case "$env" in
        "local")
            echo "Next steps for local development:"
            echo "  1. Start the deployment:"
            echo "     ./deploy/scripts/deploy.sh up local"
            echo ""
            echo "  2. Check health:"
            echo "     ./deploy/scripts/health-check.sh"
            echo ""
            echo "  3. Access services:"
            echo "     • API Gateway: http://localhost:8080"
            echo "     • PostgreSQL: localhost:5432 (user: lodestone, password: lodestone)"
            echo "     • Redis: localhost:6379"
            ;;
        "dev")
            echo "Next steps for development:"
            echo "  1. Start the deployment:"
            echo "     ./deploy/scripts/deploy.sh up dev"
            echo ""
            echo "  2. Check health:"
            echo "     ./deploy/scripts/health-check.sh"
            echo ""
            echo "  3. Access services:"
            echo "     • API Gateway: http://localhost:8080"
            echo "     • PostgreSQL: localhost:5432 (user: lodestone, password: password)"
            echo "     • Redis: localhost:6379"
            echo "     • MinIO Console: http://localhost:9001 (minioadmin/minioadmin)"
            ;;
        "prod")
            echo "Next steps for production:"
            echo "  1. IMPORTANT: Edit the .env file with your production settings!"
            echo "     nano .env"
            echo ""
            echo "  2. Configure SSL certificates (recommended):"
            echo "     # Option 1: Let's Encrypt (automatic)"
            echo "     ./deploy/scripts/deploy.sh up prod certbot"
            echo ""
            echo "     # Option 2: Custom certificates"
            echo "     # Copy your certificates to ssl/certs/ and ssl/private/"
            echo ""
            echo "  3. Start the deployment:"
            echo "     ./deploy/scripts/deploy.sh up prod"
            echo ""
            echo "  4. Check health:"
            echo "     ./deploy/scripts/health-check.sh"
            ;;
    esac
    
    echo ""
    echo "📚 Additional resources:"
    echo "  • Deployment guide: docs/DEPLOYMENT.md"
    echo "  • All commands: ./deploy/scripts/deploy.sh help"
    echo "  • Logs: ./deploy/scripts/deploy.sh logs $env"
    echo "  • Stop: ./deploy/scripts/deploy.sh down $env"
}

# Function to show usage
show_usage() {
    cat << EOF
Lodestone Setup Script

Usage: $0 <environment> [options]

Environments:
    local       Local development (minimal setup)
    dev         Development with MinIO S3 simulation  
    prod        Production deployment

Options:
    --skip-build    Skip building Docker images
    --skip-prereq   Skip prerequisite checks
    -h, --help      Show this help message

Examples:
    $0 local                # Setup local development
    $0 dev                  # Setup development environment
    $0 prod                 # Setup production (requires manual .env editing)
    $0 dev --skip-build     # Setup dev without rebuilding images

EOF
}

# Parse command line arguments
ENVIRONMENT=""
SKIP_BUILD=false
SKIP_PREREQ=false

while [[ $# -gt 0 ]]; do
    case $1 in
        local|dev|prod)
            if [ -z "$ENVIRONMENT" ]; then
                ENVIRONMENT="$1"
            else
                print_error "Multiple environments specified"
                exit 1
            fi
            shift
            ;;
        --skip-build)
            SKIP_BUILD=true
            shift
            ;;
        --skip-prereq)
            SKIP_PREREQ=true
            shift
            ;;
        -h|--help)
            show_usage
            exit 0
            ;;
        *)
            print_error "Unknown option: $1"
            show_usage
            exit 1
            ;;
    esac
done

# Validate environment
if [ -z "$ENVIRONMENT" ]; then
    print_error "Environment is required"
    show_usage
    exit 1
fi

case "$ENVIRONMENT" in
    "local"|"dev"|"prod")
        ;;
    *)
        print_error "Invalid environment: $ENVIRONMENT"
        show_usage
        exit 1
        ;;
esac

# Main setup process
echo "🏗️  Lodestone Setup - $ENVIRONMENT Environment"
echo "=============================================="
echo ""

# Check prerequisites
if [ "$SKIP_PREREQ" = false ]; then
    if ! check_prerequisites; then
        print_error "Prerequisites check failed. Please fix the issues above."
        exit 1
    fi
    echo ""
fi

# Create directories
create_directories
echo ""

# Setup environment
setup_environment "$ENVIRONMENT"
echo ""

# Build images
if [ "$SKIP_BUILD" = false ]; then
    build_images
    echo ""
fi

# Show next steps
show_next_steps "$ENVIRONMENT"
