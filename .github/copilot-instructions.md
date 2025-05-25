# Copilot Instructions for Lodestone Artifact Registry

## Project Overview
Lodestone is a multi-format artifact registry written in Go that supports NuGet, npm, Cargo, OCI/Docker, Helm, RubyGems, OPA, Maven, and Go modules. It uses a microservices architecture with PostgreSQL for persistence and Redis for caching.

## Architecture Principles

### Clean Architecture
- Follow domain-driven design principles
- Separate concerns: handlers → services → repositories
- Use dependency injection for testability
- Interfaces in the service layer, implementations in infrastructure

### Directory Structure
```
cmd/                    # Application entry points
├── api-gateway/        # Main HTTP API server
├── migrate/           # Database migration tool
internal/              # Private application code
├── auth/              # Authentication service
├── registry/          # Registry service with package handlers
│   └── registries/    # Package format implementations
├── metadata/          # Analytics and metadata service
├── storage/           # Blob storage abstraction
└── common/            # Shared utilities
pkg/                   # Public libraries
├── config/            # Configuration management
└── types/             # Shared data models
```

## Code Style Guidelines

### Go Conventions
- Use standard Go naming conventions (PascalCase for exported, camelCase for unexported)
- Prefer explicit error handling over panics
- Use context.Context for cancellation and request-scoped values
- Keep functions small and focused (max 50 lines)
- Use structured logging with zerolog

### Error Handling
```go
// Prefer explicit error returns
func (s *Service) Operation(ctx context.Context) error {
    if err := s.validate(); err != nil {
        return fmt.Errorf("validation failed: %w", err)
    }
    return nil
}

// Use structured logging for errors
log.Error().Err(err).Str("operation", "upload").Msg("operation failed")
```

### HTTP Handlers
- Use Gin framework patterns
- Always include context propagation
- Return consistent JSON error responses
- Use appropriate HTTP status codes

```go
func handleUpload(service *registry.Service) gin.HandlerFunc {
    return func(c *gin.Context) {
        user, exists := middleware.GetUserFromContext(c)
        if !exists {
            c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
            return
        }
        
        // Implementation...
    }
}
```

## Database Patterns

### GORM Models
- Use embedded `BaseModel` for common fields
- Include proper GORM tags for relationships
- Use UUID primary keys for security

```go
type Artifact struct {
    BaseModel
    Name         string    `gorm:"not null;index"`
    Version      string    `gorm:"not null;index"`
    Registry     string    `gorm:"not null;index"`
    PublishedBy  uuid.UUID `gorm:"type:uuid;not null"`
    // Additional fields...
}
```

### Service Layer
- Accept context as first parameter
- Use transactions for multi-step operations
- Handle database errors gracefully

## Registry Implementation Patterns

### Handler Interface
All registry handlers must implement:
```go
type Handler interface {
    Upload(ctx context.Context, artifact *types.Artifact, content io.Reader) error
    Download(ctx context.Context, name, version string) (*types.Artifact, io.ReadCloser, error)
    List(ctx context.Context, filter *types.ArtifactFilter) ([]*types.Artifact, error)
    Delete(ctx context.Context, name, version string) error
    Validate(ctx context.Context, artifact *types.Artifact, content io.Reader) error
    GetMetadata(ctx context.Context, content io.Reader) (map[string]interface{}, error)
    GenerateStoragePath(name, version string) string
}
```

### Package Format Standards
- Extract name/version from filename when possible
- Validate package format before upload
- Store metadata in standardized format
- Generate consistent storage paths

## Testing Guidelines

### Unit Tests
- Use testify for assertions
- Mock external dependencies
- Test error conditions
- Aim for >80% coverage

```go
func TestServiceMethod(t *testing.T) {
    // Arrange
    service := &Service{db: mockDB}
    
    // Act
    result, err := service.Method(context.Background())
    
    // Assert
    assert.NoError(t, err)
    assert.NotNil(t, result)
}
```

### Integration Tests
- Use real database with transactions
- Test complete workflows
- Verify HTTP responses
- Test authentication flows

## Configuration Management

### Environment Variables
- Use `pkg/config` for configuration
- Support both ENV vars and config files
- Validate configuration on startup
- Use sensible defaults

### Security
- Never log sensitive data (passwords, tokens)
- Use UUIDs for user/API key IDs
- Validate all inputs
- Implement proper authentication middleware

## Logging Standards

### Structured Logging with Zerolog
```go
log.Info().
    Str("registry", "npm").
    Str("package", name).
    Str("version", version).
    Int64("size", contentLength).
    Msg("package uploaded successfully")
```

### Log Levels
- `Error`: System errors, failed operations
- `Warn`: Recoverable issues, deprecated usage
- `Info`: Normal operations, audit events
- `Debug`: Detailed debugging information

## Performance Considerations

### Database
- Use prepared statements via GORM
- Implement proper indexing
- Use connection pooling
- Consider read replicas for scaling

### Storage
- Stream large files, don't buffer in memory
- Implement proper cleanup for failed uploads
- Use appropriate storage backends (local, S3, etc.)

### Caching
- Cache frequently accessed metadata
- Use Redis for distributed caching
- Implement cache invalidation strategies

## Security Best Practices

### Authentication
- Support both JWT tokens and API keys
- Implement proper token validation
- Use secure session management
- Log security events

### Authorization
- Implement package ownership validation
- Use role-based access control
- Validate permissions for all operations

### Input Validation
- Validate all user inputs
- Sanitize file uploads
- Check file size limits
- Validate package formats

## Development Workflow

### Git Practices
- Use conventional commits
- Create feature branches
- Require PR reviews
- Keep commits atomic and focused

### Code Quality
- Run `go vet` and `golint`
- Maintain test coverage
- Use consistent formatting with `gofmt`
- Document public APIs
- If an error is made during generation, correct it but make note of the change

### Continuous Integration
- Use GitHub Actions for CI
- Run tests on every push
- Lint and vet code
- Build for containers

## Common Patterns to Follow

### Service Construction
```go
func NewService(db *gorm.DB, storage storage.BlobStorage, cache cache.Cache) *Service {
    return &Service{
        db:      db,
        storage: storage,
        cache:   cache,
    }
}
```

### Context Usage
- Pass context through all layers
- Use context for cancellation
- Store request-scoped data in context
- Respect context deadlines

### Error Wrapping
```go
if err != nil {
    return fmt.Errorf("failed to upload to storage: %w", err)
}
```

Remember: The goal is to build a robust, scalable, and maintainable artifact registry that can handle multiple package formats while maintaining consistency and performance.
