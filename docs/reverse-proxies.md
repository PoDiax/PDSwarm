# Reverse Proxy & Security Guide

PDSwarm includes built-in protection against IP address spoofing and native support for all major reverse proxies: **Cloudflare (and Cloudflare Tunnels)**, **Nginx**, **Caddy**, and **Traefik**.

---

## 1. How Client IP Resolution Works

When PDSwarm is running behind a reverse proxy, the client's direct TCP connection originates from the proxy itself (e.g., `127.0.0.1`, a Docker bridge IP like `172.18.0.2`, or a Cloudflare edge IP).

To prevent malicious users from spoofing their IP by sending custom `X-Forwarded-For` or `CF-Connecting-IP` headers directly to PDSwarm, PDSwarm enforces **Trusted Proxy CIDR Verification**:
* Forwarding headers are **only** evaluated if the incoming TCP connection matches a configured trusted proxy range.
* If a request originates from an untrusted address, all forwarding headers are ignored, and the socket IP is used.
* This ensures that security audit logs (`auth.log`), rate limiters, and jail tools (Fail2ban, CrowdSec) always record and ban the real perpetrator, not a spoofed IP.

### Default Trusted Proxy Ranges

By default, PDSwarm trusts standard private and loopback networks:
```
127.0.0.0/8, 10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16, ::1/128
```

You can customize this list using the `--trusted-proxies` flag or the `PDSWARM_TRUSTED_PROXIES` environment variable:
```bash
./pdswarm -trusted-proxies "127.0.0.1/32,172.18.0.0/16"
```

---

## 2. Reverse Proxy Configurations

### Cloudflare & Cloudflare Tunnels (`cloudflared`)

When using Cloudflare, requests carry the client's real IP in the `CF-Connecting-IP` header.

#### Cloudflare Tunnels (`cloudflared`)
If you run `cloudflared` locally on the same host or in Docker, it connects to PDSwarm over localhost or a Docker network bridge.

**`config.yml` example for `cloudflared`:**
```yaml
tunnel: <YOUR-TUNNEL-UUID>
credentials-file: /etc/cloudflared/<YOUR-TUNNEL-UUID>.json

ingress:
  - hostname: pdswarm.yourdomain.com
    service: http://localhost:9000
  - service: http_status:404
```

Because `cloudflared` connects from localhost or a private Docker IP, PDSwarm's default trusted proxies will automatically trust it and extract the genuine `CF-Connecting-IP`.

#### Cloudflare Proxy (Standard DNS Orange Cloud)
If PDSwarm directly receives traffic from Cloudflare's public edge network, configure PDSwarm to trust Cloudflare's official IP ranges:
```bash
PDSWARM_TRUSTED_PROXIES="173.245.48.0/20,103.21.244.0/22,103.22.200.0/22,103.31.4.0/22,141.101.64.0/18,108.162.192.0/18,190.93.240.0/20,188.114.96.0/20,197.234.240.0/22,198.41.128.0/17,162.158.0.0/15,104.16.0.0/13,104.24.0.0/14,172.64.0.0/13,131.0.72.0/22"
```

---

### Nginx

Make sure to pass WebSocket upgrade headers for live terminals and log streams.

```nginx
server {
    listen 80;
    server_name pdswarm.example.com;
    return 301 https://$host$request_uri;
}

server {
    listen 443 ssl http2;
    server_name pdswarm.example.com;

    ssl_certificate     /etc/letsencrypt/live/pdswarm.example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/pdswarm.example.com/privkey.pem;

    location / {
        proxy_pass http://127.0.0.1:9000;
        proxy_http_version 1.1;

        # WebSocket support
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";

        # Forwarded IP headers
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        # Timeouts for interactive terminal sessions
        proxy_read_timeout 86400s;
        proxy_send_timeout 86400s;
    }
}
```

---

### Caddy

Caddy enables HTTPS and passes standard proxy headers (`X-Forwarded-For`, `X-Forwarded-Proto`) automatically:

```caddy
pdswarm.example.com {
    reverse_proxy 127.0.0.1:9000
}
```

---

### Traefik

When using Traefik in Docker Compose, configure labels on the PDSwarm service container:

```yaml
services:
  pdswarm:
    image: ghcr.io/podiax/pdswarm:latest
    container_name: pdswarm
    restart: unless-stopped
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
      - pdswarm_data:/data
      - /var/log/pdswarm:/var/log/pdswarm
    environment:
      - PDSWARM_AUTH_LOG=/var/log/pdswarm/auth.log
      - PDSWARM_TRUSTED_PROXIES=172.16.0.0/12
    labels:
      - "traefik.enable=true"
      - "traefik.http.routers.pdswarm.rule=Host(`pdswarm.example.com`)"
      - "traefik.http.routers.pdswarm.entrypoints=websecure"
      - "traefik.http.routers.pdswarm.tls.certresolver=myresolver"
      - "traefik.http.services.pdswarm.loadbalancer.server.port=9000"

volumes:
  pdswarm_data:
```

---

## 3. Rate Limiting & Fail2ban Jails

PDSwarm includes two defense layers:

1. **In-Memory Rate Limiting**:
   * Global limit: 100 requests/sec with a burst of 200 per IP.
   * Auth endpoints (`/api/auth/*`): 5 requests/sec with a burst of 10 per IP to mitigate brute-force password guessing.
   * Exceeded thresholds immediately return `HTTP 429 Too Many Requests`.

2. **OS / Firewall Level Bans (Fail2ban & CrowdSec)**:
   * Failed authentication attempts are appended in real time to the file defined by `PDSWARM_AUTH_LOG`.
   * Go to **Settings &rarr; Audit & Jails** in the PDSwarm UI to copy the auto-generated `/etc/fail2ban/filter.d/pdswarm.conf` and `/etc/fail2ban/jail.d/pdswarm.conf` configurations.
