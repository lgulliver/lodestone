# Package Ownership in Lodestone

This document describes the package ownership system in Lodestone, which provides a way to control who can publish, update, and delete packages.

## Overview

The package ownership system implements a role-based access control model at the package level. Each package can have multiple users associated with it, each with a specific role that grants different permissions.

## Roles and Permissions

The following roles are available:

| Role | Description | Permissions |
|------|-------------|------------|
| `owner` | Full control of a package | - Upload new versions<br>- Delete versions<br>- Add/remove other owners<br>- Transfer ownership |
| `maintainer` | Can publish but not manage ownership | - Upload new versions<br>- Cannot delete versions<br>- Cannot modify ownership |
| `contributor` | Read-only access (for future use) | - Currently has no special permissions |

## Implementation

The ownership system is implemented using the following components:

1. **Database Schema**: A `package_ownerships` table with the following fields:
   - `id`: UUID primary key
   - `package_key`: String in the format `"registry:package_name"`  
   - `user_id`: Reference to the users table
   - `role`: String value (`"owner"`, `"maintainer"`, `"contributor"`)
   - `granted_by`: Reference to the user who granted this permission
   - `granted_at`: Timestamp when the permission was granted

2. **Service Layer**: The `OwnershipService` in the registry package implements the business logic for:
   - Checking if a user can publish a package version
   - Checking if a user can delete a package version
   - Managing ownership (add/remove owners)
   - Establishing initial ownership when a package is first published

3. **API Endpoints**: The API provides endpoints for managing package ownership:
   - `GET /api/v1/packages/{registry}/{name}/owners`: List all owners of a package
   - `POST /api/v1/packages/{registry}/{name}/owners`: Add a user as an owner/maintainer of a package
   - `DELETE /api/v1/packages/{registry}/{name}/owners/{username}`: Remove a user's ownership of a package
   - `GET /api/v1/packages/owned`: List all packages owned by the current user

## Initial Ownership

When a new package is published for the first time, the publishing user is automatically made an `owner` of the package.

## Admin Override

Users with admin privileges (`is_admin = true` in the users table) can bypass ownership checks and have full control over all packages.

## Package Keys

Package keys are a standardized format for uniquely identifying packages across different registries, using the format `"registry:package_name"`. 

Examples:
- `"npm:lodestone-ui"`
- `"nuget:Lodestone.Core"`
- `"maven:org.lodestone:server"`
- `"cargo:lodestone-client"`

## API Examples

### Listing Package Owners

```http
GET /api/v1/packages/npm/lodestone-client/owners
Authorization: Bearer <token>
```

Response:
```json
{
  "owners": [
    {
      "username": "alice",
      "role": "owner",
      "granted_by": "alice",
      "granted_at": "2025-05-01T12:00:00Z"
    },
    {
      "username": "bob",
      "role": "maintainer",
      "granted_by": "alice",
      "granted_at": "2025-05-02T14:30:00Z"
    }
  ]
}
```

### Adding Package Owner

```http
POST /api/v1/packages/npm/lodestone-client/owners
Authorization: Bearer <token>
Content-Type: application/json

{
  "username": "carol",
  "role": "maintainer"
}
```

Response:
```json
{
  "success": true,
  "message": "User carol added as maintainer"
}
```

### Removing Package Owner

```http
DELETE /api/v1/packages/npm/lodestone-client/owners/bob
Authorization: Bearer <token>
```

Response:
```json
{
  "success": true,
  "message": "User bob removed from package"
}
```

## Security Considerations

- Only owners can manage ownership
- The last owner of a package cannot be removed
- Admins can bypass ownership checks
- All ownership changes are logged
