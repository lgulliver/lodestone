# List of todo items

- [x] Move logging to use zerolog
- [ ] Create auth service
  - [ ] Local accounts
  - [ ] OIDC accounts (Microsoft, Google, GitHub) (doesn't need doing now)
- [ ] Base registry service
- [ ] NuGet registry
- [ ] Cargo registry
- [ ] npm registry
- [ ] OCI registry
- [ ] Helm registry
- [ ] Rubygems registry
- [ ] OPA registry
- [ ] Go registry
- [ ] Metadata service
- [ ] Ensure only owners of a package can modify a package
- [ ] UI
- [ ] Ensure commands like `docker pull` work against the registry

## High Priority Implementation Tasks

### Authentication & Security
- [ ] Implement input validation for all package uploads
- [ ] Add comprehensive authentication and authorization for package access
- [ ] Implement package ownership validation (ensure only owners can modify packages)
- [ ] Add rate limiting for API endpoints
- [ ] Implement security testing for authentication flows

### Database & Storage
- [ ] Complete database integration for metadata and user management
- [ ] Design and implement database schema and migrations
- [ ] Configure storage backend (local/S3/Azure/GCP)
- [ ] Implement proper storage path generation algorithms

### Registry Implementations
- [ ] Complete all registry handler implementations (some are currently stubs)
- [ ] Add package validation logic for each format
- [ ] Implement metadata extraction from uploaded packages
- [ ] Add format-specific package validation
- [ ] Ensure Docker commands (`docker pull`, `docker push`) work against OCI registry
- [ ] Ensure npm commands (`npm install`, `npm publish`) work against npm registry
- [ ] Ensure NuGet commands (`nuget push`, `nuget restore`) work against NuGet registry

### API & Error Handling
- [ ] Improve error handling across all endpoints
- [ ] Add comprehensive API documentation (OpenAPI/Swagger)
- [ ] Implement proper HTTP status codes and error messages
- [ ] Add request/response validation

### Monitoring & Observability
- [ ] Enhance logging for better observability
- [ ] Implement monitoring and metrics collection
- [ ] Add health check endpoints
- [ ] Implement distributed tracing

### Testing
- [ ] Add integration tests for complete workflows
- [ ] Implement performance testing for large package uploads
- [ ] Add end-to-end testing for each package format
- [ ] Create test coverage reports

### Infrastructure & Deployment
- [ ] Create Docker containerization
- [ ] Design Kubernetes deployment manifests
- [ ] Set up CI/CD pipeline
- [ ] Create deployment guides and configuration examples
- [ ] Add environment-specific configuration management

### Documentation
- [ ] Complete API documentation
- [ ] Write deployment and configuration guides
- [ ] Create user guides for each package format
- [ ] Add developer documentation for contributing
