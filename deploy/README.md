# Lodestone Deployment

This directory contains all deployment configurations and scripts for the Lodestone Artifact Registry.

## 🚀 Quick Start

### First-Time Setup

1. **Choose your environment and run setup:**
   ```bash
   # Local development (minimal)
   ./deploy/scripts/setup.sh local
   
   # Development with S3 simulation
   ./deploy/scripts/setup.sh dev
   
   # Production deployment
   ./deploy/scripts/setup.sh prod
   ```

2. **Start the deployment:**
   ```bash
   # Start your chosen environment
   ./deploy/scripts/deploy.sh up <environment>
   ```

3. **Verify everything is working:**
   ```bash
   ./deploy/scripts/health-check.sh
   ```

## 📁 Directory Structure

```
deploy/
├── compose/                    # Docker Compose configurations
│   ├── docker-compose.yml     # Base configuration (all services)
│   ├── docker-compose.dev.yml # Development overrides (+ MinIO)
│   └── docker-compose.prod.yml# Production overrides (+ Nginx, SSL)
├── configs/                    # Service configurations
│   ├── docker/                # Dockerfiles
│   ├── nginx/                 # Nginx configuration files
│   └── logrotate/            # Log rotation configuration
├── environments/               # Environment templates
│   ├── .env.local            # Local development settings
│   ├── .env.dev              # Development settings
│   └── .env.prod.template    # Production template (edit first!)
└── scripts/                   # Deployment management scripts
    ├── deploy.sh             # Main deployment script
    ├── setup.sh              # First-time setup
    └── health-check.sh       # Health monitoring
```

## 🛠️ Deployment Scripts

### Main Deployment Script (`deploy.sh`)

Manages all deployment operations:

```bash
# Start environments
./deploy/scripts/deploy.sh up local     # Local development
./deploy/scripts/deploy.sh up dev       # Development with MinIO
./deploy/scripts/deploy.sh up prod      # Production with Nginx

# Stop environments
./deploy/scripts/deploy.sh down <env>

# Restart services
./deploy/scripts/deploy.sh restart <env> [service...]

# View logs
./deploy/scripts/deploy.sh logs <env> [service...]

# Check running services
./deploy/scripts/deploy.sh ps <env>

# Build/update images
./deploy/scripts/deploy.sh build <env>
./deploy/scripts/deploy.sh pull <env>

# Clean up (removes containers and volumes)
./deploy/scripts/deploy.sh clean <env>

# Health check
./deploy/scripts/deploy.sh health <env>
```

### Setup Script (`setup.sh`)

First-time environment setup:

```bash
./deploy/scripts/setup.sh <environment> [options]

# Options:
#   --skip-build    Skip building Docker images
#   --skip-prereq   Skip prerequisite checks
```

### Health Check Script (`health-check.sh`)

Comprehensive health monitoring:

```bash
./deploy/scripts/health-check.sh [options]

# Options:
#   -e, --environment ENV    Specify environment (auto-detects by default)
#   -v, --verbose           Show detailed output
```

## 🌍 Environments

### Local Development (`local`)

- **Purpose**: Quick local testing, minimal resource usage
- **Services**: PostgreSQL, Redis, API Gateway
- **Ports**: 5432 (PostgreSQL), 6379 (Redis), 8080 (API)
- **Storage**: Local filesystem
- **Security**: Basic (development-friendly passwords)

### Development (`dev`)

- **Purpose**: Full-featured development with S3 simulation
- **Services**: PostgreSQL, Redis, API Gateway, MinIO
- **Ports**: 5432, 6379, 8080, 9000/9001 (MinIO)
- **Storage**: Local + MinIO S3 simulation
- **Features**: Debug logging, exposed database ports

### Production (`prod`)

- **Purpose**: Production deployment with security and performance
- **Services**: PostgreSQL, Redis, API Gateway, Nginx, Certbot
- **Ports**: 80 (HTTP), 443 (HTTPS), 8080 (API - internal)
- **Storage**: S3 (recommended) or local
- **Features**: SSL/TLS, rate limiting, optimized settings

## ⚙️ Configuration

### Environment Files

Each environment has a corresponding `.env` template in `deploy/environments/`:

- **`.env.local`**: Ready-to-use local development settings
- **`.env.dev`**: Development settings with MinIO
- **`.env.prod.template`**: Production template requiring customization

### Customizing Production

For production deployment, you **must** edit the `.env` file:

```bash
# Copy the production template
cp deploy/environments/.env.prod.template .env

# Edit with your settings
nano .env
```

**Required production changes:**
- Set strong passwords for `POSTGRES_PASSWORD` and `REDIS_PASSWORD`
- Set a secure `JWT_SECRET` (64+ characters)
- Configure S3 storage settings
- Set your domain in `CORS_ORIGINS`
- Set `CERTBOT_EMAIL` for SSL certificates

## 🔒 SSL/TLS Setup (Production)

### Option 1: Automatic (Let's Encrypt)

```bash
# Edit .env with your domain and email
nano .env

# Start with SSL profile
./deploy/scripts/deploy.sh up prod --profile ssl
```

### Option 2: Custom Certificates

```bash
# Copy your certificates
cp your-cert.crt ssl/certs/lodestone.crt
cp your-key.key ssl/private/lodestone.key

# Update nginx configuration if needed
nano deploy/configs/nginx/conf.d/ssl.conf.example

# Start production
./deploy/scripts/deploy.sh up prod
```

## 📊 Monitoring

### Health Checks

```bash
# Auto-detect environment and check health
./deploy/scripts/health-check.sh

# Check specific environment
./deploy/scripts/health-check.sh -e prod -v
```

### Logs

```bash
# View all logs
./deploy/scripts/deploy.sh logs <env>

# View specific service logs
./deploy/scripts/deploy.sh logs <env> api-gateway

# Follow logs in real-time
./deploy/scripts/deploy.sh logs <env> -f
```

### Container Status

```bash
# Show running containers
./deploy/scripts/deploy.sh ps <env>

# Show resource usage
docker stats
```

## 🐛 Troubleshooting

### Common Issues

**Services won't start:**
```bash
# Check prerequisites
./deploy/scripts/setup.sh <env> --skip-build

# Check container logs
./deploy/scripts/deploy.sh logs <env>

# Restart services
./deploy/scripts/deploy.sh restart <env>
```

**Database connection errors:**
```bash
# Check if database is running
./deploy/scripts/deploy.sh ps <env>

# Restart database
./deploy/scripts/deploy.sh restart <env> postgres

# Check database logs
./deploy/scripts/deploy.sh logs <env> postgres
```

**SSL certificate issues:**
```bash
# Check certificate files
ls -la ssl/certs/ ssl/private/

# Restart nginx
./deploy/scripts/deploy.sh restart prod nginx

# Check nginx logs
./deploy/scripts/deploy.sh logs prod nginx
```

### Reset Everything

```bash
# Complete cleanup (removes all data!)
./deploy/scripts/deploy.sh clean <env>

# Fresh setup
./deploy/scripts/setup.sh <env>
./deploy/scripts/deploy.sh up <env>
```

## 🔧 Advanced Usage

### Custom Services

Add services to `docker-compose.yml` or environment-specific override files.

### Scaling

```bash
# Scale API Gateway instances
docker-compose -f deploy/compose/docker-compose.yml \
               -f deploy/compose/docker-compose.prod.yml \
               up -d --scale api-gateway=3
```

### Development Workflow

```bash
# Start development environment
./deploy/scripts/deploy.sh up dev

# Make code changes...

# Rebuild and restart API
./deploy/scripts/deploy.sh build dev api-gateway
./deploy/scripts/deploy.sh restart dev api-gateway

# Check health
./deploy/scripts/health-check.sh -v
```
