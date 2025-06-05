# Lodestone System RBAC Design

## Overview

Lodestone implements Role-Based Access Control (RBAC) to manage permissions for both system-wide and package-level operations. This document describes the RBAC model, current state, and what needs to be built to achieve robust, maintainable authorization across the application.

---

## 1. Current State

- **Package-level roles** (`owner`, `maintainer`, `contributor`) are implemented for package ownership and permissions.
- **System-level admin** is present (`is_admin` or `role` field in `users` table), with some admin checks in middleware and service logic.
- **No extensible, general-purpose RBAC** for system-wide roles/permissions (e.g., for future roles like `support`, `auditor`, or fine-grained permissions).

---

## 2. RBAC Model

### System Roles

| Role      | Description                                 | Example Permissions                                  |
|-----------|---------------------------------------------|------------------------------------------------------|
| admin     | Full system access                          | Manage users, registry settings, all packages        |
| user      | Default, regular user                       | Publish/manage own packages                          |
| support   | (future) Support staff                      | View all packages and users, limited user management |
| auditor   | (future) Read-only for logs/audit           | View audit logs, no write access                     |

### Package Roles

| Role         | Description                  | Permissions                                   |
|--------------|------------------------------|-----------------------------------------------|
| owner        | Full control of a package    | Upload, delete, manage owners, transfer       |
| maintainer   | Can publish, not manage own. | Upload new versions, cannot delete/modify own.|
| contributor  | Read-only (future)           | No special permissions                        |

---

## 3. Data Model

### User Model

```go
type User struct {
    ID        uuid.UUID `gorm:"primaryKey"`
    Email     string    `gorm:"uniqueIndex;not null"`
    // ...
    Role      string    `gorm:"not null;default:'user'"` // 'admin', 'user', etc.
}
```

### Package Ownership Model

Already supports package-level roles.

---

## 4. Enforcement

### Middleware

- Use a generic `RequireRole(roles ...string)` middleware for Gin to protect system endpoints.
- Example:

```go
func RequireRole(roles ...string) gin.HandlerFunc {
    return func(c *gin.Context) {
        user, exists := middleware.GetUserFromContext(c)
        if !exists {
            c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
            c.Abort()
            return
        }
        for _, role := range roles {
            if user.Role == role {
                c.Next()
                return
            }
        }
        c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
        c.Abort()
    }
}
```


### Service Layer

Check roles in service methods for sensitive operations (e.g., registry enable/disable, user management).

---

## 5. What Needs to Be Built

- [ ] **Database:** Add `role` column to `users` table (if not present). Migrate existing users.
- [ ] **Middleware:** Implement and use `RequireRole` for all system-level endpoints.
- [ ] **Service Layer:** Add role checks to sensitive service methods.
- [ ] **API/UI:** Show/hide features based on role. Return clear error messages for forbidden actions.
- [ ] **Documentation:** Add RBAC section to project docs. Document how to assign roles to users.

---

## 6. Example Usage

```go
// Protect an endpoint for admins only
admin := api.Group("/admin")
admin.Use(middleware.RequireRole("admin"))
admin.POST("/registries/:name/enable", enableRegistryHandler)
```

---

## 7. Future Extensions

- Support for custom roles/permissions.
- Organization/team-level RBAC.
- Audit logging for all RBAC-protected actions.

---

## Summary

Lodestone will use a unified RBAC system for both system and package-level authorization. This will be enforced via middleware, service logic, and database roles, and documented for maintainability and extensibility.
