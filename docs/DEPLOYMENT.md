# Lodestone Deployment Guide

This guide covers deploying Lodestone artifact registry using Docker Compose for both development and production environments.

## Quick Start

1. **Clone and setup environment**:
   ```bash
   git clone <repository-url>
   cd lodestone
   make env-setup
   ```

2. **Edit environment variables**:
   ```bash
   vim .env
   ```

3. **Start the stack**:
   ```bash
   make docker-up
   ```

4. **Verify deployment**:
   ```bash
   make deploy-check
   ```

## Environment Configuration

### Required Environment Variables

Copy `.env.example` to `.env` and configure the following:

#### Database
```bash
POSTGRES_DB=lodestone
POSTGRES_USER=lodestone
POSTGRES_PASSWORD=your-secure-postgres-password  # REQUIRED
```

#### Security
```bash
JWT_SECRET=your-very-secure-jwt-secret-key-at-least-32-chars  # REQUIRED
REDIS_PASSWORD=your-secure-redis-password  # REQUIRED
```

#### Storage (Optional)
```bash
# Local storage (default)
STORAGE_TYPE=local

# Or S3-compatible storage
STORAGE_TYPE=s3
S3_BUCKET=your-bucket-name
S3_REGION=us-east-1
S3_ACCESS_KEY=your-access-key
S3_SECRET_KEY=your-secret-key
```

## Deployment Options

### Development Environment

For local development with hot reloading and debug features:

```bash
make docker-dev
```

This uses `deployments/docker-compose.dev.yml` with:
- Debug logging enabled
- Development-friendly settings
- MinIO for S3-compatible local storage
- Exposed database ports for inspection

### Production Environment

For production deployment with Nginx reverse proxy:

```bash
make docker-prod
```

This includes:
- Nginx reverse proxy with rate limiting
- Security headers
- SSL termination ready
- Optimized container settings
- Health checks

### Basic Production (No Nginx)

For simple production without reverse proxy:

```bash
make docker-up
```

## Registry Support

Lodestone supports multiple package formats:

| Registry | Endpoint | Status |
|----------|----------|--------|
| npm | `/npm/` | ✅ Ready |
| NuGet | `/nuget/` | ✅ Ready |
| Maven | `/maven2/` | ✅ Ready |
| Go Modules | `/go/` | ✅ Ready |
| Helm | `/helm/` | ✅ Ready |
| Cargo | `/crates/` | ✅ Ready |
| RubyGems | `/gems/` | ✅ Ready |
| OPA | `/opa/` | ✅ Ready |
| OCI/Docker | `/v2/` | 🚧 In Progress |

## Management Commands

### Service Management
```bash
make docker-up         # Start all services
make docker-down       # Stop all services
make docker-restart    # Restart application
make docker-logs       # View logs
make deploy-status     # Check service status
```

### Database Management
```bash
make db-migrate        # Run migrations
make db-reset          # Reset database (⚠️  DESTRUCTIVE)
```

### Monitoring
```bash
make docker-logs       # Follow all logs
make deploy-check      # Health check
make deploy-status     # Service status
```

## Service Architecture

### Core Services

1. **PostgreSQL** (`postgres:5432`)
   - Primary data store
   - Artifact metadata and user data
   - Automatic health checks

2. **Redis** (`redis:6379`)
   - Caching layer
   - Session storage
   - Rate limiting data

3. **API Gateway** (`api-gateway:8080`)
   - Main application server
   - Registry endpoints
   - Authentication and authorization

4. **Nginx** (`nginx:80/443`) - Production only
   - Reverse proxy
   - Rate limiting
   - SSL termination
   - Static file serving

### Data Persistence

Persistent volumes:
- `postgres_data`: Database files
- `redis_data`: Redis persistence
- `artifacts_data`: Local artifact storage
- `nginx_logs`: Nginx access and error logs

## Network Configuration

Services communicate on an isolated Docker network (`lodestone-network`) with subnet `172.20.0.0/16`.

### Port Mapping

| Service | Internal Port | External Port | Description |
|---------|---------------|---------------|-------------|
| PostgreSQL | 5432 | 5432 | Database (dev only) |
| Redis | 6379 | 6379 | Cache (dev only) |
| API Gateway | 8080 | 8080 | Main API |
| Nginx | 80/443 | 80/443 | HTTP/HTTPS (prod) |

## Security Considerations

### Production Security Checklist

- [ ] Set strong passwords for all services
- [ ] Use secure JWT secret (min 32 chars)
- [ ] Configure CORS origins appropriately
- [ ] Set up SSL certificates for HTTPS
- [ ] Enable rate limiting
- [ ] Review firewall rules
- [ ] Set up log monitoring
- [ ] Configure backup strategy

### Authentication

1. **JWT Tokens**: For API access and web sessions
2. **API Keys**: For programmatic access (future feature)
3. **BCrypt**: For password hashing

### Network Security

- Services isolated in Docker network
- Only necessary ports exposed
- Rate limiting on API endpoints
- Security headers via Nginx

## Monitoring and Logging

### Health Checks

All services include health checks:
- PostgreSQL: `pg_isready`
- Redis: `redis-cli ping`
- API Gateway: HTTP `/health` endpoint
- Nginx: Process check

### Logging

Structured JSON logging in production:
```bash
docker-compose logs -f api-gateway
```

Log levels: `debug`, `info`, `warn`, `error`

### Metrics

Future: Prometheus metrics endpoint at `/metrics`

## Backup and Recovery

### Database Backup
```bash
docker-compose exec postgres pg_dump -U lodestone lodestone > backup.sql
```

### Artifact Storage Backup
```bash
docker run --rm -v lodestone_artifacts_data:/data -v $(pwd):/backup alpine tar czf /backup/artifacts.tar.gz /data
```

### Restore Database
```bash
docker-compose exec -T postgres psql -U lodestone lodestone < backup.sql
```

## Troubleshooting

### Common Issues

1. **Permission Denied**
   ```bash
   # Fix artifact directory permissions
   docker-compose exec api-gateway chown -R appuser:appgroup /app/artifacts
   ```

2. **Database Connection Failed**
   ```bash
   # Check database status
   docker-compose exec postgres pg_isready -U lodestone
   ```

3. **Out of Disk Space**
   ```bash
   # Clean up Docker resources
   make docker-clean
   ```

### Debug Mode

Enable debug logging:
```bash
echo "LOG_LEVEL=debug" >> .env
make docker-restart
```

### Performance Tuning

For high-load environments:
1. Increase PostgreSQL connections
2. Scale Redis memory
3. Adjust Nginx worker processes
4. Configure CDN for artifacts

## SSL/TLS Setup (Production)

1. **Obtain SSL certificates** (Let's Encrypt recommended):
   ```bash
   certbot certonly --webroot -w /var/www/html -d your-domain.com
   ```

2. **Update Nginx configuration**:
   ```bash
   # Add SSL configuration to nginx/conf.d/lodestone.conf
   ```

3. **Mount certificates**:
   ```bash
   # Update docker-compose.yml nginx volumes
   ```

## Scaling Considerations

### Horizontal Scaling

- Multiple API Gateway instances behind load balancer
- Read replicas for PostgreSQL
- Redis Cluster for distributed caching
- CDN for artifact downloads

### Vertical Scaling

Adjust resource limits in `docker-compose.yml`:
```yaml
deploy:
  resources:
    limits:
      memory: 2G
      cpus: '1.0'
```

## Migration from Development

1. Export development data
2. Update environment configuration
3. Deploy production stack
4. Import data
5. Verify functionality

## Support

For issues and support:
1. Check service logs: `make docker-logs`
2. Verify health: `make deploy-check`
3. Review configuration
4. Check GitHub issues

---

**⚠️ Important**: Always backup your data before major updates or configuration changes.
