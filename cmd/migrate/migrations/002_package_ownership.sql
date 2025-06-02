-- +migrate Up
-- Package ownership table for managing package-level permissions

CREATE TABLE package_ownerships (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    package_key VARCHAR(255) NOT NULL, -- format: "registry:package_name"
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role VARCHAR(50) NOT NULL, -- "owner", "maintainer", "contributor"
    granted_by UUID NOT NULL REFERENCES users(id),
    granted_at TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    
    -- Ensure unique user-package combinations
    UNIQUE(package_key, user_id)
);

-- Indexes for performance
CREATE INDEX idx_package_ownerships_package_key ON package_ownerships(package_key);
CREATE INDEX idx_package_ownerships_user_id ON package_ownerships(user_id);
CREATE INDEX idx_package_ownerships_role ON package_ownerships(role);
CREATE INDEX idx_package_ownerships_package_user ON package_ownerships(package_key, user_id);

-- +migrate Down
-- Drop package ownership table and indexes

DROP INDEX IF EXISTS idx_package_ownerships_package_user;
DROP INDEX IF EXISTS idx_package_ownerships_role;
DROP INDEX IF EXISTS idx_package_ownerships_user_id;
DROP INDEX IF EXISTS idx_package_ownerships_package_key;

DROP TABLE IF EXISTS package_ownerships;
