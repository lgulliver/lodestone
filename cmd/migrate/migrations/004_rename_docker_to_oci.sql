-- +migrate Up
-- Rename the Docker/OCI registry setting key from 'docker' to 'oci' to align with
-- the canonical name used by the middleware and route validation logic.
-- Migration 003 seeded the row as 'docker'; the API-gateway middleware normalises
-- all /v2/* paths to 'oci', so IsRegistryEnabled('oci') always returned not-found
-- (defaulting to disabled) even when the registry was intentionally enabled.

UPDATE registry_settings
SET    registry_name = 'oci',
       description   = 'Docker/OCI container registry'
WHERE  registry_name = 'docker';

-- +migrate Down
UPDATE registry_settings
SET    registry_name = 'docker',
       description   = 'Docker/OCI container registry'
WHERE  registry_name = 'oci';
