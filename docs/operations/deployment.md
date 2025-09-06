# Deployment Guide

Comprehensive guide for deploying LDAP Manager in production environments, covering various deployment strategies, configurations, and operational best practices.

## Overview

LDAP Manager is designed for production deployment with enterprise-grade features:

- **Stateless Architecture**: Scales horizontally with external session storage
- **Container-First**: Optimized Docker images with security best practices
- **Configuration Flexibility**: Environment variables, files, or orchestration configs
- **High Availability**: Load balancer ready with health checks
- **Security**: HTTPS termination, secure session management, LDAP over TLS

## Deployment Options

### 1. Docker Container (Recommended)

The simplest production deployment uses the official Docker image with persistent session storage.

#### Basic Production Setup

```bash
# Create data directory for persistent sessions
mkdir -p /opt/ldap-manager/data

# Create configuration
cat > /opt/ldap-manager/.env << EOF
LDAP_SERVER=ldaps://dc1.company.com:636
LDAP_BASE_DN=DC=company,DC=com
LDAP_READONLY_USER=svc_ldap_readonly@company.com
LDAP_READONLY_PASSWORD=secure_password_here
LDAP_IS_AD=true
LOG_LEVEL=warn
PERSIST_SESSIONS=true
SESSION_PATH=/data/sessions.bbolt
SESSION_DURATION=30m
EOF

# Run container
docker run -d \
  --name ldap-manager \
  --restart unless-stopped \
  -p 3000:3000 \
  --env-file /opt/ldap-manager/.env \
  -v /opt/ldap-manager/data:/data \
  -v /etc/ssl/certs:/etc/ssl/certs:ro \
  ghcr.io/netresearch/ldap-manager:latest
```

#### Docker Compose Production

```yaml
# compose.yml

services:
  ldap-manager:
    image: ghcr.io/netresearch/ldap-manager:latest
    container_name: ldap-manager
    restart: unless-stopped
    
    environment:
      LDAP_SERVER: ldaps://dc1.company.com:636
      LDAP_BASE_DN: DC=company,DC=com
      LDAP_READONLY_USER: svc_ldap_readonly@company.com
      LDAP_READONLY_PASSWORD: ${LDAP_PASSWORD}  # From environment
      LDAP_IS_AD: "true"
      LOG_LEVEL: warn
      PERSIST_SESSIONS: "true"
      SESSION_PATH: /data/sessions.bbolt
      SESSION_DURATION: 30m
    
    ports:
      - "127.0.0.1:3000:3000"  # Bind to localhost only
    
    volumes:
      - ./data:/data:rw
      - /etc/ssl/certs:/etc/ssl/certs:ro
      - ./logs:/app/logs:rw
    
    healthcheck:
      test: ["CMD", "wget", "--quiet", "--tries=1", "--spider", "http://localhost:3000/"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 40s
    
    # Security constraints
    read_only: true
    tmpfs:
      - /tmp:rw,size=100M
    cap_drop:
      - ALL
    cap_add:
      - CHOWN
      - DAC_OVERRIDE
    user: "1000:1000"  # Run as non-root user

  # Optional: Reverse proxy with SSL termination
  nginx:
    image: nginx:alpine
    container_name: ldap-manager-nginx
    restart: unless-stopped
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./nginx.conf:/etc/nginx/nginx.conf:ro
      - ./ssl:/etc/nginx/ssl:ro
    depends_on:
      - ldap-manager
```

**Start deployment:**
```bash
# Set password via environment
export LDAP_PASSWORD="your_secure_password"

# Deploy
docker compose up -d

# Check status
docker compose ps
docker compose logs ldap-manager
```

### 2. Kubernetes Deployment

For container orchestration environments.

#### Kubernetes Manifests

**Namespace and ConfigMap:**
```yaml
# namespace.yaml
apiVersion: v1
kind: Namespace
metadata:
  name: ldap-manager
  labels:
    name: ldap-manager

---
# configmap.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: ldap-manager-config
  namespace: ldap-manager
data:
  LDAP_SERVER: "ldaps://dc1.company.com:636"
  LDAP_BASE_DN: "DC=company,DC=com"
  LDAP_IS_AD: "true"
  LOG_LEVEL: "warn"
  PERSIST_SESSIONS: "true"
  SESSION_PATH: "/data/sessions.bbolt"
  SESSION_DURATION: "30m"
```

**Secret for Credentials:**
```yaml
# secret.yaml
apiVersion: v1
kind: Secret
metadata:
  name: ldap-manager-secrets
  namespace: ldap-manager
type: Opaque
data:
  LDAP_READONLY_USER: c3ZjX2xkYXBfcmVhZG9ubHlAY29tcGFueS5jb20=  # base64 encoded
  LDAP_READONLY_PASSWORD: eW91cl9zZWN1cmVfcGFzc3dvcmQ=  # base64 encoded
```

**Deployment:**
```yaml
# deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: ldap-manager
  namespace: ldap-manager
  labels:
    app: ldap-manager
spec:
  replicas: 2
  selector:
    matchLabels:
      app: ldap-manager
  template:
    metadata:
      labels:
        app: ldap-manager
    spec:
      containers:
      - name: ldap-manager
        image: ghcr.io/netresearch/ldap-manager:latest
        imagePullPolicy: Always
        
        ports:
        - containerPort: 3000
          name: http
        
        envFrom:
        - configMapRef:
            name: ldap-manager-config
        - secretRef:
            name: ldap-manager-secrets
        
        volumeMounts:
        - name: session-storage
          mountPath: /data
        - name: ca-certs
          mountPath: /etc/ssl/certs
          readOnly: true
        
        resources:
          requests:
            memory: "128Mi"
            cpu: "100m"
          limits:
            memory: "512Mi"
            cpu: "500m"
        
        livenessProbe:
          httpGet:
            path: /
            port: 3000
          initialDelaySeconds: 30
          periodSeconds: 10
        
        readinessProbe:
          httpGet:
            path: /
            port: 3000
          initialDelaySeconds: 5
          periodSeconds: 5
        
        securityContext:
          allowPrivilegeEscalation: false
          readOnlyRootFilesystem: true
          runAsNonRoot: true
          runAsUser: 1000
          capabilities:
            drop:
            - ALL
      
      volumes:
      - name: session-storage
        persistentVolumeClaim:
          claimName: ldap-manager-sessions
      - name: ca-certs
        configMap:
          name: ca-certificates  # Your CA certificates ConfigMap
      
      securityContext:
        fsGroup: 1000
```

**Service and Ingress:**
```yaml
# service.yaml
apiVersion: v1
kind: Service
metadata:
  name: ldap-manager-service
  namespace: ldap-manager
spec:
  selector:
    app: ldap-manager
  ports:
  - port: 80
    targetPort: 3000
    protocol: TCP
  type: ClusterIP

---
# ingress.yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: ldap-manager-ingress
  namespace: ldap-manager
  annotations:
    cert-manager.io/cluster-issuer: "letsencrypt-prod"
    nginx.ingress.kubernetes.io/ssl-redirect: "true"
    nginx.ingress.kubernetes.io/force-ssl-redirect: "true"
spec:
  tls:
  - hosts:
    - ldap.company.com
    secretName: ldap-manager-tls
  rules:
  - host: ldap.company.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: ldap-manager-service
            port:
              number: 80
```

**Persistent Volume:**
```yaml
# pvc.yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: ldap-manager-sessions
  namespace: ldap-manager
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 1Gi
  storageClassName: ssd  # Use appropriate storage class
```

**Deploy to Kubernetes:**
```bash
# Apply manifests
kubectl apply -f namespace.yaml
kubectl apply -f configmap.yaml
kubectl apply -f secret.yaml
kubectl apply -f pvc.yaml
kubectl apply -f deployment.yaml
kubectl apply -f service.yaml
kubectl apply -f ingress.yaml

# Check deployment status
kubectl get pods -n ldap-manager
kubectl get svc -n ldap-manager
kubectl logs -n ldap-manager deployment/ldap-manager
```

### 3. Systemd Service

For traditional Linux server deployments.

#### Installation

```bash
# Create user and directory
sudo useradd --system --create-home --home-dir /opt/ldap-manager ldap-manager

# Install binary
sudo cp ldap-manager /opt/ldap-manager/
sudo cp -r internal/web/static /opt/ldap-manager/static/
sudo chown -R ldap-manager:ldap-manager /opt/ldap-manager
sudo chmod +x /opt/ldap-manager/ldap-manager
```

#### Configuration

```bash
# Create production configuration
sudo -u ldap-manager tee /opt/ldap-manager/.env.local << EOF
LDAP_SERVER=ldaps://dc1.company.com:636
LDAP_BASE_DN=DC=company,DC=com
LDAP_READONLY_USER=svc_ldap_readonly@company.com
LDAP_READONLY_PASSWORD=secure_password
LDAP_IS_AD=true
LOG_LEVEL=warn
PERSIST_SESSIONS=true
SESSION_PATH=/opt/ldap-manager/sessions.bbolt
SESSION_DURATION=30m
LISTEN_ADDR=127.0.0.1:3000
EOF

# Secure configuration file
sudo chmod 600 /opt/ldap-manager/.env.local
```

#### Systemd Service

```bash
# Create systemd service
sudo tee /etc/systemd/system/ldap-manager.service << EOF
[Unit]
Description=LDAP Manager - Web-based LDAP directory management
Documentation=https://github.com/netresearch/ldap-manager
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=ldap-manager
Group=ldap-manager
WorkingDirectory=/opt/ldap-manager
ExecStart=/opt/ldap-manager/ldap-manager
EnvironmentFile=/opt/ldap-manager/.env.local

# Restart configuration
Restart=on-failure
RestartSec=5
TimeoutStopSec=30

# Security settings
NoNewPrivileges=yes
PrivateTmp=yes
ProtectSystem=strict
ReadWritePaths=/opt/ldap-manager
ProtectHome=yes
ProtectKernelTunables=yes
ProtectKernelModules=yes
ProtectControlGroups=yes

# Resource limits
LimitNOFILE=65536
LimitNPROC=4096

[Install]
WantedBy=multi-user.target
EOF

# Enable and start service
sudo systemctl daemon-reload
sudo systemctl enable ldap-manager
sudo systemctl start ldap-manager

# Check status
sudo systemctl status ldap-manager
sudo journalctl -u ldap-manager -f
```

## Reverse Proxy Configuration

### Nginx (Recommended)

```nginx
# /etc/nginx/sites-available/ldap-manager
upstream ldap_manager {
    server 127.0.0.1:3000;
    keepalive 32;
}

# HTTP redirect to HTTPS
server {
    listen 80;
    server_name ldap.company.com;
    return 301 https://$server_name$request_uri;
}

# HTTPS server
server {
    listen 443 ssl http2;
    server_name ldap.company.com;
    
    # SSL configuration
    ssl_certificate /etc/ssl/certs/ldap.company.com.crt;
    ssl_certificate_key /etc/ssl/private/ldap.company.com.key;
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers ECDHE-RSA-AES128-GCM-SHA256:ECDHE-RSA-AES256-GCM-SHA384;
    ssl_prefer_server_ciphers off;
    
    # Security headers
    add_header Strict-Transport-Security "max-age=63072000" always;
    add_header X-Frame-Options DENY;
    add_header X-Content-Type-Options nosniff;
    add_header Referrer-Policy "strict-origin-when-cross-origin";
    
    # Logging
    access_log /var/log/nginx/ldap-manager_access.log;
    error_log /var/log/nginx/ldap-manager_error.log;
    
    location / {
        proxy_pass http://ldap_manager;
        proxy_http_version 1.1;
        
        # Headers
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header Connection "";
        
        # Timeouts
        proxy_connect_timeout 30s;
        proxy_send_timeout 30s;
        proxy_read_timeout 30s;
        
        # Buffering
        proxy_buffering on;
        proxy_buffer_size 4k;
        proxy_buffers 8 4k;
    }
    
    # Static assets with long caching
    location /static/ {
        proxy_pass http://ldap_manager;
        proxy_cache_valid 200 1d;
        expires 1d;
        add_header Cache-Control "public, immutable";
    }
}
```

### Apache

```apache
# /etc/apache2/sites-available/ldap-manager.conf
<VirtualHost *:80>
    ServerName ldap.company.com
    Redirect permanent / https://ldap.company.com/
</VirtualHost>

<VirtualHost *:443>
    ServerName ldap.company.com
    
    # SSL configuration
    SSLEngine on
    SSLCertificateFile /etc/ssl/certs/ldap.company.com.crt
    SSLCertificateKeyFile /etc/ssl/private/ldap.company.com.key
    SSLProtocol all -SSLv3 -TLSv1 -TLSv1.1
    
    # Security headers
    Header always set Strict-Transport-Security "max-age=63072000"
    Header always set X-Frame-Options DENY
    Header always set X-Content-Type-Options nosniff
    
    # Proxy configuration
    ProxyPreserveHost On
    ProxyRequests Off
    
    ProxyPass / http://127.0.0.1:3000/
    ProxyPassReverse / http://127.0.0.1:3000/
    
    # Static asset caching
    <LocationMatch "/static/">
        ExpiresActive On
        ExpiresDefault "access plus 1 day"
        Header append Cache-Control "public, immutable"
    </LocationMatch>
    
    # Logging
    CustomLog /var/log/apache2/ldap-manager_access.log combined
    ErrorLog /var/log/apache2/ldap-manager_error.log
</VirtualHost>
```

### Traefik

```yaml
# traefik/compose.yml
version: '3.8'

services:
  ldap-manager:
    image: ghcr.io/netresearch/ldap-manager:latest
    environment:
      # Your LDAP configuration
    networks:
      - web
    labels:
      - "traefik.enable=true"
      - "traefik.http.routers.ldap-manager.rule=Host(`ldap.company.com`)"
      - "traefik.http.routers.ldap-manager.tls=true"
      - "traefik.http.routers.ldap-manager.tls.certresolver=letsencrypt"
      - "traefik.http.services.ldap-manager.loadbalancer.server.port=3000"
      
      # Security headers
      - "traefik.http.middlewares.ldap-security.headers.stsSeconds=63072000"
      - "traefik.http.middlewares.ldap-security.headers.frameDeny=true"
      - "traefik.http.middlewares.ldap-security.headers.contentTypeNosniff=true"
      - "traefik.http.routers.ldap-manager.middlewares=ldap-security"

networks:
  web:
    external: true
```

## High Availability Setup

### Load Balanced Deployment

For high availability, deploy multiple instances behind a load balancer:

```yaml
# compose-ha.yml
version: '3.8'

services:
  ldap-manager-1:
    image: ghcr.io/netresearch/ldap-manager:latest
    environment: &ldap-env
      LDAP_SERVER: ldaps://dc1.company.com:636
      LDAP_BASE_DN: DC=company,DC=com
      LDAP_READONLY_USER: svc_ldap_readonly@company.com
      LDAP_READONLY_PASSWORD: ${LDAP_PASSWORD}
      LDAP_IS_AD: "true"
      PERSIST_SESSIONS: "true"
      SESSION_PATH: /data/sessions.bbolt
    volumes:
      - sessions:/data
    networks:
      - ldap-net
    
  ldap-manager-2:
    image: ghcr.io/netresearch/ldap-manager:latest
    environment: *ldap-env
    volumes:
      - sessions:/data  # Shared session storage
    networks:
      - ldap-net

  # Load balancer
  nginx-lb:
    image: nginx:alpine
    ports:
      - "443:443"
    volumes:
      - ./nginx-lb.conf:/etc/nginx/nginx.conf:ro
    networks:
      - ldap-net
    depends_on:
      - ldap-manager-1
      - ldap-manager-2

volumes:
  sessions:
    driver: local

networks:
  ldap-net:
    driver: bridge
```

**Nginx Load Balancer Config:**
```nginx
# nginx-lb.conf
upstream ldap_cluster {
    least_conn;
    server ldap-manager-1:3000 max_fails=3 fail_timeout=30s;
    server ldap-manager-2:3000 max_fails=3 fail_timeout=30s;
}

server {
    listen 443 ssl http2;
    
    location / {
        proxy_pass http://ldap_cluster;
        # Sticky sessions for memory-based sessions
        ip_hash;  # Remove if using persistent sessions
        
        # Health checks
        proxy_next_upstream error timeout invalid_header http_500 http_502 http_503;
    }
}
```

### Database Session Storage

For true stateless deployment, use external session storage:

```bash
# Option 1: Redis session store (requires custom implementation)
# Option 2: Database session store (requires custom implementation)
# Option 3: BBolt on shared storage (current implementation)

# Shared storage example with NFS
docker run -d \
  --name ldap-manager \
  -v /nfs/ldap-sessions:/data \
  -e SESSION_PATH=/data/sessions.bbolt \
  ghcr.io/netresearch/ldap-manager:latest
```

## Security Hardening

### Container Security

```dockerfile
# Production Dockerfile with security hardening
FROM golang:1.23-alpine AS builder
RUN apk add --no-cache git make
WORKDIR /build
COPY . .
RUN make build-release

FROM scratch
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /build/ldap-manager /ldap-manager
COPY --from=builder /etc/passwd /etc/passwd
USER nobody
EXPOSE 3000
ENTRYPOINT ["/ldap-manager"]
```

### Network Security

```bash
# Firewall rules (iptables)
# Allow only necessary ports
iptables -A INPUT -p tcp --dport 22 -j ACCEPT    # SSH
iptables -A INPUT -p tcp --dport 80 -j ACCEPT    # HTTP redirect
iptables -A INPUT -p tcp --dport 443 -j ACCEPT   # HTTPS
iptables -A INPUT -p tcp --dport 636 -j ACCEPT   # LDAPS outbound
iptables -A INPUT -j DROP

# Or using ufw
ufw allow 22
ufw allow 80
ufw allow 443
ufw allow out 636
ufw --force enable
```

### SSL/TLS Configuration

**Generate Strong SSL Configuration:**
```bash
# Generate strong DH parameters
openssl dhparam -out /etc/ssl/dhparam.pem 4096

# SSL cipher configuration for nginx
ssl_dhparam /etc/ssl/dhparam.pem;
ssl_ciphers ECDHE-RSA-AES256-GCM-SHA512:DHE-RSA-AES256-GCM-SHA512;
ssl_ecdh_curve secp384r1;
ssl_session_timeout 10m;
ssl_session_cache shared:SSL:10m;
ssl_session_tickets off;
ssl_stapling on;
ssl_stapling_verify on;
```

## Monitoring and Logging

### Application Monitoring

```bash
# Health check endpoint
curl -f https://ldap.company.com/ > /dev/null

# Detailed health check script
#!/bin/bash
HEALTH_URL="https://ldap.company.com/"
RESPONSE=$(curl -s -o /dev/null -w "%{http_code}:%{time_total}" "$HEALTH_URL")
STATUS_CODE=$(echo $RESPONSE | cut -d: -f1)
RESPONSE_TIME=$(echo $RESPONSE | cut -d: -f2)

if [ "$STATUS_CODE" = "200" ] || [ "$STATUS_CODE" = "302" ]; then
    echo "OK - LDAP Manager healthy (${RESPONSE_TIME}s)"
    exit 0
else
    echo "CRITICAL - LDAP Manager unhealthy (HTTP $STATUS_CODE)"
    exit 2
fi
```

### Log Management

**Centralized Logging with Docker:**
```yaml
services:
  ldap-manager:
    # ... other config
    logging:
      driver: "json-file"
      options:
        max-size: "10m"
        max-file: "3"
        labels: "service=ldap-manager"
```

**Logrotate Configuration:**
```bash
# /etc/logrotate.d/ldap-manager
/var/log/ldap-manager/*.log {
    daily
    rotate 30
    compress
    delaycompress
    missingok
    notifempty
    copytruncate
}
```

### Metrics Collection

**Prometheus Metrics (requires custom implementation):**
```yaml
# prometheus.yml
scrape_configs:
  - job_name: 'ldap-manager'
    static_configs:
      - targets: ['ldap.company.com:9090']  # Metrics endpoint
    scrape_interval: 30s
    metrics_path: /metrics
```

## Backup and Recovery

### Session Data Backup

```bash
#!/bin/bash
# backup-sessions.sh
BACKUP_DIR="/backup/ldap-manager"
SESSION_FILE="/opt/ldap-manager/sessions.bbolt"
DATE=$(date +%Y%m%d-%H%M%S)

mkdir -p "$BACKUP_DIR"
cp "$SESSION_FILE" "$BACKUP_DIR/sessions-$DATE.bbolt"

# Keep only last 7 days of backups
find "$BACKUP_DIR" -name "sessions-*.bbolt" -mtime +7 -delete
```

### Configuration Backup

```bash
#!/bin/bash
# backup-config.sh
tar -czf "/backup/ldap-manager-config-$(date +%Y%m%d).tar.gz" \
    /opt/ldap-manager/.env.local \
    /etc/systemd/system/ldap-manager.service \
    /etc/nginx/sites-available/ldap-manager
```

## Troubleshooting

### Common Deployment Issues

**Container Won't Start:**
```bash
# Check logs
docker logs ldap-manager

# Common issues:
# - Missing environment variables
# - LDAP connection failures
# - Permission issues with volume mounts
# - Port conflicts
```

**LDAP Connection Issues:**
```bash
# Test LDAP connectivity from container
docker exec ldap-manager sh -c "echo | openssl s_client -connect dc1.company.com:636"

# Test authentication
docker exec ldap-manager ldapsearch -H ldaps://dc1.company.com:636 \
    -D "svc_ldap_readonly@company.com" -w "password" -b "DC=company,DC=com" -s base
```

**Performance Issues:**
```bash
# Check resource usage
docker stats ldap-manager

# Monitor application logs for slow queries
docker logs ldap-manager 2>&1 | grep -E "(slow|timeout|error)"

# Check LDAP server performance
```

### Disaster Recovery

**Complete System Recovery:**
1. Restore configuration from backup
2. Restore session database
3. Recreate containers/services
4. Verify LDAP connectivity
5. Test authentication flow

**Rollback Procedure:**
```bash
# Docker rollback
docker compose down
docker compose -f compose.yml.backup up -d

# Systemd rollback
sudo systemctl stop ldap-manager
sudo cp /opt/ldap-manager/ldap-manager.backup /opt/ldap-manager/ldap-manager
sudo systemctl start ldap-manager
```

This deployment guide provides production-ready configurations for various environments. Choose the deployment method that best fits your infrastructure and security requirements.