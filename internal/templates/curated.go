package templates

// GetCuratedTemplates returns the built-in library of 35+ production-ready application templates.
func GetCuratedTemplates() []Template {
	return []Template{
		// ---------------------------------------------------------------------
		// DATABASES
		// ---------------------------------------------------------------------
		{
			ID:          "postgres",
			Title:       "PostgreSQL 16",
			Description: "Powerful, open source object-relational database system.",
			Categories:  []string{"Database"},
			Logo:        "https://cdn.jsdelivr.net/gh/walkxcode/dashboard-icons/png/postgres.png",
			Platform:    "linux",
			Type:        "compose",
			ComposeYAML: `version: '3.8'
services:
  db:
    image: postgres:16-alpine
    restart: unless-stopped
    environment:
      POSTGRES_DB: ${POSTGRES_DB:-pdswarm}
      POSTGRES_USER: ${POSTGRES_USER:-postgres}
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD:-postgres_password}
    ports:
      - "${POSTGRES_PORT:-5432}:5432"
    volumes:
      - postgres_data:/var/lib/postgresql/data

volumes:
  postgres_data:
`,
			Env: []TemplateEnvVar{
				{Name: "POSTGRES_DB", Label: "Database Name", Default: "pdswarm", Description: "Initial database name", Required: false},
				{Name: "POSTGRES_USER", Label: "Username", Default: "postgres", Description: "Superuser username", Required: false},
				{Name: "POSTGRES_PASSWORD", Label: "Password", Default: "postgres_password", Description: "Superuser password", Required: true},
				{Name: "POSTGRES_PORT", Label: "Host Port", Default: "5432", Description: "Port exposed on host", Required: false},
			},
			Volumes: []string{"postgres_data:/var/lib/postgresql/data"},
			Ports:   []string{"5432:5432"},
			Source:  "curated",
		},
		{
			ID:          "mysql",
			Title:       "MySQL 8",
			Description: "World's most popular open source relational database.",
			Categories:  []string{"Database"},
			Logo:        "https://cdn.jsdelivr.net/gh/walkxcode/dashboard-icons/png/mysql.png",
			Platform:    "linux",
			Type:        "compose",
			ComposeYAML: `version: '3.8'
services:
  db:
    image: mysql:8.0
    restart: unless-stopped
    environment:
      MYSQL_ROOT_PASSWORD: ${MYSQL_ROOT_PASSWORD:-root_password}
      MYSQL_DATABASE: ${MYSQL_DATABASE:-app_db}
      MYSQL_USER: ${MYSQL_USER:-db_user}
      MYSQL_PASSWORD: ${MYSQL_PASSWORD:-db_password}
    ports:
      - "${MYSQL_PORT:-3306}:3306"
    volumes:
      - mysql_data:/var/lib/mysql

volumes:
  mysql_data:
`,
			Env: []TemplateEnvVar{
				{Name: "MYSQL_ROOT_PASSWORD", Label: "Root Password", Default: "root_password", Description: "MySQL root password", Required: true},
				{Name: "MYSQL_DATABASE", Label: "Database Name", Default: "app_db", Description: "Database name to create", Required: false},
				{Name: "MYSQL_USER", Label: "User", Default: "db_user", Description: "Non-root user", Required: false},
				{Name: "MYSQL_PASSWORD", Label: "User Password", Default: "db_password", Description: "Non-root user password", Required: false},
				{Name: "MYSQL_PORT", Label: "Host Port", Default: "3306", Description: "Host port for MySQL", Required: false},
			},
			Volumes: []string{"mysql_data:/var/lib/mysql"},
			Ports:   []string{"3306:3306"},
			Source:  "curated",
		},
		{
			ID:          "mariadb",
			Title:       "MariaDB 11",
			Description: "Fast, scalable and robust open-source relational database created by MySQL original developers.",
			Categories:  []string{"Database"},
			Logo:        "https://cdn.jsdelivr.net/gh/walkxcode/dashboard-icons/png/mariadb.png",
			Platform:    "linux",
			Type:        "compose",
			ComposeYAML: `version: '3.8'
services:
  db:
    image: mariadb:11
    restart: unless-stopped
    environment:
      MARIADB_ROOT_PASSWORD: ${MARIADB_ROOT_PASSWORD:-root_password}
      MARIADB_DATABASE: ${MARIADB_DATABASE:-mariadb_db}
      MARIADB_USER: ${MARIADB_USER:-mariadb_user}
      MARIADB_PASSWORD: ${MARIADB_PASSWORD:-mariadb_password}
    ports:
      - "${MARIADB_PORT:-3306}:3306"
    volumes:
      - mariadb_data:/var/lib/mysql

volumes:
  mariadb_data:
`,
			Env: []TemplateEnvVar{
				{Name: "MARIADB_ROOT_PASSWORD", Label: "Root Password", Default: "root_password", Description: "Root password", Required: true},
				{Name: "MARIADB_DATABASE", Label: "Database Name", Default: "mariadb_db", Description: "Initial database", Required: false},
				{Name: "MARIADB_USER", Label: "Database User", Default: "mariadb_user", Description: "Username", Required: false},
				{Name: "MARIADB_PASSWORD", Label: "User Password", Default: "mariadb_password", Description: "Password", Required: false},
				{Name: "MARIADB_PORT", Label: "Host Port", Default: "3306", Description: "Port exposed on host", Required: false},
			},
			Volumes: []string{"mariadb_data:/var/lib/mysql"},
			Ports:   []string{"3306:3306"},
			Source:  "curated",
		},
		{
			ID:          "redis",
			Title:       "Redis 7",
			Description: "In-memory data structure store used as a database, cache, and message broker.",
			Categories:  []string{"Database"},
			Logo:        "https://cdn.jsdelivr.net/gh/walkxcode/dashboard-icons/png/redis.png",
			Platform:    "linux",
			Type:        "compose",
			ComposeYAML: `version: '3.8'
services:
  cache:
    image: redis:7-alpine
    restart: unless-stopped
    command: redis-server --appendonly yes --requirepass "${REDIS_PASSWORD:-redis_password}"
    ports:
      - "${REDIS_PORT:-6379}:6379"
    volumes:
      - redis_data:/data

volumes:
  redis_data:
`,
			Env: []TemplateEnvVar{
				{Name: "REDIS_PASSWORD", Label: "Redis Password", Default: "redis_password", Description: "Password for Redis auth", Required: true},
				{Name: "REDIS_PORT", Label: "Host Port", Default: "6379", Description: "Port exposed on host", Required: false},
			},
			Volumes: []string{"redis_data:/data"},
			Ports:   []string{"6379:6379"},
			Source:  "curated",
		},
		{
			ID:          "mongodb",
			Title:       "MongoDB 7",
			Description: "Document-oriented NoSQL database for modern apps and flexible schemas.",
			Categories:  []string{"Database"},
			Logo:        "https://cdn.jsdelivr.net/gh/walkxcode/dashboard-icons/png/mongodb.png",
			Platform:    "linux",
			Type:        "compose",
			ComposeYAML: `version: '3.8'
services:
  db:
    image: mongo:7.0
    restart: unless-stopped
    environment:
      MONGO_INITDB_ROOT_USERNAME: ${MONGO_INITDB_ROOT_USERNAME:-admin}
      MONGO_INITDB_ROOT_PASSWORD: ${MONGO_INITDB_ROOT_PASSWORD:-mongo_password}
    ports:
      - "${MONGO_PORT:-27017}:27017"
    volumes:
      - mongo_data:/data/db

volumes:
  mongo_data:
`,
			Env: []TemplateEnvVar{
				{Name: "MONGO_INITDB_ROOT_USERNAME", Label: "Admin Username", Default: "admin", Description: "Initial root username", Required: false},
				{Name: "MONGO_INITDB_ROOT_PASSWORD", Label: "Admin Password", Default: "mongo_password", Description: "Initial root password", Required: true},
				{Name: "MONGO_PORT", Label: "Host Port", Default: "27017", Description: "Port exposed on host", Required: false},
			},
			Volumes: []string{"mongo_data:/data/db"},
			Ports:   []string{"27017:27017"},
			Source:  "curated",
		},
		{
			ID:          "clickhouse",
			Title:       "ClickHouse",
			Description: "High-performance, column-oriented database management system for real-time analytics.",
			Categories:  []string{"Database"},
			Logo:        "https://cdn.jsdelivr.net/gh/walkxcode/dashboard-icons/png/clickhouse.png",
			Platform:    "linux",
			Type:        "compose",
			ComposeYAML: `version: '3.8'
services:
  analytics:
    image: clickhouse/clickhouse-server:latest
    restart: unless-stopped
    environment:
      CLICKHOUSE_DB: ${CLICKHOUSE_DB:-analytics}
      CLICKHOUSE_USER: ${CLICKHOUSE_USER:-default}
      CLICKHOUSE_PASSWORD: ${CLICKHOUSE_PASSWORD:-clickhouse_password}
    ports:
      - "${CLICKHOUSE_HTTP_PORT:-8123}:8123"
      - "${CLICKHOUSE_NATIVE_PORT:-9000}:9000"
    volumes:
      - clickhouse_data:/var/lib/clickhouse

volumes:
  clickhouse_data:
`,
			Env: []TemplateEnvVar{
				{Name: "CLICKHOUSE_DB", Label: "Database Name", Default: "analytics", Description: "Default database", Required: false},
				{Name: "CLICKHOUSE_USER", Label: "User", Default: "default", Description: "Username", Required: false},
				{Name: "CLICKHOUSE_PASSWORD", Label: "Password", Default: "clickhouse_password", Description: "Password for user", Required: true},
				{Name: "CLICKHOUSE_HTTP_PORT", Label: "HTTP Port", Default: "8123", Description: "HTTP query port", Required: false},
				{Name: "CLICKHOUSE_NATIVE_PORT", Label: "Native Client Port", Default: "9000", Description: "TCP port", Required: false},
			},
			Volumes: []string{"clickhouse_data:/var/lib/clickhouse"},
			Ports:   []string{"8123:8123", "9000:9000"},
			Source:  "curated",
		},
		{
			ID:          "pgadmin",
			Title:       "pgAdmin 4",
			Description: "Comprehensive administration and management tool for PostgreSQL databases.",
			Categories:  []string{"Database"},
			Logo:        "https://cdn.jsdelivr.net/gh/walkxcode/dashboard-icons/png/pgadmin.png",
			Platform:    "linux",
			Type:        "compose",
			ComposeYAML: `version: '3.8'
services:
  pgadmin:
    image: dpage/pgadmin4:latest
    restart: unless-stopped
    environment:
      PGADMIN_DEFAULT_EMAIL: ${PGADMIN_DEFAULT_EMAIL:-admin@example.com}
      PGADMIN_DEFAULT_PASSWORD: ${PGADMIN_DEFAULT_PASSWORD:-admin_password}
    ports:
      - "${PGADMIN_PORT:-5050}:80"
    volumes:
      - pgadmin_data:/var/lib/pgadmin

volumes:
  pgadmin_data:
`,
			Env: []TemplateEnvVar{
				{Name: "PGADMIN_DEFAULT_EMAIL", Label: "Login Email", Default: "admin@example.com", Description: "Email for admin login", Required: true},
				{Name: "PGADMIN_DEFAULT_PASSWORD", Label: "Password", Default: "admin_password", Description: "Password for admin login", Required: true},
				{Name: "PGADMIN_PORT", Label: "Host Port", Default: "5050", Description: "Web UI port", Required: false},
			},
			Volumes: []string{"pgadmin_data:/var/lib/pgadmin"},
			Ports:   []string{"5050:80"},
			Source:  "curated",
		},
		{
			ID:          "redis-commander",
			Title:       "Redis Commander",
			Description: "Web-based management UI for Redis written in node.js.",
			Categories:  []string{"Database"},
			Logo:        "https://cdn.jsdelivr.net/gh/walkxcode/dashboard-icons/png/redis.png",
			Platform:    "linux",
			Type:        "compose",
			ComposeYAML: `version: '3.8'
services:
  ui:
    image: rediscommander/redis-commander:latest
    restart: unless-stopped
    environment:
      REDIS_HOSTS: ${REDIS_HOSTS:-local:redis:6379}
    ports:
      - "${REDIS_COMMANDER_PORT:-8081}:8081"
`,
			Env: []TemplateEnvVar{
				{Name: "REDIS_HOSTS", Label: "Redis Connection", Default: "local:redis:6379", Description: "Redis host definition", Required: false},
				{Name: "REDIS_COMMANDER_PORT", Label: "Host Port", Default: "8081", Description: "Port exposed on host", Required: false},
			},
			Ports:  []string{"8081:8081"},
			Source: "curated",
		},

		// ---------------------------------------------------------------------
		// WEB & PROXIES
		// ---------------------------------------------------------------------
		{
			ID:          "nginx",
			Title:       "Nginx",
			Description: "High-performance HTTP server, reverse proxy, and IMAP/POP3 proxy server.",
			Categories:  []string{"Web & Proxies"},
			Logo:        "https://cdn.jsdelivr.net/gh/walkxcode/dashboard-icons/png/nginx.png",
			Platform:    "linux",
			Type:        "compose",
			ComposeYAML: `version: '3.8'
services:
  web:
    image: nginx:alpine
    restart: unless-stopped
    ports:
      - "${HTTP_PORT:-80}:80"
      - "${HTTPS_PORT:-443}:443"
    volumes:
      - nginx_html:/usr/share/nginx/html

volumes:
  nginx_html:
`,
			Env: []TemplateEnvVar{
				{Name: "HTTP_PORT", Label: "HTTP Port", Default: "80", Description: "HTTP host port", Required: false},
				{Name: "HTTPS_PORT", Label: "HTTPS Port", Default: "443", Description: "HTTPS host port", Required: false},
			},
			Volumes: []string{"nginx_html:/usr/share/nginx/html"},
			Ports:   []string{"80:80", "443:443"},
			Source:  "curated",
		},
		{
			ID:          "traefik",
			Title:       "Traefik v3",
			Description: "Modern HTTP reverse proxy and load balancer that integrates natively with Docker.",
			Categories:  []string{"Web & Proxies"},
			Logo:        "https://cdn.jsdelivr.net/gh/walkxcode/dashboard-icons/png/traefik.png",
			Platform:    "linux",
			Type:        "compose",
			ComposeYAML: `version: '3.8'
services:
  traefik:
    image: traefik:v3.1
    restart: unless-stopped
    command:
      - "--api.insecure=true"
      - "--providers.docker=true"
      - "--providers.docker.exposedbydefault=false"
      - "--entrypoints.web.address=:80"
      - "--entrypoints.websecure.address=:443"
    ports:
      - "${TRAEFIK_HTTP_PORT:-80}:80"
      - "${TRAEFIK_HTTPS_PORT:-443}:443"
      - "${TRAEFIK_DASHBOARD_PORT:-8080}:8080"
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
      - traefik_certs:/etc/traefik/acme

volumes:
  traefik_certs:
`,
			Env: []TemplateEnvVar{
				{Name: "TRAEFIK_HTTP_PORT", Label: "HTTP Port", Default: "80", Description: "Traefik entrypoint port", Required: false},
				{Name: "TRAEFIK_HTTPS_PORT", Label: "HTTPS Port", Default: "443", Description: "Secure port", Required: false},
				{Name: "TRAEFIK_DASHBOARD_PORT", Label: "Dashboard Port", Default: "8080", Description: "Web UI dashboard port", Required: false},
			},
			Volumes: []string{"/var/run/docker.sock:/var/run/docker.sock:ro", "traefik_certs:/etc/traefik/acme"},
			Ports:   []string{"80:80", "443:443", "8080:8080"},
			Source:  "curated",
		},
		{
			ID:          "caddy",
			Title:       "Caddy 2",
			Description: "Powerful enterprise-ready open-source web server with automatic TLS certificate renewals.",
			Categories:  []string{"Web & Proxies"},
			Logo:        "https://cdn.jsdelivr.net/gh/walkxcode/dashboard-icons/png/caddy.png",
			Platform:    "linux",
			Type:        "compose",
			ComposeYAML: `version: '3.8'
services:
  caddy:
    image: caddy:2-alpine
    restart: unless-stopped
    ports:
      - "${CADDY_HTTP_PORT:-80}:80"
      - "${CADDY_HTTPS_PORT:-443}:443"
    volumes:
      - caddy_data:/data
      - caddy_config:/config

volumes:
  caddy_data:
  caddy_config:
`,
			Env: []TemplateEnvVar{
				{Name: "CADDY_HTTP_PORT", Label: "HTTP Port", Default: "80", Description: "Host port for HTTP", Required: false},
				{Name: "CADDY_HTTPS_PORT", Label: "HTTPS Port", Default: "443", Description: "Host port for HTTPS", Required: false},
			},
			Volumes: []string{"caddy_data:/data", "caddy_config:/config"},
			Ports:   []string{"80:80", "443:443"},
			Source:  "curated",
		},
		{
			ID:          "cloudflared",
			Title:       "Cloudflare Tunnel",
			Description: "Connect your local container services securely to Cloudflare network without opening public ports.",
			Categories:  []string{"Web & Proxies"},
			Logo:        "https://cdn.jsdelivr.net/gh/walkxcode/dashboard-icons/png/cloudflare.png",
			Platform:    "linux",
			Type:        "compose",
			ComposeYAML: `version: '3.8'
services:
  tunnel:
    image: cloudflare/cloudflared:latest
    restart: unless-stopped
    command: tunnel --no-autoupdate run --token ${TUNNEL_TOKEN}
`,
			Env: []TemplateEnvVar{
				{Name: "TUNNEL_TOKEN", Label: "Tunnel Token", Default: "", Description: "Cloudflare Zero Trust connector token", Required: true},
			},
			Source: "curated",
		},

		// ---------------------------------------------------------------------
		// MEDIA & STREAMING
		// ---------------------------------------------------------------------
		{
			ID:          "jellyfin",
			Title:       "Jellyfin",
			Description: "The Free Software Media System that puts you in control of managing and streaming your media.",
			Categories:  []string{"Media"},
			Logo:        "https://cdn.jsdelivr.net/gh/walkxcode/dashboard-icons/png/jellyfin.png",
			Platform:    "linux",
			Type:        "compose",
			ComposeYAML: `version: '3.8'
services:
  jellyfin:
    image: jellyfin/jellyfin:latest
    restart: unless-stopped
    ports:
      - "${JELLYFIN_PORT:-8096}:8096"
    volumes:
      - jellyfin_config:/config
      - jellyfin_cache:/cache
      - jellyfin_media:/media

volumes:
  jellyfin_config:
  jellyfin_cache:
  jellyfin_media:
`,
			Env: []TemplateEnvVar{
				{Name: "JELLYFIN_PORT", Label: "Host Port", Default: "8096", Description: "Port exposed on host", Required: false},
			},
			Volumes: []string{"jellyfin_config:/config", "jellyfin_cache:/cache", "jellyfin_media:/media"},
			Ports:   []string{"8096:8096"},
			Source:  "curated",
		},
		{
			ID:          "plex",
			Title:       "Plex Media Server",
			Description: "Organizes video, audio, and photos from a user's collections and streams them to players.",
			Categories:  []string{"Media"},
			Logo:        "https://cdn.jsdelivr.net/gh/walkxcode/dashboard-icons/png/plex.png",
			Platform:    "linux",
			Type:        "compose",
			ComposeYAML: `version: '3.8'
services:
  plex:
    image: lscr.io/linuxserver/plex:latest
    restart: unless-stopped
    environment:
      PUID: 1000
      PGID: 1000
      VERSION: docker
      PLEX_CLAIM: ${PLEX_CLAIM:-}
    ports:
      - "${PLEX_PORT:-32400}:32400"
    volumes:
      - plex_config:/config
      - plex_media:/media

volumes:
  plex_config:
  plex_media:
`,
			Env: []TemplateEnvVar{
				{Name: "PLEX_CLAIM", Label: "Plex Claim Token", Default: "", Description: "Obtain claim token from plex.tv/claim", Required: false},
				{Name: "PLEX_PORT", Label: "Host Port", Default: "32400", Description: "Plex web server port", Required: false},
			},
			Volumes: []string{"plex_config:/config", "plex_media:/media"},
			Ports:   []string{"32400:32400"},
			Source:  "curated",
		},
		{
			ID:          "navidrome",
			Title:       "Navidrome",
			Description: "Modern music server and streamer compatible with Subsonic/Madsonic clients.",
			Categories:  []string{"Media"},
			Logo:        "https://cdn.jsdelivr.net/gh/walkxcode/dashboard-icons/png/navidrome.png",
			Platform:    "linux",
			Type:        "compose",
			ComposeYAML: `version: '3.8'
services:
  navidrome:
    image: deluan/navidrome:latest
    restart: unless-stopped
    ports:
      - "${NAVIDROME_PORT:-4533}:4533"
    environment:
      ND_SCANSCHEDULE: 1h
      ND_LOGLEVEL: info
    volumes:
      - navidrome_data:/data
      - navidrome_music:/music

volumes:
  navidrome_data:
  navidrome_music:
`,
			Env: []TemplateEnvVar{
				{Name: "NAVIDROME_PORT", Label: "Host Port", Default: "4533", Description: "Navidrome web UI port", Required: false},
			},
			Volumes: []string{"navidrome_data:/data", "navidrome_music:/music"},
			Ports:   []string{"4533:4533"},
			Source:  "curated",
		},
		{
			ID:          "immich",
			Title:       "Immich",
			Description: "Self-hosted photo and video backup solution directly from your mobile phone.",
			Categories:  []string{"Media"},
			Logo:        "https://cdn.jsdelivr.net/gh/walkxcode/dashboard-icons/png/immich.png",
			Platform:    "linux",
			Type:        "compose",
			ComposeYAML: `version: '3.8'
services:
  immich-server:
    image: ghcr.io/immich-app/immich-server:release
    restart: unless-stopped
    environment:
      DB_HOSTNAME: postgres
      DB_DATABASE_NAME: immich
      DB_USERNAME: postgres
      DB_PASSWORD: ${DB_PASSWORD:-immich_password}
      REDIS_HOSTNAME: redis
    ports:
      - "${IMMICH_PORT:-2283}:2283"
    volumes:
      - immich_upload:/usr/src/app/upload
    depends_on:
      - redis
      - postgres

  redis:
    image: redis:6.2-alpine
    restart: unless-stopped

  postgres:
    image: tensorchord/pgvecto-rs:pg14-v0.2.0
    restart: unless-stopped
    environment:
      POSTGRES_DB: immich
      POSTGRES_USER: postgres
      POSTGRES_PASSWORD: ${DB_PASSWORD:-immich_password}
    volumes:
      - immich_pgdata:/var/lib/postgresql/data

volumes:
  immich_upload:
  immich_pgdata:
`,
			Env: []TemplateEnvVar{
				{Name: "DB_PASSWORD", Label: "Database Password", Default: "immich_password", Description: "PostgreSQL password for Immich", Required: true},
				{Name: "IMMICH_PORT", Label: "Host Port", Default: "2283", Description: "Port exposed on host", Required: false},
			},
			Volumes: []string{"immich_upload:/usr/src/app/upload", "immich_pgdata:/var/lib/postgresql/data"},
			Ports:   []string{"2283:2283"},
			Source:  "curated",
		},

		// ---------------------------------------------------------------------
		// MONITORING & OPS
		// ---------------------------------------------------------------------
		{
			ID:          "prometheus",
			Title:       "Prometheus",
			Description: "Open-source systems monitoring and alerting toolkit with dimensional data model.",
			Categories:  []string{"Monitoring"},
			Logo:        "https://cdn.jsdelivr.net/gh/walkxcode/dashboard-icons/png/prometheus.png",
			Platform:    "linux",
			Type:        "compose",
			ComposeYAML: `version: '3.8'
services:
  prometheus:
    image: prom/prometheus:latest
    restart: unless-stopped
    ports:
      - "${PROMETHEUS_PORT:-9090}:9090"
    volumes:
      - prometheus_data:/prometheus

volumes:
  prometheus_data:
`,
			Env: []TemplateEnvVar{
				{Name: "PROMETHEUS_PORT", Label: "Host Port", Default: "9090", Description: "Port exposed on host", Required: false},
			},
			Volumes: []string{"prometheus_data:/prometheus"},
			Ports:   []string{"9090:9090"},
			Source:  "curated",
		},
		{
			ID:          "grafana",
			Title:       "Grafana",
			Description: "The open observability platform for visualizing metrics, logs, and traces.",
			Categories:  []string{"Monitoring"},
			Logo:        "https://cdn.jsdelivr.net/gh/walkxcode/dashboard-icons/png/grafana.png",
			Platform:    "linux",
			Type:        "compose",
			ComposeYAML: `version: '3.8'
services:
  grafana:
    image: grafana/grafana:latest
    restart: unless-stopped
    environment:
      GF_SECURITY_ADMIN_USER: ${GRAFANA_ADMIN_USER:-admin}
      GF_SECURITY_ADMIN_PASSWORD: ${GRAFANA_ADMIN_PASSWORD:-grafana_admin}
    ports:
      - "${GRAFANA_PORT:-3000}:3000"
    volumes:
      - grafana_data:/var/lib/grafana

volumes:
  grafana_data:
`,
			Env: []TemplateEnvVar{
				{Name: "GRAFANA_ADMIN_USER", Label: "Admin Username", Default: "admin", Description: "Initial admin username", Required: false},
				{Name: "GRAFANA_ADMIN_PASSWORD", Label: "Admin Password", Default: "grafana_admin", Description: "Initial admin password", Required: true},
				{Name: "GRAFANA_PORT", Label: "Host Port", Default: "3000", Description: "Web UI port", Required: false},
			},
			Volumes: []string{"grafana_data:/var/lib/grafana"},
			Ports:   []string{"3000:3000"},
			Source:  "curated",
		},
		{
			ID:          "dozzle",
			Title:       "Dozzle",
			Description: "Simple, lightweight real-time log viewer for Docker containers.",
			Categories:  []string{"Monitoring"},
			Logo:        "https://cdn.jsdelivr.net/gh/walkxcode/dashboard-icons/png/dozzle.png",
			Platform:    "linux",
			Type:        "compose",
			ComposeYAML: `version: '3.8'
services:
  dozzle:
    image: amir20/dozzle:latest
    restart: unless-stopped
    ports:
      - "${DOZZLE_PORT:-8888}:8080"
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
`,
			Env: []TemplateEnvVar{
				{Name: "DOZZLE_PORT", Label: "Host Port", Default: "8888", Description: "Web interface port", Required: false},
			},
			Volumes: []string{"/var/run/docker.sock:/var/run/docker.sock:ro"},
			Ports:   []string{"8888:8080"},
			Source:  "curated",
		},
		{
			ID:          "uptime-kuma",
			Title:       "Uptime Kuma",
			Description: "Self-hosted monitoring tool with status pages and alerts for HTTP, TCP, Ping, DNS.",
			Categories:  []string{"Monitoring"},
			Logo:        "https://cdn.jsdelivr.net/gh/walkxcode/dashboard-icons/png/uptime-kuma.png",
			Platform:    "linux",
			Type:        "compose",
			ComposeYAML: `version: '3.8'
services:
  uptime-kuma:
    image: louislam/uptime-kuma:latest
    restart: unless-stopped
    ports:
      - "${UPTIME_KUMA_PORT:-3001}:3001"
    volumes:
      - uptime_kuma_data:/app/data

volumes:
  uptime_kuma_data:
`,
			Env: []TemplateEnvVar{
				{Name: "UPTIME_KUMA_PORT", Label: "Host Port", Default: "3001", Description: "Web UI port", Required: false},
			},
			Volumes: []string{"uptime_kuma_data:/app/data"},
			Ports:   []string{"3001:3001"},
			Source:  "curated",
		},
		{
			ID:          "netdata",
			Title:       "Netdata",
			Description: "High-resolution real-time infrastructure metrics and distributed telemetry engine.",
			Categories:  []string{"Monitoring"},
			Logo:        "https://cdn.jsdelivr.net/gh/walkxcode/dashboard-icons/png/netdata.png",
			Platform:    "linux",
			Type:        "compose",
			ComposeYAML: `version: '3.8'
services:
  netdata:
    image: netdata/netdata:latest
    restart: unless-stopped
    hostname: pdswarm-node
    ports:
      - "${NETDATA_PORT:-19999}:19999"
    cap_add:
      - SYS_PTRACE
      - SYS_ADMIN
    security_opt:
      - apparmor:unconfined
    volumes:
      - netdataconfig:/etc/netdata
      - netdatalib:/var/lib/netdata
      - netdatacache:/var/cache/netdata
      - /etc/passwd:/host/etc/passwd:ro
      - /etc/group:/host/etc/group:ro
      - /proc:/host/proc:ro
      - /sys:/host/sys:ro
      - /etc/os-release:/host/etc/os-release:ro
      - /var/run/docker.sock:/var/run/docker.sock:ro

volumes:
  netdataconfig:
  netdatalib:
  netdatacache:
`,
			Env: []TemplateEnvVar{
				{Name: "NETDATA_PORT", Label: "Host Port", Default: "19999", Description: "Netdata web dashboard port", Required: false},
			},
			Volumes: []string{"netdataconfig:/etc/netdata", "netdatalib:/var/lib/netdata"},
			Ports:   []string{"19999:19999"},
			Source:  "curated",
		},

		// ---------------------------------------------------------------------
		// PRODUCTIVITY & CLOUD
		// ---------------------------------------------------------------------
		{
			ID:          "nextcloud",
			Title:       "Nextcloud Hub",
			Description: "Self-hosted productivity platform that puts you in control of your documents, files, and calendars.",
			Categories:  []string{"Productivity"},
			Logo:        "https://cdn.jsdelivr.net/gh/walkxcode/dashboard-icons/png/nextcloud.png",
			Platform:    "linux",
			Type:        "compose",
			ComposeYAML: `version: '3.8'
services:
  nextcloud:
    image: nextcloud:apache
    restart: unless-stopped
    ports:
      - "${NEXTCLOUD_PORT:-8080}:80"
    environment:
      NEXTCLOUD_ADMIN_USER: ${NEXTCLOUD_ADMIN_USER:-admin}
      NEXTCLOUD_ADMIN_PASSWORD: ${NEXTCLOUD_ADMIN_PASSWORD:-nextcloud_admin}
    volumes:
      - nextcloud_data:/var/www/html

volumes:
  nextcloud_data:
`,
			Env: []TemplateEnvVar{
				{Name: "NEXTCLOUD_ADMIN_USER", Label: "Admin Username", Default: "admin", Description: "Initial administrator user", Required: false},
				{Name: "NEXTCLOUD_ADMIN_PASSWORD", Label: "Admin Password", Default: "nextcloud_admin", Description: "Initial administrator password", Required: true},
				{Name: "NEXTCLOUD_PORT", Label: "Host Port", Default: "8080", Description: "Port exposed on host", Required: false},
			},
			Volumes: []string{"nextcloud_data:/var/www/html"},
			Ports:   []string{"8080:80"},
			Source:  "curated",
		},
		{
			ID:          "vaultwarden",
			Title:       "Vaultwarden",
			Description: "Lightweight, unofficial Bitwarden compatible server written in Rust.",
			Categories:  []string{"Productivity"},
			Logo:        "https://cdn.jsdelivr.net/gh/walkxcode/dashboard-icons/png/vaultwarden.png",
			Platform:    "linux",
			Type:        "compose",
			ComposeYAML: `version: '3.8'
services:
  vaultwarden:
    image: vaultwarden/server:latest
    restart: unless-stopped
    environment:
      SIGNUPS_ALLOWED: ${SIGNUPS_ALLOWED:-true}
      ADMIN_TOKEN: ${ADMIN_TOKEN:-}
    ports:
      - "${VAULTWARDEN_PORT:-8000}:80"
    volumes:
      - vaultwarden_data:/data

volumes:
  vaultwarden_data:
`,
			Env: []TemplateEnvVar{
				{Name: "SIGNUPS_ALLOWED", Label: "Allow Signups", Default: "true", Description: "Enable or disable user registration", Required: false},
				{Name: "ADMIN_TOKEN", Label: "Admin Token", Default: "", Description: "Token to access /admin portal", Required: false},
				{Name: "VAULTWARDEN_PORT", Label: "Host Port", Default: "8000", Description: "Web vault port", Required: false},
			},
			Volumes: []string{"vaultwarden_data:/data"},
			Ports:   []string{"8000:80"},
			Source:  "curated",
		},
		{
			ID:          "minio",
			Title:       "MinIO",
			Description: "High Performance Object Storage compatible with Amazon S3 cloud storage service.",
			Categories:  []string{"Productivity"},
			Logo:        "https://cdn.jsdelivr.net/gh/walkxcode/dashboard-icons/png/minio.png",
			Platform:    "linux",
			Type:        "compose",
			ComposeYAML: `version: '3.8'
services:
  minio:
    image: minio/minio:latest
    restart: unless-stopped
    command: server /data --console-address ":9001"
    environment:
      MINIO_ROOT_USER: ${MINIO_ROOT_USER:-minioadmin}
      MINIO_ROOT_PASSWORD: ${MINIO_ROOT_PASSWORD:-minioadmin123}
    ports:
      - "${MINIO_API_PORT:-9000}:9000"
      - "${MINIO_CONSOLE_PORT:-9001}:9001"
    volumes:
      - minio_data:/data

volumes:
  minio_data:
`,
			Env: []TemplateEnvVar{
				{Name: "MINIO_ROOT_USER", Label: "Root User", Default: "minioadmin", Description: "Access key ID", Required: false},
				{Name: "MINIO_ROOT_PASSWORD", Label: "Root Password", Default: "minioadmin123", Description: "Secret access key", Required: true},
				{Name: "MINIO_API_PORT", Label: "API Port", Default: "9000", Description: "S3 API port", Required: false},
				{Name: "MINIO_CONSOLE_PORT", Label: "Console Port", Default: "9001", Description: "Web UI console port", Required: false},
			},
			Volumes: []string{"minio_data:/data"},
			Ports:   []string{"9000:9000", "9001:9001"},
			Source:  "curated",
		},
		{
			ID:          "ghost",
			Title:       "Ghost",
			Description: "Professional modern publishing platform for blogs, newsletters, and content creators.",
			Categories:  []string{"Productivity"},
			Logo:        "https://cdn.jsdelivr.net/gh/walkxcode/dashboard-icons/png/ghost.png",
			Platform:    "linux",
			Type:        "compose",
			ComposeYAML: `version: '3.8'
services:
  ghost:
    image: ghost:5-alpine
    restart: unless-stopped
    environment:
      url: ${GHOST_URL:-http://localhost:2368}
    ports:
      - "${GHOST_PORT:-2368}:2368"
    volumes:
      - ghost_content:/var/lib/ghost/content

volumes:
  ghost_content:
`,
			Env: []TemplateEnvVar{
				{Name: "GHOST_URL", Label: "Site URL", Default: "http://localhost:2368", Description: "Public URL for Ghost instance", Required: false},
				{Name: "GHOST_PORT", Label: "Host Port", Default: "2368", Description: "Web port", Required: false},
			},
			Volumes: []string{"ghost_content:/var/lib/ghost/content"},
			Ports:   []string{"2368:2368"},
			Source:  "curated",
		},
		{
			ID:          "wordpress",
			Title:       "WordPress",
			Description: "Web publishing software you can use to create a beautiful website or blog.",
			Categories:  []string{"Productivity"},
			Logo:        "https://cdn.jsdelivr.net/gh/walkxcode/dashboard-icons/png/wordpress.png",
			Platform:    "linux",
			Type:        "compose",
			ComposeYAML: `version: '3.8'
services:
  wordpress:
    image: wordpress:latest
    restart: unless-stopped
    environment:
      WORDPRESS_DB_HOST: db:3306
      WORDPRESS_DB_NAME: wordpress
      WORDPRESS_DB_USER: wp_user
      WORDPRESS_DB_PASSWORD: ${WP_DB_PASSWORD:-wp_secure_pass}
    ports:
      - "${WORDPRESS_PORT:-8080}:80"
    volumes:
      - wp_data:/var/www/html
    depends_on:
      - db

  db:
    image: mariadb:10.11
    restart: unless-stopped
    environment:
      MARIADB_DATABASE: wordpress
      MARIADB_USER: wp_user
      MARIADB_PASSWORD: ${WP_DB_PASSWORD:-wp_secure_pass}
      MARIADB_ROOT_PASSWORD: ${WP_ROOT_PASSWORD:-wp_root_pass}
    volumes:
      - db_data:/var/lib/mysql

volumes:
  wp_data:
  db_data:
`,
			Env: []TemplateEnvVar{
				{Name: "WP_DB_PASSWORD", Label: "Database Password", Default: "wp_secure_pass", Description: "MySQL user password", Required: true},
				{Name: "WP_ROOT_PASSWORD", Label: "Root DB Password", Default: "wp_root_pass", Description: "MySQL root password", Required: true},
				{Name: "WORDPRESS_PORT", Label: "Host Port", Default: "8080", Description: "Host port for WordPress", Required: false},
			},
			Volumes: []string{"wp_data:/var/www/html", "db_data:/var/lib/mysql"},
			Ports:   []string{"8080:80"},
			Source:  "curated",
		},
		{
			ID:          "paperless-ngx",
			Title:       "Paperless-ngx",
			Description: "Document management system that transforms physical documents into searchable electronic archives with OCR.",
			Categories:  []string{"Productivity"},
			Logo:        "https://cdn.jsdelivr.net/gh/walkxcode/dashboard-icons/png/paperless-ngx.png",
			Platform:    "linux",
			Type:        "compose",
			ComposeYAML: `version: '3.8'
services:
  webserver:
    image: ghcr.io/paperless-ngx/paperless-ngx:latest
    restart: unless-stopped
    environment:
      PAPERLESS_REDIS: redis://broker:6379
      PAPERLESS_SECRET_KEY: ${PAPERLESS_SECRET_KEY:-change-me-secret-key-32chars}
      PAPERLESS_ADMIN_USER: ${PAPERLESS_ADMIN_USER:-admin}
      PAPERLESS_ADMIN_PASSWORD: ${PAPERLESS_ADMIN_PASSWORD:-admin_pass}
    ports:
      - "${PAPERLESS_PORT:-8000}:8000"
    volumes:
      - paperless_data:/usr/src/paperless/data
      - paperless_media:/usr/src/paperless/media
    depends_on:
      - broker

  broker:
    image: redis:7-alpine
    restart: unless-stopped

volumes:
  paperless_data:
  paperless_media:
`,
			Env: []TemplateEnvVar{
				{Name: "PAPERLESS_SECRET_KEY", Label: "Secret Key", Default: "change-me-secret-key-32chars", Description: "Django encryption secret", Required: true},
				{Name: "PAPERLESS_ADMIN_USER", Label: "Admin Username", Default: "admin", Description: "Default admin username", Required: false},
				{Name: "PAPERLESS_ADMIN_PASSWORD", Label: "Admin Password", Default: "admin_pass", Description: "Default admin password", Required: true},
				{Name: "PAPERLESS_PORT", Label: "Host Port", Default: "8000", Description: "Web UI port", Required: false},
			},
			Volumes: []string{"paperless_data:/usr/src/paperless/data", "paperless_media:/usr/src/paperless/media"},
			Ports:   []string{"8000:8000"},
			Source:  "curated",
		},
		{
			ID:          "syncthing",
			Title:       "Syncthing",
			Description: "Continuous file synchronization program that synchronizes files between two or more computers in real time.",
			Categories:  []string{"Productivity"},
			Logo:        "https://cdn.jsdelivr.net/gh/walkxcode/dashboard-icons/png/syncthing.png",
			Platform:    "linux",
			Type:        "compose",
			ComposeYAML: `version: '3.8'
services:
  syncthing:
    image: syncthing/syncthing:latest
    restart: unless-stopped
    hostname: syncthing
    environment:
      - PUID=1000
      - PGID=1000
    ports:
      - "${SYNCTHING_WEB_PORT:-8384}:8384"
      - "${SYNCTHING_LISTEN_PORT:-22000}:22000/tcp"
      - "${SYNCTHING_LISTEN_PORT:-22000}:22000/udp"
    volumes:
      - syncthing_data:/var/syncthing

volumes:
  syncthing_data:
`,
			Env: []TemplateEnvVar{
				{Name: "SYNCTHING_WEB_PORT", Label: "Web GUI Port", Default: "8384", Description: "Web management GUI port", Required: false},
				{Name: "SYNCTHING_LISTEN_PORT", Label: "Sync Port", Default: "22000", Description: "Sync protocol listening port", Required: false},
			},
			Volumes: []string{"syncthing_data:/var/syncthing"},
			Ports:   []string{"8384:8384", "22000:22000"},
			Source:  "curated",
		},

		// ---------------------------------------------------------------------
		// DEV TOOLS & AUTOMATION
		// ---------------------------------------------------------------------
		{
			ID:          "gitea",
			Title:       "Gitea",
			Description: "Painless self-hosted Git service written in Go. Fast, lightweight, and easy to run.",
			Categories:  []string{"Dev Tools"},
			Logo:        "https://cdn.jsdelivr.net/gh/walkxcode/dashboard-icons/png/gitea.png",
			Platform:    "linux",
			Type:        "compose",
			ComposeYAML: `version: '3.8'
services:
  gitea:
    image: gitea/gitea:latest
    restart: unless-stopped
    environment:
      - USER_UID=1000
      - USER_GID=1000
    ports:
      - "${GITEA_HTTP_PORT:-3000}:3000"
      - "${GITEA_SSH_PORT:-2222}:22"
    volumes:
      - gitea_data:/data

volumes:
  gitea_data:
`,
			Env: []TemplateEnvVar{
				{Name: "GITEA_HTTP_PORT", Label: "HTTP Port", Default: "3000", Description: "Web UI port", Required: false},
				{Name: "GITEA_SSH_PORT", Label: "SSH Port", Default: "2222", Description: "Git SSH clone port", Required: false},
			},
			Volumes: []string{"gitea_data:/data"},
			Ports:   []string{"3000:3000", "2222:22"},
			Source:  "curated",
		},
		{
			ID:          "meilisearch",
			Title:       "Meilisearch",
			Description: "Lightning-fast, ultra relevant, and typo-tolerant search engine API.",
			Categories:  []string{"Dev Tools"},
			Logo:        "https://cdn.jsdelivr.net/gh/walkxcode/dashboard-icons/png/meilisearch.png",
			Platform:    "linux",
			Type:        "compose",
			ComposeYAML: `version: '3.8'
services:
  meilisearch:
    image: getmeili/meilisearch:latest
    restart: unless-stopped
    environment:
      MEILI_MASTER_KEY: ${MEILI_MASTER_KEY:-master_key_pdswarm}
      MEILI_ENV: ${MEILI_ENV:-production}
    ports:
      - "${MEILISEARCH_PORT:-7700}:7700"
    volumes:
      - meili_data:/meili_data

volumes:
  meili_data:
`,
			Env: []TemplateEnvVar{
				{Name: "MEILI_MASTER_KEY", Label: "Master Key", Default: "master_key_pdswarm", Description: "API master key for security", Required: true},
				{Name: "MEILI_ENV", Label: "Environment", Default: "production", Description: "Run environment", Required: false},
				{Name: "MEILISEARCH_PORT", Label: "Host Port", Default: "7700", Description: "HTTP API port", Required: false},
			},
			Volumes: []string{"meili_data:/meili_data"},
			Ports:   []string{"7700:7700"},
			Source:  "curated",
		},
		{
			ID:          "searxng",
			Title:       "SearXNG",
			Description: "Privacy-respecting, hackable metasearch engine aggregating results from multiple sources.",
			Categories:  []string{"Dev Tools"},
			Logo:        "https://cdn.jsdelivr.net/gh/walkxcode/dashboard-icons/png/searxng.png",
			Platform:    "linux",
			Type:        "compose",
			ComposeYAML: `version: '3.8'
services:
  searxng:
    image: searxng/searxng:latest
    restart: unless-stopped
    ports:
      - "${SEARXNG_PORT:-8080}:8080"
    volumes:
      - searxng_data:/etc/searxng

volumes:
  searxng_data:
`,
			Env: []TemplateEnvVar{
				{Name: "SEARXNG_PORT", Label: "Host Port", Default: "8080", Description: "Web UI port", Required: false},
			},
			Volumes: []string{"searxng_data:/etc/searxng"},
			Ports:   []string{"8080:8080"},
			Source:  "curated",
		},
		{
			ID:          "homeassistant",
			Title:       "Home Assistant",
			Description: "Open source home automation that puts local control and privacy first.",
			Categories:  []string{"Dev Tools"},
			Logo:        "https://cdn.jsdelivr.net/gh/walkxcode/dashboard-icons/png/home-assistant.png",
			Platform:    "linux",
			Type:        "compose",
			ComposeYAML: `version: '3.8'
services:
  homeassistant:
    image: ghcr.io/home-assistant/home-assistant:stable
    restart: unless-stopped
    ports:
      - "${HASS_PORT:-8123}:8123"
    volumes:
      - hass_config:/config

volumes:
  hass_config:
`,
			Env: []TemplateEnvVar{
				{Name: "HASS_PORT", Label: "Host Port", Default: "8123", Description: "Web UI port", Required: false},
			},
			Volumes: []string{"hass_config:/config"},
			Ports:   []string{"8123:8123"},
			Source:  "curated",
		},
		{
			ID:          "n8n",
			Title:       "n8n",
			Description: "Fair-code workflow automation tool allowing you to connect anything to everything.",
			Categories:  []string{"Dev Tools"},
			Logo:        "https://cdn.jsdelivr.net/gh/walkxcode/dashboard-icons/png/n8n.png",
			Platform:    "linux",
			Type:        "compose",
			ComposeYAML: `version: '3.8'
services:
  n8n:
    image: docker.n8n.io/n8nio/n8n:latest
    restart: unless-stopped
    environment:
      - N8N_BASIC_AUTH_ACTIVE=${N8N_AUTH_ACTIVE:-true}
      - N8N_BASIC_AUTH_USER=${N8N_USER:-admin}
      - N8N_BASIC_AUTH_PASSWORD=${N8N_PASSWORD:-n8n_password}
    ports:
      - "${N8N_PORT:-5678}:5678"
    volumes:
      - n8n_data:/home/node/.n8n

volumes:
  n8n_data:
`,
			Env: []TemplateEnvVar{
				{Name: "N8N_USER", Label: "Auth User", Default: "admin", Description: "Basic auth username", Required: false},
				{Name: "N8N_PASSWORD", Label: "Auth Password", Default: "n8n_password", Description: "Basic auth password", Required: true},
				{Name: "N8N_PORT", Label: "Host Port", Default: "5678", Description: "Web UI port", Required: false},
			},
			Volumes: []string{"n8n_data:/home/node/.n8n"},
			Ports:   []string{"5678:5678"},
			Source:  "curated",
		},
		{
			ID:          "portainer-ce",
			Title:       "Portainer CE",
			Description: "Universal container management platform for Docker, Docker Swarm, and Kubernetes.",
			Categories:  []string{"Dev Tools"},
			Logo:        "https://cdn.jsdelivr.net/gh/walkxcode/dashboard-icons/png/portainer.png",
			Platform:    "linux",
			Type:        "compose",
			ComposeYAML: `version: '3.8'
services:
  portainer:
    image: portainer/portainer-ce:latest
    restart: unless-stopped
    ports:
      - "${PORTAINER_HTTPS_PORT:-9443}:9443"
      - "${PORTAINER_HTTP_PORT:-9000}:9000"
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
      - portainer_data:/data

volumes:
  portainer_data:
`,
			Env: []TemplateEnvVar{
				{Name: "PORTAINER_HTTPS_PORT", Label: "HTTPS Port", Default: "9443", Description: "HTTPS web management port", Required: false},
				{Name: "PORTAINER_HTTP_PORT", Label: "HTTP Port", Default: "9000", Description: "HTTP web management port", Required: false},
			},
			Volumes: []string{"/var/run/docker.sock:/var/run/docker.sock", "portainer_data:/data"},
			Ports:   []string{"9443:9443", "9000:9000"},
			Source:  "curated",
		},

		// ---------------------------------------------------------------------
		// NETWORK & SECURITY
		// ---------------------------------------------------------------------
		{
			ID:          "adguard-home",
			Title:       "AdGuard Home",
			Description: "Network-wide software for blocking ads, trackers, and malicious domains via DNS.",
			Categories:  []string{"Network & Security"},
			Logo:        "https://cdn.jsdelivr.net/gh/walkxcode/dashboard-icons/png/adguard-home.png",
			Platform:    "linux",
			Type:        "compose",
			ComposeYAML: `version: '3.8'
services:
  adguardhome:
    image: adguard/adguardhome:latest
    restart: unless-stopped
    ports:
      - "${ADGUARD_PORT:-3000}:3000"
      - "${ADGUARD_DNS_PORT:-53}:53/tcp"
      - "${ADGUARD_DNS_PORT:-53}:53/udp"
    volumes:
      - adguard_work:/opt/adguardhome/work
      - adguard_conf:/opt/adguardhome/conf

volumes:
  adguard_work:
  adguard_conf:
`,
			Env: []TemplateEnvVar{
				{Name: "ADGUARD_PORT", Label: "Web Setup Port", Default: "3000", Description: "Initial web setup port", Required: false},
				{Name: "ADGUARD_DNS_PORT", Label: "DNS Port", Default: "53", Description: "DNS listening port", Required: false},
			},
			Volumes: []string{"adguard_work:/opt/adguardhome/work", "adguard_conf:/opt/adguardhome/conf"},
			Ports:   []string{"3000:3000", "53:53"},
			Source:  "curated",
		},
	}
}
