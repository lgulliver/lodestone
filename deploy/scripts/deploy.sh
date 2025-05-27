#!/bin/bash
# Lodestone Deployment Manager
# Provides easy commands to deploy Lodestone in different environments

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
DEPLOY_DIR="$PROJECT_ROOT/deploy"
COMPOSE_DIR="$DEPLOY_DIR/compose"
ENV_DIR="$DEPLOY_DIR/environments"

# Default values
ENVIRONMENT=""
ACTION=""
SERVICES=""

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

# Function to show usage
show_usage() {
    cat << EOF
Lodestone Deployment Manager

Usage: $0 <action> <environment> [services...]

Actions:
    up          Start services (with optional migration)
    down        Stop services  
    restart     Restart services
    logs        Show logs
    ps          Show running services
    build       Build images
    pull        Pull latest images
    clean       Remove containers and volumes
    health      Check service health
    migrate     Run database migrations
    migrate-up  Run pending migrations
    migrate-down Roll back last migration

Environments:
    local       Local development (no external dependencies)
    dev         Development with MinIO S3 simulation
    prod        Production with Nginx and SSL support

Services (optional, default: all):
    postgres    PostgreSQL database
    redis       Redis cache
    api-gateway Lodestone API server
    nginx       Nginx reverse proxy (prod only)
    minio       MinIO S3 simulation (dev only)

Examples:
    $0 up local                    # Start local development
    $0 up dev                      # Start development with MinIO
    $0 up prod                     # Start production deployment
    $0 up local --migrate          # Start with automatic migrations
    $0 migrate-up dev              # Run pending migrations only
    $0 migrate-down prod           # Roll back last migration
    $0 restart prod api-gateway    # Restart only API gateway in prod
    $0 logs dev                    # Show all logs in dev
    $0 logs dev api-gateway        # Show API gateway logs in dev
    $0 down prod                   # Stop production deployment
    $0 clean local                 # Remove local containers and volumes

Environment Setup:
    For first-time setup, copy the appropriate .env file:
    cp $ENV_DIR/.env.local .env           # For local development
    cp $ENV_DIR/.env.dev .env             # For development 
    cp $ENV_DIR/.env.prod.template .env   # For production (edit first!)

EOF
}

# Function to check if environment file exists
check_env_file() {
    local env_file="$PROJECT_ROOT/.env"
    if [ ! -f "$env_file" ]; then
        print_warning "No .env file found in project root"
        print_info "Copy one of the template files:"
        echo "  cp $ENV_DIR/.env.$ENVIRONMENT .env"
        echo "  # Then edit .env to customize your settings"
        return 1
    fi
    return 0
}

# Function to get docker-compose command based on environment
get_compose_cmd() {
    local env="$1"
    local base_cmd="docker-compose -f $COMPOSE_DIR/docker-compose.yml"
    
    case "$env" in
        "local")
            echo "$base_cmd"
            ;;
        "dev")
            echo "$base_cmd -f $COMPOSE_DIR/docker-compose.dev.yml"
            ;;
        "prod")
            echo "$base_cmd -f $COMPOSE_DIR/docker-compose.prod.yml"
            ;;
        *)
            print_error "Unknown environment: $env"
            exit 1
            ;;
    esac
}

# Function to run database migrations
run_migrations() {
    local env="$1"
    local compose_cmd="$2"
    
    print_info "Running database migrations..."
    
    # Ensure postgres is running and healthy
    $compose_cmd up -d postgres
    
    # Wait for postgres to be ready
    local max_attempts=30
    local attempt=1
    
    while [ $attempt -le $max_attempts ]; do
        if $compose_cmd ps postgres | grep -q "healthy"; then
            break
        fi
        
        print_info "Waiting for database... (attempt $attempt/$max_attempts)"
        sleep 2
        ((attempt++))
        
        if [ $attempt -gt $max_attempts ]; then
            print_error "Database failed to become ready"
            return 1
        fi
    done
    
    # Run migrations
    if $compose_cmd run --rm migrate -up; then
        print_success "Database migrations completed successfully"
        return 0
    else
        print_error "Database migrations failed"
        return 1
    fi
}

# Function to execute docker-compose commands
execute_compose() {
    local env="$1"
    local action="$2"
    shift 2
    local services="$@"
    local run_migrate=false
    
    # Check for --migrate flag in services
    local filtered_services=""
    for service in $services; do
        if [ "$service" = "--migrate" ]; then
            run_migrate=true
        else
            filtered_services="$filtered_services $service"
        fi
    done
    services="$filtered_services"
    
    if ! check_env_file; then
        exit 1
    fi
    
    local compose_cmd
    compose_cmd=$(get_compose_cmd "$env")
    
    cd "$PROJECT_ROOT"
    
    case "$action" in
        "up")
            print_info "Starting Lodestone ($env environment)..."
            
            # Run migrations if requested or if no specific services specified
            if [ "$run_migrate" = true ] || [ -z "$services" ]; then
                if ! run_migrations "$env" "$compose_cmd"; then
                    print_error "Failed to run migrations, stopping deployment"
                    exit 1
                fi
            fi
            
            if [ "$env" = "prod" ]; then
                $compose_cmd up -d $services
                print_success "Production deployment started"
                print_info "Services available at:"
                echo "  • API Gateway: http://localhost:8080"
                echo "  • Nginx Proxy: http://localhost (if configured)"
            else
                $compose_cmd up -d $services
                print_success "$env environment started"
                print_info "Services available at:"
                echo "  • API Gateway: http://localhost:8080"
                echo "  • PostgreSQL: localhost:5432 (user: lodestone)"
                echo "  • Redis: localhost:6379"
                if [ "$env" = "dev" ]; then
                    echo "  • MinIO Console: http://localhost:9001 (minioadmin/minioadmin)"
                fi
            fi
            ;;
        "down")
            print_info "Stopping Lodestone ($env environment)..."
            $compose_cmd down $services
            print_success "Services stopped"
            ;;
        "restart")
            print_info "Restarting services..."
            $compose_cmd restart $services
            print_success "Services restarted"
            ;;
        "logs")
            if [ -n "$services" ]; then
                $compose_cmd logs -f $services
            else
                $compose_cmd logs -f
            fi
            ;;
        "ps")
            $compose_cmd ps
            ;;
        "build")
            print_info "Building images..."
            $compose_cmd build $services
            print_success "Build completed"
            ;;
        "pull")
            print_info "Pulling latest images..."
            $compose_cmd pull $services
            print_success "Images updated"
            ;;
        "clean")
            print_warning "This will remove all containers and volumes!"
            read -p "Are you sure? (y/N): " -n 1 -r
            echo
            if [[ $REPLY =~ ^[Yy]$ ]]; then
                $compose_cmd down -v --remove-orphans
                docker system prune -f
                print_success "Cleanup completed"
            else
                print_info "Cleanup cancelled"
            fi
            ;;
        "health")
            print_info "Checking service health..."
            $DEPLOY_DIR/scripts/health-check.sh
            ;;
        "migrate"|"migrate-up")
            print_info "Running database migrations..."
            if run_migrations "$env" "$compose_cmd"; then
                print_success "Migrations completed successfully"
            else
                print_error "Migration failed"
                exit 1
            fi
            ;;
        "migrate-down")
            print_warning "Rolling back the last migration..."
            print_warning "This will undo the most recent database changes!"
            read -p "Are you sure you want to continue? (y/N): " -n 1 -r
            echo
            if [[ $REPLY =~ ^[Yy]$ ]]; then
                # Ensure postgres is running
                $compose_cmd up -d postgres
                
                # Wait for postgres to be ready
                local max_attempts=30
                local attempt=1
                
                while [ $attempt -le $max_attempts ]; do
                    if $compose_cmd ps postgres | grep -q "healthy"; then
                        break
                    fi
                    
                    print_info "Waiting for database... (attempt $attempt/$max_attempts)"
                    sleep 2
                    ((attempt++))
                    
                    if [ $attempt -gt $max_attempts ]; then
                        print_error "Database failed to become ready"
                        exit 1
                    fi
                done
                
                # Run migration rollback
                if $compose_cmd run --rm migrate -down; then
                    print_success "Migration rollback completed successfully"
                else
                    print_error "Migration rollback failed"
                    exit 1
                fi
            else
                print_info "Migration rollback cancelled"
            fi
            ;;
        *)
            print_error "Unknown action: $action"
            show_usage
            exit 1
            ;;
    esac
}

# Main script logic
if [ $# -lt 2 ]; then
    show_usage
    exit 1
fi

ACTION="$1"
ENVIRONMENT="$2"
shift 2
SERVICES="$@"

# Validate environment
case "$ENVIRONMENT" in
    "local"|"dev"|"prod")
        ;;
    *)
        print_error "Invalid environment: $ENVIRONMENT"
        print_info "Valid environments: local, dev, prod"
        exit 1
        ;;
esac

# Special case for help
if [ "$ACTION" = "help" ] || [ "$ACTION" = "-h" ] || [ "$ACTION" = "--help" ]; then
    show_usage
    exit 0
fi

# Execute the command
execute_compose "$ENVIRONMENT" "$ACTION" $SERVICES
