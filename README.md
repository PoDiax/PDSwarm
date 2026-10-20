<p align="center">
  <img src="assets/logo.svg" alt="PDSwarm Logo" width="380" />
</p>

<p align="center">
  <strong>Fast, lightweight Docker & Compose management. Single binary, zero bloat.</strong>
</p>

<p align="center">
  <a href="https://github.com/PoDiax/PDSwarm/releases"><img src="https://img.shields.io/github/v/release/PoDiax/PDSwarm?style=flat-square&color=7c3aed" alt="GitHub Release"></a>
  <img src="https://img.shields.io/badge/go-1.24+-00ADD8?style=flat-square&logo=go&logoColor=white" alt="Go 1.24+">
  <img src="https://img.shields.io/badge/frontend-Svelte%205-FF3E00?style=flat-square&logo=svelte&logoColor=white" alt="Svelte 5">
  <img src="https://img.shields.io/badge/binary-13MB%20standalone-10b981?style=flat-square" alt="Binary Size">
  <img src="https://img.shields.io/badge/docker-amd64%20%7C%20arm64-06b6d4?style=flat-square&logo=docker&logoColor=white" alt="Docker">
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-AGPL--3.0-blue?style=flat-square" alt="License: AGPL-3.0"></a>
</p>

---

Most Docker management dashboards fall into two extremes: either they're simple log viewers that can't manage containers, or they're heavy platforms that eat hundreds of megabytes of RAM just sitting idle with enterprise upsells on every screen.

**PDSwarm** is the middle ground: a fast, single-binary web UI that gives you full container management, Docker Compose stacks, live xterm terminals, container file browsing, and 1-click app templates — while staying under 15MB on disk and using practically zero CPU when idle.

No database server needed (it uses embedded bbolt), no Node.js runtime on your host, and no external agent requirements. Just download and run, or spin it up with Docker.

---

## Features

- **Single Static Binary**: Go backend + Svelte 5 frontend compiled into a single ~13MB executable with zero runtime dependencies.
- **Real-Time Terminals & Logs**: Interactive xterm.js terminal attached via `docker exec`, plus live streaming container logs over WebSockets.
- **Docker Compose Stacks**: Deploy, start, stop, edit, and inspect multi-container stacks. Includes automatic variable interpolation (`${VAR:-default}`) and relative path handling.
- **In-Container File Browser**: Browse directories, upload files, download logs, and edit configuration files inside running containers without `docker cp` or SSH.
- **1-Click App Store**: Over 35+ curated, production-ready application templates (PostgreSQL, Redis, Traefik, Jellyfin, Nextcloud, Uptime Kuma, etc.) with custom environment variable forms.
- **Duplicate & Edit Containers**: Clone containers or change environment variables, port mappings, volume mounts, and resource limits with a safe replace toggle.
- **Browser Dockerfile Studio**: Write Dockerfiles in your browser with starter templates and build images with live streaming build logs.
- **Multi-Host Management**: Switch between local socket and remote Docker hosts over TCP or TLS directly from the navbar.
- **Private Registries**: Save credentials for Docker Hub, GitHub Container Registry (GHCR), Quay, or self-hosted Harbor instances with auto-authentication on pull.
- **Security Audit Logs & Fail2ban Jails**: Dedicated authentication audit logging (`-auth-log`, `-log-level`) producing standardized CEF/Syslog auth events for fail2ban, CrowdSec, and FreeBSD/Linux jails.
- **API Access Tokens**: Create scoped `pds_...` access tokens for automation, scripts, and CI/CD pipelines alongside standard multi-user login.

<p align="center">
  <a href="SCREENSHOTS.md">
    <img src="assets/screenshots/dashboard.png" alt="PDSwarm Dashboard Preview" width="580" />
  </a>
  <br />
  <sub>Clean, dark Neo-Obsidian interface. <a href="SCREENSHOTS.md"><strong>Browse full UI gallery (12 screenshots) &rarr;</strong></a></sub>
</p>

---

## Quickstart

### Option 1: 1-Line Script Install (Systemd Service + Auto-Updates)

The easiest way to run PDSwarm on a Linux server. It installs the static binary, sets up a systemd service, and configures a daily auto-update timer:

```bash
curl -fsSL https://raw.githubusercontent.com/podiax/pdswarm/master/scripts/install.sh | sudo bash
```

Once installed, open **`http://<your-server-ip>:9000`** in your browser.

#### Managing the service:
```bash
# Check service status
sudo systemctl status pdswarm

# View live logs
sudo journalctl -u pdswarm -f

# View security audit logs (for fail2ban / jails)
tail -f /var/log/pdswarm/auth.log

# Check for updates manually right now
sudo pdswarm-update

# View the daily auto-update timer
sudo systemctl list-timers pdswarm-update.timer

# Uninstall
sudo /usr/local/bin/pdswarm --uninstall
```

#### Custom install options:
```bash
# Custom port, data directory, and security audit log path
curl -fsSL https://raw.githubusercontent.com/podiax/pdswarm/master/scripts/install.sh | sudo bash -s -- --port 8080 --data-dir /opt/pdswarm --auth-log /var/log/pdswarm/auth.log

# Set audit log level (auth, failures_only, all, off)
curl -fsSL https://raw.githubusercontent.com/podiax/pdswarm/master/scripts/install.sh | sudo bash -s -- --log-level failures_only

# Custom trusted reverse proxy CIDRs/IPs (for Cloudflare edge, upstream load balancers, etc.)
curl -fsSL https://raw.githubusercontent.com/podiax/pdswarm/master/scripts/install.sh | sudo bash -s -- --trusted-proxies "127.0.0.1/32,172.16.0.0/12,192.168.1.0/24"

# Allow cross-origin requests or external dashboard domains (CORS)
curl -fsSL https://raw.githubusercontent.com/podiax/pdswarm/master/scripts/install.sh | sudo bash -s -- --allowed-origins "https://dashboard.example.com"

# Install without the daily auto-updater timer
curl -fsSL https://raw.githubusercontent.com/podiax/pdswarm/master/scripts/install.sh | sudo bash -s -- --no-auto-update
```

---

### Option 2: Docker Run

If you prefer keeping everything inside Docker:

```bash
docker run -d \
  --name pdswarm \
  --restart unless-stopped \
  -p 9000:9000 \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v pdswarm_data:/data \
  -v /var/log/pdswarm:/var/log/pdswarm \
  -e PDSWARM_AUTH_LOG=/var/log/pdswarm/auth.log \
  -e PDSWARM_LOG_LEVEL=auth \
  -e PDSWARM_TRUSTED_PROXIES=127.0.0.0/8,10.0.0.0/8,172.16.0.0/12,192.168.0.0/16,::1/128 \
  # -e PDSWARM_ALLOWED_ORIGINS=https://dashboard.example.com \
  ghcr.io/podiax/pdswarm:latest
```

---

### Option 3: Docker Compose

A `docker-compose.yml` is included in the repo:

```yaml
services:
  pdswarm:
    image: ghcr.io/podiax/pdswarm:latest
    container_name: pdswarm
    restart: unless-stopped
    ports:
      - "9000:9000"
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
      - pdswarm_data:/data
      - /var/log/pdswarm:/var/log/pdswarm
    environment:
      - PDSWARM_HOST=0.0.0.0
      - PDSWARM_PORT=9000
      - PDSWARM_DATA_DIR=/data
      - PDSWARM_AUTH_LOG=/var/log/pdswarm/auth.log
      - PDSWARM_LOG_LEVEL=auth
      # - PDSWARM_TRUSTED_PROXIES=127.0.0.0/8,10.0.0.0/8,172.16.0.0/12,192.168.0.0/16,::1/128
      # - PDSWARM_ALLOWED_ORIGINS=https://dashboard.example.com

volumes:
  pdswarm_data:
```

Start it:
```bash
docker compose up -d
```

---

## Configuration

PDSwarm works out of the box with sensible defaults, but you can configure anything using CLI flags or environment variables:

| Flag | Environment Variable | Default | Description |
|---|---|---|---|
| `-port` | `PDSWARM_PORT` | `9000` | Port to bind the HTTP server |
| `-host` | `PDSWARM_HOST` | `0.0.0.0` | Host IP to bind the HTTP server |
| `-socket` | `DOCKER_HOST` | `/var/run/docker.sock` | Docker daemon UNIX socket or TCP/TLS URL |
| `-data-dir` | `PDSWARM_DATA_DIR` | `./data` | Directory where the bbolt database is stored |
| `-auth-log` | `PDSWARM_AUTH_LOG` | `/var/log/pdswarm/auth.log` | Security audit log file path for fail2ban / jails |
| `-log-level` | `PDSWARM_LOG_LEVEL` | `auth` | Audit log level: `auth`, `failures_only`, `all`, or `off` |
| `-trusted-proxies` | `PDSWARM_TRUSTED_PROXIES` | `127.0.0.0/8,10.0.0.0/8,172.16.0.0/12,192.168.0.0/16,::1/128` | Comma-separated trusted reverse proxy CIDRs/IPs |
| `-allowed-origins` | `PDSWARM_ALLOWED_ORIGINS` | *(empty)* | Comma-separated allowed CORS & WebSocket origins (empty = same-origin only) |
| `-no-auth` | `PDSWARM_NO_AUTH` | `false` | Disable authentication (useful for trusted LANs/tailnets) |
| `-jwt-secret` | `PDSWARM_JWT_SECRET` | *(auto-generated)* | JWT signing secret (persisted across restarts) |
| `-version`, `-v` | — | — | Print PDSwarm version and exit |

> For production reverse proxy setups (Cloudflare Tunnels, Nginx, Caddy, Traefik) and IP spoofing prevention, see the [**Reverse Proxy & Security Guide**](docs/reverse-proxies.md).

Example running with flags:
```bash
./pdswarm -port 8080 -data-dir /var/lib/pdswarm -auth-log /var/log/pdswarm/auth.log -socket /var/run/docker.sock
```

---

## Security Architecture

PDSwarm provides robust, multi-layer security protections out of the box:

### 1. Cross-Origin Resource Sharing (CORS) & Browser Protection
* **Default Same-Origin Policy**: By default, `AllowedOrigins` is empty. The backend strictly forbids cross-origin browser requests, preventing external websites from reading API responses or issuing authenticated requests using your browser session.
* **External Dashboards / Portals**: If you run a custom frontend or separate dashboard that consumes PDSwarm, specify your allowed domains via `-allowed-origins` or `PDSWARM_ALLOWED_ORIGINS="https://dashboard.example.com"`.

### 2. Cross-Site Request Forgery (CSRF) & Cookie Hardening
* **SameSite Cookie Isolation**: Session cookies (`pdswarm_token`) use `SameSite=Lax` and `HttpOnly`, blocking third-party websites from attaching cookies on cross-origin requests.
* **Automatic HTTPS / Secure Cookie Flag**: When PDSwarm is accessed over HTTPS (or behind an SSL-terminating reverse proxy setting `X-Forwarded-Proto: https`), cookies automatically enforce the `Secure` flag, preventing transmission over unencrypted connections.

### 3. Cross-Site WebSocket Hijacking (CSWSH) Defense
* Live container terminals (`/ws/containers/{id}/exec`), real-time log viewers (`/logs`), and metrics streams (`/stats`) strictly validate that incoming `Origin` headers match the target `Host` (or an explicitly allowed origin), blocking rogue websites from hijacking interactive shells.

### 4. Anti-IP Spoofing & Fail2ban / CrowdSec Accuracy
* When sitting behind reverse proxies (Cloudflare, Nginx, Traefik, Caddy), client IPs are resolved from forwarding headers **only** when the immediate TCP connection originates from a trusted proxy CIDR (`-trusted-proxies`).
* If an attacker attempts to spoof `X-Forwarded-For` or `CF-Connecting-IP` from an untrusted network, the headers are ignored and the real IP is recorded in `/var/log/pdswarm/auth.log`, ensuring jail bans (Fail2ban/CrowdSec) target the true attacker.

### 5. Role-Based Access Control (RBAC)
* Endpoints strictly enforce permissions across **Admin**, **Operator**, and **Viewer** roles:
  * **Admin**: User management, environment configuration, system updates, and security logs.
  * **Operator**: Container, stack, image, volume, and network mutations, plus terminal exec.
  * **Viewer**: Read-only monitoring of containers, metrics, and logs.
* API tokens strictly inherit the creator's role, preventing privilege escalation.

---

## How Auto-Updates Work

When installed via `install.sh`, PDSwarm sets up a systemd timer that checks GitHub Releases daily. 

If a newer release is published:
1. It downloads the new binary for your architecture to a temporary file.
2. It tests the binary to verify it executes cleanly.
3. It performs an atomic `mv` to replace `/usr/local/bin/pdswarm` without leaving the file corrupted.
4. It restarts `pdswarm.service`.

If you prefer to update manually or via your own scripts:
```bash
sudo pdswarm-update
```
Or force an update check regardless of current version:
```bash
sudo pdswarm-update --force
```

---

## Building from Source

Prerequisites:
- Go 1.24 or newer
- Node.js 20 or newer

```bash
# Clone the repository
git clone https://github.com/podiax/pdswarm.git
cd pdswarm

# Build frontend and compile the static binary
make build

# Run it
./pdswarm -port 9000
```

The resulting binary (`./pdswarm`) is completely self-contained. The Svelte 5 frontend dist is embedded into the binary using Go's `embed` package, so you only need to copy that single file to deploy.

To build the Docker image locally:
```bash
make docker-build
```

---

## Architecture Overview

```
┌────────────────────────────────────────────────────────┐
│                        PDSwarm                         │
│                                                        │
│  ┌─────────────────────────┐  ┌─────────────────────┐  │
│  │   Svelte 5 Frontend     │  │   Go HTTP Router    │  │
│  │  (Single Page App)      │  │     (chi router)    │  │
│  │  - Embedded in binary   │  │  - REST & WebSockets│  │
│  │  - xterm.js terminals   │  │  - Auth & API tokens│  │
│  │  - Tailwind CSS         │  │  - Reverse proxy    │  │
│  └────────────┬────────────┘  └──────────┬──────────┘  │
│               │                          │             │
│               └─────────────┬────────────┘             │
│                             │                          │
│                ┌────────────┴────────────┐             │
│                │      Docker Engine      │             │
│                │   (/var/run/docker.sock)│             │
│                │   or remote TCP / TLS   │             │
│                └─────────────────────────┘             │
│                             │                          │
│                ┌────────────┴────────────┐             │
│                │   Embedded bbolt DB     │             │
│                │ (users, stacks, tokens) │             │
│                └─────────────────────────┘             │
└────────────────────────────────────────────────────────┘
```

- **Backend**: Go with `moby/moby` Docker client, standard library HTTP + `chi` router, and `bbolt` for zero-overhead embedded key-value storage.
- **Frontend**: Svelte 5 with modern runes (`$state`, `$derived`, `$props`), Lucide icons, and Tailwind CSS.
- **Binary embedding**: `embed.FS` bundles all HTML, JS, CSS, and SVG assets into the executable at compile time.

---

## License

This project is licensed under the **GNU Affero General Public License v3.0** (AGPL-3.0). See [LICENSE](LICENSE) for details.
