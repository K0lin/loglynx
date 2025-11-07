# Security Policy

## Supported Versions

We release patches for security vulnerabilities in the latest version.

## Reporting a Vulnerability

We take the security of LogLynx seriously. If you believe you have found a security vulnerability, please report it to us as described below.

### Please DO:

1. **Open a GitHub issue** for security vulnerabilities
2. **Provide detailed information** including:
   - Type of vulnerability (e.g., SQL injection, XSS)
   - Full paths of source file(s) related to the vulnerability
   - Location of the affected source code (tag/branch/commit or direct URL)
   - Step-by-step instructions to reproduce the issue
   - Proof-of-concept or exploit code (if possible)
   - Impact of the vulnerability and how an attacker might exploit it

### What to Expect:

- **Initial Response**: We will acknowledge receipt of your vulnerability report within 48 hours
- **Status Updates**: We will send you regular updates about our progress (at least every 5 business days)
- **Disclosure Timeline**: We aim to address critical vulnerabilities within 7-14 days
- **Credit**: We will credit you in our security advisory (unless you prefer to remain anonymous)

## Security Best Practices for Users

### Database Security

1. **Access Control**
   - Restrict file system access to the SQLite database file
   - Use proper file permissions (600 or 640)
   - Never expose the database file via web server

2. **Network Security**
   - Run LogLynx behind a reverse proxy (Traefik, Nginx, Caddy)
   - Use HTTPS/TLS for all connections
   - Implement authentication at the proxy level if needed
   - Restrict access by IP/network when possible

3. **Log File Security**
   - Ensure log files don't contain sensitive information (passwords, tokens, etc.)
   - Use log rotation to manage file sizes
   - Restrict read access to log directories

### Docker Security

1. **Container Hardening**
   - Run container as non-root user (already implemented in Dockerfile)
   - Use read-only file systems where possible
   - Limit container resources (memory, CPU)

2. **Secrets Management**
   - Never commit `.env` files to version control
   - Use Docker secrets for sensitive configuration
   - Rotate credentials regularly

3. **Image Security**
   - Use official base images (Alpine)
   - Keep images updated regularly
   - Scan images for vulnerabilities (`docker scan`)

### Configuration Security

1. **Environment Variables**
   ```bash
   # Production settings
   SERVER_PRODUCTION=true
   LOG_LEVEL=warn  # Don't use 'trace' or 'debug' in production
   ```

2. **Database Configuration**
   - Enable WAL mode for better concurrency (enabled by default)
   - Set appropriate connection pool limits
   - Monitor for database locks

3. **GeoIP Database**
   - Keep MaxMind GeoLite2 databases updated
   - Download from official sources only

## Known Security Considerations

### SQLite Limitations

- **Concurrent Writes**: SQLite has limited concurrent write support
  - Use connection pooling (implemented)
  - Monitor for database locks
  - Consider PostgreSQL/MySQL for high-write workloads

- **File-Based**: Database is a file on disk
  - Ensure proper file permissions
  - Implement regular backups
  - Protect against unauthorized access

### Input Validation

- **Log Parsing**: Input validation is performed on parsed log entries
  - Status codes are validated (0-599)
  - Request schemes are validated (http, https, ws, wss)
  - IP addresses are validated for GeoIP lookup

- **API Endpoints**: All API endpoints validate input parameters
  - Query parameters are sanitized
  - Pagination limits are enforced
  - SQL injection protection via ORM (GORM)

### Dependencies

We regularly update dependencies to address security vulnerabilities:

- **Go Modules**: Updated regularly via `go get -u`
- **MaxMind GeoIP**: Update databases monthly
- **JavaScript Libraries**: Frontend uses minimal dependencies

## Security Features

### Implemented Protections

1. **SQL Injection Protection**
   - GORM ORM with parameterized queries
   - No raw SQL concatenation

2. **Path Traversal Protection**
   - File paths are validated and sanitized
   - No user-controllable file paths

3. **Resource Limits**
   - Connection pool limits prevent resource exhaustion
   - Query timeouts prevent long-running queries
   - Batch size limits for inserts

4. **Rate Limiting** (Recommended)
   - Implement at reverse proxy level
   - Use tools like Traefik, Nginx rate limiting
   - Protect against DoS attacks

### Logging Security

- **Sensitive Data**: No sensitive data is logged by default
- **Log Levels**: Configurable via `LOG_LEVEL` environment variable
- **Audit Trail**: All API requests are logged by Gin framework

## Vulnerability Disclosure Policy

When we receive a security vulnerability report, we will:

1. **Confirm** the vulnerability and determine its severity
2. **Develop** a fix in a private repository
3. **Notify** affected users if the vulnerability is critical
4. **Release** a patched version
5. **Publish** a security advisory on GitHub
6. **Credit** the reporter (with permission)

## Security Updates

Security updates are released as patch versions and announced via:

- GitHub Discussion
- Release notes
- README.md updates

Subscribe to GitHub repository notifications to stay informed.

**Last Updated**: November 2025
