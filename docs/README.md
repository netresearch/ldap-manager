# LDAP Manager Documentation

Welcome to the LDAP Manager documentation. This guide provides comprehensive information for users, developers, and operators working with LDAP Manager.

## About LDAP Manager

LDAP Manager is a web-based frontend that allows users to administrate LDAP directory entries including users, groups, and computers. Built with Go and modern web technologies, it provides a secure, performant interface for LDAP directory management.

## Documentation Structure

### User Guide

For end users and system administrators:

- **[Installation Guide](user-guide/installation.md)** - Setup and deployment instructions
- **[Configuration Reference](user-guide/configuration.md)** - Complete configuration options and examples
- **[API Documentation](user-guide/api.md)** - REST API endpoints and usage
- **[Implementation Examples](user-guide/implementation-examples.md)** - Practical tutorials and real-world scenarios

### Development

For developers contributing to the project:

- **[Development Setup](development/setup.md)** - Local development environment setup
- **[Contributing Guidelines](development/contributing.md)** - Code standards and contribution process
- **[Architecture Overview](development/architecture.md)** - System design and technical architecture
- **[Detailed Architecture Guide](development/architecture-detailed.md)** - Comprehensive technical architecture
- **[Go Documentation Reference](development/go-doc-reference.md)** - Complete package documentation

### Operations

For DevOps and system administrators:

- **[Deployment Guide](operations/deployment.md)** - Production deployment strategies
- **[Monitoring & Troubleshooting](operations/monitoring.md)** - Operational procedures and diagnostics
- **[Performance Optimization](operations/performance-optimization.md)** - Performance tuning and optimization
- **[Security Configuration](operations/security-configuration.md)** - Security best practices and hardening
- **[Troubleshooting Guide](operations/troubleshooting.md)** - Comprehensive problem resolution

## Quick Start

### For Users

1. See the [Installation Guide](user-guide/installation.md) for setup instructions
2. Configure your LDAP connection using the [Configuration Reference](user-guide/configuration.md)
3. Deploy using Docker or build from source

### For Developers

1. Follow the [Development Setup](development/setup.md) guide
2. Review [Contributing Guidelines](development/contributing.md) before making changes
3. Understand the [Architecture](development/architecture.md) for code organization

## Features

- **Web-based LDAP Management**: Intuitive interface for directory operations
- **Multi-Directory Support**: Works with standard LDAP and Active Directory
- **Secure Authentication**: Session-based security with configurable storage
- **High Performance**: Efficient caching and connection pooling
- **Modern Architecture**: Built with Go, Fiber, and TailwindCSS

## Support

- **Issues**: Report bugs and feature requests on the project repository
- **Documentation**: This documentation covers all aspects of installation, configuration, and development
- **Community**: Contribute to the project following our [Contributing Guidelines](development/contributing.md)

## License

LDAP Manager is licensed under the MIT license. For more information, see the [LICENSE](../LICENSE) file in the project root.

---

Last updated: September 2025 | [Edit on GitHub](../../docs/)
