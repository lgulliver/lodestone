-- +migrate Up
-- Initial database schema for Lodestone Artifact Registry

-- Enable UUID extension for PostgreSQL
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- Users table for authentication
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username VARCHAR(50) UNIQUE NOT NULL,
    email VARCHAR(255) UNIQUE NOT NULL,
    password TEXT NOT NULL,
    is_active BOOLEAN DEFAULT true NOT NULL,
    is_admin BOOLEAN DEFAULT false NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- API Keys table for programmatic access
CREATE TABLE api_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    key_hash TEXT NOT NULL,
    permissions JSONB DEFAULT '[]'::jsonb,
    expires_at TIMESTAMP WITH TIME ZONE,
    last_used_at TIMESTAMP WITH TIME ZONE,
    is_active BOOLEAN DEFAULT true NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Artifacts table for package metadata
CREATE TABLE artifacts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    version VARCHAR(100) NOT NULL,
    registry VARCHAR(50) NOT NULL,
    content_type VARCHAR(255),
    size BIGINT DEFAULT 0,
    sha256 VARCHAR(64),
    storage_path TEXT NOT NULL,
    metadata JSONB DEFAULT '{}'::jsonb,
    downloads BIGINT DEFAULT 0,
    published_by UUID NOT NULL REFERENCES users(id),
    is_public BOOLEAN DEFAULT false NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    
    -- Ensure unique packages per registry
    UNIQUE(name, version, registry)
);

-- Permissions table for fine-grained access control
CREATE TABLE permissions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    resource VARCHAR(255) NOT NULL, -- registry:nuget, package:lodestone/myapp
    action VARCHAR(50) NOT NULL,    -- read, write, delete
    granted_by UUID REFERENCES users(id),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Artifact search index for fast searching
CREATE TABLE artifact_indices (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    artifact_id UUID UNIQUE NOT NULL REFERENCES artifacts(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    registry VARCHAR(50) NOT NULL,
    searchable_text TEXT,
    tags JSONB DEFAULT '[]'::jsonb,
    description TEXT,
    author VARCHAR(255),
    keywords JSONB DEFAULT '[]'::jsonb,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Download events for analytics
CREATE TABLE download_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    artifact_id UUID NOT NULL REFERENCES artifacts(id) ON DELETE CASCADE,
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    ip_address INET,
    user_agent TEXT,
    registry VARCHAR(50) NOT NULL,
    name VARCHAR(255) NOT NULL,
    version VARCHAR(100) NOT NULL,
    timestamp TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Indexes for performance
CREATE INDEX idx_users_username ON users(username);
CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_users_active ON users(is_active) WHERE is_active = true;

CREATE INDEX idx_api_keys_user_id ON api_keys(user_id);
CREATE INDEX idx_api_keys_active ON api_keys(is_active) WHERE is_active = true;

CREATE INDEX idx_artifacts_name ON artifacts(name);
CREATE INDEX idx_artifacts_registry ON artifacts(registry);
CREATE INDEX idx_artifacts_published_by ON artifacts(published_by);
CREATE INDEX idx_artifacts_sha256 ON artifacts(sha256);
CREATE INDEX idx_artifacts_public ON artifacts(is_public) WHERE is_public = true;
CREATE INDEX idx_artifacts_registry_name ON artifacts(registry, name);
CREATE INDEX idx_artifacts_created_at ON artifacts(created_at DESC);
CREATE INDEX idx_artifacts_downloads ON artifacts(downloads DESC);

CREATE INDEX idx_permissions_user_id ON permissions(user_id);
CREATE INDEX idx_permissions_resource ON permissions(resource);
CREATE INDEX idx_permissions_user_resource ON permissions(user_id, resource);

CREATE INDEX idx_artifact_indices_name ON artifact_indices(name);
CREATE INDEX idx_artifact_indices_registry ON artifact_indices(registry);
CREATE INDEX idx_artifact_indices_author ON artifact_indices(author);
CREATE INDEX idx_artifact_indices_searchable_text ON artifact_indices USING gin(to_tsvector('english', searchable_text));
CREATE INDEX idx_artifact_indices_tags ON artifact_indices USING gin(tags);
CREATE INDEX idx_artifact_indices_keywords ON artifact_indices USING gin(keywords);

CREATE INDEX idx_download_events_artifact_id ON download_events(artifact_id);
CREATE INDEX idx_download_events_user_id ON download_events(user_id);
CREATE INDEX idx_download_events_timestamp ON download_events(timestamp DESC);
CREATE INDEX idx_download_events_ip_address ON download_events(ip_address);
CREATE INDEX idx_download_events_registry ON download_events(registry);
CREATE INDEX idx_download_events_name ON download_events(name);
CREATE INDEX idx_download_events_registry_name ON download_events(registry, name);
CREATE INDEX idx_download_events_timestamp_registry ON download_events(timestamp DESC, registry);

-- Triggers for updated_at timestamps
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

CREATE TRIGGER update_users_updated_at BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_api_keys_updated_at BEFORE UPDATE ON api_keys
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_artifacts_updated_at BEFORE UPDATE ON artifacts
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_artifact_indices_updated_at BEFORE UPDATE ON artifact_indices
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- +migrate Down
-- Drop everything in reverse order

DROP TRIGGER IF EXISTS update_artifact_indices_updated_at ON artifact_indices;
DROP TRIGGER IF EXISTS update_artifacts_updated_at ON artifacts;
DROP TRIGGER IF EXISTS update_api_keys_updated_at ON api_keys;
DROP TRIGGER IF EXISTS update_users_updated_at ON users;

DROP FUNCTION IF EXISTS update_updated_at_column();

DROP INDEX IF EXISTS idx_download_events_timestamp_registry;
DROP INDEX IF EXISTS idx_download_events_registry_name;
DROP INDEX IF EXISTS idx_download_events_name;
DROP INDEX IF EXISTS idx_download_events_registry;
DROP INDEX IF EXISTS idx_download_events_ip_address;
DROP INDEX IF EXISTS idx_download_events_timestamp;
DROP INDEX IF EXISTS idx_download_events_user_id;
DROP INDEX IF EXISTS idx_download_events_artifact_id;

DROP INDEX IF EXISTS idx_artifact_indices_keywords;
DROP INDEX IF EXISTS idx_artifact_indices_tags;
DROP INDEX IF EXISTS idx_artifact_indices_searchable_text;
DROP INDEX IF EXISTS idx_artifact_indices_author;
DROP INDEX IF EXISTS idx_artifact_indices_registry;
DROP INDEX IF EXISTS idx_artifact_indices_name;

DROP INDEX IF EXISTS idx_permissions_user_resource;
DROP INDEX IF EXISTS idx_permissions_resource;
DROP INDEX IF EXISTS idx_permissions_user_id;

DROP INDEX IF EXISTS idx_artifacts_downloads;
DROP INDEX IF EXISTS idx_artifacts_created_at;
DROP INDEX IF EXISTS idx_artifacts_registry_name;
DROP INDEX IF EXISTS idx_artifacts_public;
DROP INDEX IF EXISTS idx_artifacts_sha256;
DROP INDEX IF EXISTS idx_artifacts_published_by;
DROP INDEX IF EXISTS idx_artifacts_registry;
DROP INDEX IF EXISTS idx_artifacts_name;

DROP INDEX IF EXISTS idx_api_keys_active;
DROP INDEX IF EXISTS idx_api_keys_user_id;

DROP INDEX IF EXISTS idx_users_active;
DROP INDEX IF EXISTS idx_users_email;
DROP INDEX IF EXISTS idx_users_username;

DROP TABLE IF EXISTS download_events;
DROP TABLE IF EXISTS artifact_indices;
DROP TABLE IF EXISTS permissions;
DROP TABLE IF EXISTS artifacts;
DROP TABLE IF EXISTS api_keys;
DROP TABLE IF EXISTS users;

DROP EXTENSION IF EXISTS "pgcrypto";
DROP EXTENSION IF EXISTS "uuid-ossp";
