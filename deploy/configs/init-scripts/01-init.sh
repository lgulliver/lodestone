#!/bin/bash
set -e

# Database initialization script for Lodestone
# This script runs when PostgreSQL container starts for the first time

echo "🗄️  Initializing Lodestone database..."

# Enable required PostgreSQL extensions
psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<-EOSQL
    -- Enable UUID extension for generating UUIDs
    CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
    
    -- Enable pgcrypto for additional cryptographic functions
    CREATE EXTENSION IF NOT EXISTS "pgcrypto";
    
    -- Enable pg_trgm for trigram matching (useful for search)
    CREATE EXTENSION IF NOT EXISTS "pg_trgm";
    
    -- Create database schema comment
    COMMENT ON DATABASE $POSTGRES_DB IS 'Lodestone Artifact Registry Database';
    
    -- Set database timezone to UTC
    ALTER DATABASE $POSTGRES_DB SET timezone TO 'UTC';
EOSQL

echo "✅ Database initialization completed successfully!"

# Optional: Create read-only user for monitoring/analytics
if [ -n "$POSTGRES_READONLY_USER" ] && [ -n "$POSTGRES_READONLY_PASSWORD" ]; then
    echo "👀 Creating read-only monitoring user..."
    psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<-EOSQL
        -- Create read-only user
        CREATE USER $POSTGRES_READONLY_USER WITH PASSWORD '$POSTGRES_READONLY_PASSWORD';
        
        -- Grant connection privileges
        GRANT CONNECT ON DATABASE $POSTGRES_DB TO $POSTGRES_READONLY_USER;
        
        -- Grant usage on schema
        GRANT USAGE ON SCHEMA public TO $POSTGRES_READONLY_USER;
        
        -- Grant select on all existing tables
        GRANT SELECT ON ALL TABLES IN SCHEMA public TO $POSTGRES_READONLY_USER;
        
        -- Grant select on all future tables
        ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT ON TABLES TO $POSTGRES_READONLY_USER;
        
        -- Comment on user
        COMMENT ON ROLE $POSTGRES_READONLY_USER IS 'Read-only user for monitoring and analytics';
EOSQL
    echo "✅ Read-only user created successfully!"
fi
