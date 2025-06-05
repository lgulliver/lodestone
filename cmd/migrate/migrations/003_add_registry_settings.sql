-- +migrate Up
-- Registry settings table for runtime management of package format enabling/disabling

CREATE TABLE registry_settings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    registry_name VARCHAR(50) NOT NULL UNIQUE, -- "nuget", "npm", "cargo", "docker", "helm", "rubygems", "opa", "maven", "go"
    enabled BOOLEAN NOT NULL DEFAULT true,
    description TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_by UUID REFERENCES users(id)
);

-- Index for performance
CREATE INDEX idx_registry_settings_registry_name ON registry_settings(registry_name);
CREATE INDEX idx_registry_settings_enabled ON registry_settings(enabled);

-- Insert default registry settings for all supported formats
INSERT INTO registry_settings (registry_name, enabled, description) VALUES
    ('nuget', true, 'NuGet package registry'),
    ('npm', true, 'npm package registry'),
    ('cargo', true, 'Cargo/Rust package registry'),
    ('docker', true, 'Docker/OCI container registry'),
    ('helm', true, 'Helm chart registry'),
    ('rubygems', true, 'RubyGems package registry'),
    ('opa', true, 'Open Policy Agent bundle registry'),
    ('maven', true, 'Maven package registry'),
    ('go', true, 'Go module registry');

-- Add trigger for updated_at timestamp
CREATE TRIGGER update_registry_settings_updated_at BEFORE UPDATE ON registry_settings
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- +migrate Down
DROP TRIGGER IF EXISTS update_registry_settings_updated_at ON registry_settings;
DROP TABLE IF EXISTS registry_settings;
