#!/bin/bash
# Database Migration Manager for Lodestone
# Provides commands to manage database migrations

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

# Default values
ENVIRONMENT=""
ACTION=""

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
Lodestone Database Migration Manager

Usage: $0 <action> <environment>

Actions:
    up          Run pending migrations
    down        Roll back the last migration
    status      Check migration status (planned feature)
    reset       Reset database and apply all migrations (planned feature)
    create      Create a new migration file (planned feature)

Environments:
    local       Local development
    dev         Development with external services
    prod        Production environment

Examples:
    $0 up local      # Run pending migrations in local environment
    $0 down prod     # Roll back last migration in production
    $0 status dev    # Check migration status in development

Notes:
    - Always run migrations before starting the application
    - Backup your database before running migrations in production
    - Migrations are run in a separate container with proper dependency ordering

EOF
}

# Function to get docker-compose command based on environment
get_compose_cmd() {
    local env="$1"
    local base_cmd="docker-compose -p lodestone -f $COMPOSE_DIR/docker-compose.yml"
    
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

# Function to check if environment file exists
check_env_file() {
    local env_file="$PROJECT_ROOT/.env"
    if [ ! -f "$env_file" ]; then
        print_warning "No .env file found in project root"
        print_info "Copy one of the template files:"
        echo "  cp $DEPLOY_DIR/environments/.env.$ENVIRONMENT .env"
        echo "  # Then edit .env to customize your settings"
        return 1
    fi
    return 0
}

# Function to wait for database to be ready
wait_for_database() {
    local env="$1"
    local compose_cmd
    compose_cmd=$(get_compose_cmd "$env")
    
    print_info "Ensuring database is ready..."
    
    # Start postgres if not running
    cd "$PROJECT_ROOT"
    $compose_cmd up -d postgres
    
    # Wait for health check to pass
    local max_attempts=30
    local attempt=1
    
    while [ $attempt -le $max_attempts ]; do
        if $compose_cmd ps postgres | grep -q "healthy"; then
            print_success "Database is ready"
            return 0
        fi
        
        print_info "Waiting for database... (attempt $attempt/$max_attempts)"
        sleep 2
        ((attempt++))
    done
    
    print_error "Database failed to become ready after $max_attempts attempts"
    print_info "Check database logs: $0 logs $env postgres"
    return 1
}

# Function to run migrations
run_migration() {
    local env="$1"
    local action="$2"
    
    if ! check_env_file; then
        exit 1
    fi
    
    if ! wait_for_database "$env"; then
        exit 1
    fi
    
    local compose_cmd
    compose_cmd=$(get_compose_cmd "$env")
    
    cd "$PROJECT_ROOT"
    
    case "$action" in
        "up")
            print_info "Running pending database migrations..."
            $compose_cmd run --rm migrate -up
            if [ $? -eq 0 ]; then
                print_success "Database migrations completed successfully"
            else
                print_error "Migration failed"
                exit 1
            fi
            ;;
        "down")
            print_warning "Rolling back the last migration..."
            print_warning "This will undo the most recent database changes!"
            read -p "Are you sure you want to continue? (y/N): " -n 1 -r
            echo
            if [[ $REPLY =~ ^[Yy]$ ]]; then
                $compose_cmd run --rm migrate -down
                if [ $? -eq 0 ]; then
                    print_success "Migration rollback completed successfully"
                else
                    print_error "Migration rollback failed"
                    exit 1
                fi
            else
                print_info "Rollback cancelled"
            fi
            ;;
        "status")
            print_info "Checking migration status..."
            print_warning "Status command not yet implemented"
            print_info "You can check applied migrations by connecting to the database:"
            echo "  SELECT version, name, applied_at FROM schema_migrations ORDER BY version;"
            ;;
        "reset")
            print_warning "Database reset not yet implemented"
            print_info "To manually reset:"
            echo "  1. Stop all services: $SCRIPT_DIR/deploy.sh down $env"
            echo "  2. Remove database volume: docker volume rm <project>_postgres_data"
            echo "  3. Start services: $SCRIPT_DIR/deploy.sh up $env"
            echo "  4. Run migrations: $0 up $env"
            ;;
        "create")
            print_warning "Migration creation not yet implemented"
            print_info "To manually create a migration:"
            echo "  1. Create file: cmd/migrate/migrations/XXX_description.sql"
            echo "  2. Use format: -- +migrate Up / -- +migrate Down"
            echo "  3. Increment version number from last migration"
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

# Execute the migration command
run_migration "$ENVIRONMENT" "$ACTION"
