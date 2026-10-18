#!/usr/bin/env bash
set -euo pipefail

# ==============================================================================
# PDSwarm 1-Line Installer & Systemd Service Setup
# https://github.com/podiax/pdswarm
# ==============================================================================

REPO="podiax/pdswarm"
INSTALL_BIN="/usr/local/bin/pdswarm"
UPDATER_BIN="/usr/local/bin/pdswarm-update"
SERVICE_NAME="pdswarm.service"
TIMER_NAME="pdswarm-update.timer"
DEFAULT_DATA_DIR="/var/lib/pdswarm"
DEFAULT_PORT="9000"
DEFAULT_HOST="0.0.0.0"
DOCKER_SOCK="/var/run/docker.sock"

# Options
PORT="${PORT:-$DEFAULT_PORT}"
HOST="${HOST:-$DEFAULT_HOST}"
DATA_DIR="${DATA_DIR:-$DEFAULT_DATA_DIR}"
ENABLE_AUTO_UPDATE=true
LOCAL_INSTALL=false
DRY_RUN=false
UNINSTALL=false

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m' # No Color

log_info() { echo -e "${BLUE}[INFO]${NC} $*"; }
log_success() { echo -e "${GREEN}[SUCCESS]${NC} $*"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $*"; }
log_error() { echo -e "${RED}[ERROR]${NC} $*" >&2; }

# Parse arguments
while [[ $# -gt 0 ]]; do
  case "$1" in
    --port)
      PORT="$2"
      shift 2
      ;;
    --host)
      HOST="$2"
      shift 2
      ;;
    --data-dir)
      DATA_DIR="$2"
      shift 2
      ;;
    --socket)
      DOCKER_SOCK="$2"
      shift 2
      ;;
    --no-auto-update)
      ENABLE_AUTO_UPDATE=false
      shift
      ;;
    --local)
      LOCAL_INSTALL=true
      shift
      ;;
    --dry-run)
      DRY_RUN=true
      shift
      ;;
    --uninstall)
      UNINSTALL=true
      shift
      ;;
    -h|--help)
      echo "PDSwarm Installer"
      echo "Usage: sudo ./install.sh [options]"
      echo ""
      echo "Options:"
      echo "  --port <port>        Port to bind PDSwarm (default: 9000)"
      echo "  --host <host>        Host to bind PDSwarm (default: 0.0.0.0)"
      echo "  --data-dir <path>    Data directory (default: /var/lib/pdswarm)"
      echo "  --socket <path>      Container engine socket (default: /var/run/docker.sock or /run/podman/podman.sock)"
      echo "  --no-auto-update     Disable daily auto-update timer"
      echo "  --local              Install from local ./pdswarm binary instead of GitHub"
      echo "  --dry-run            Simulate installation without making system changes"
      echo "  --uninstall          Stop services and remove PDSwarm from system"
      echo "  -h, --help           Show this help"
      exit 0
      ;;
    *)
      log_warn "Unknown parameter: $1"
      shift
      ;;
  esac
done

# Root check
if [[ $EUID -ne 0 && "${DRY_RUN}" != "true" ]]; then
  log_error "This script must be run as root (or with sudo)."
  echo "Usage: curl -fsSL https://raw.githubusercontent.com/${REPO}/master/scripts/install.sh | sudo bash"
  exit 1
fi

# Handle uninstallation
if [[ "${UNINSTALL}" == "true" ]]; then
  log_info "Uninstalling PDSwarm..."
  if command -v systemctl >/dev/null 2>&1; then
    systemctl stop "${TIMER_NAME}" "${SERVICE_NAME}" 2>/dev/null || true
    systemctl disable "${TIMER_NAME}" "${SERVICE_NAME}" 2>/dev/null || true
    rm -f "/etc/systemd/system/${SERVICE_NAME}" "/etc/systemd/system/pdswarm-update.service" "/etc/systemd/system/${TIMER_NAME}"
    systemctl daemon-reload 2>/dev/null || true
  fi
  rm -f "${INSTALL_BIN}" "${UPDATER_BIN}"
  log_success "PDSwarm binary and services removed."
  log_warn "Note: Data folder '${DATA_DIR}' was preserved to prevent data loss. Delete manually if desired: rm -rf ${DATA_DIR}"
  exit 0
fi

# OS Check
OS="$(uname -s)"
if [[ "${OS}" != "Linux" ]]; then
  log_error "PDSwarm service installer currently supports Linux systems. Detected OS: ${OS}"
  exit 1
fi

# Systemd Check
if ! command -v systemctl >/dev/null 2>&1 || [[ ! -d /run/systemd/system && "${DRY_RUN}" != "true" ]]; then
  log_error "Systemd was not detected. PDSwarm service requires systemd. For non-systemd systems, please run with Docker."
  exit 1
fi

# Architecture detection
ARCH="$(uname -m)"
case "${ARCH}" in
  x86_64|amd64)
    TARGET_ARCH="amd64"
    ;;
  aarch64|arm64)
    TARGET_ARCH="arm64"
    ;;
  armv7l|armhf)
    TARGET_ARCH="armv7"
    ;;
  *)
    log_error "Unsupported architecture: ${ARCH}"
    exit 1
    ;;
esac

log_info "System: ${OS} (${TARGET_ARCH})"

# Docker / Podman Socket Check
if [[ ! -S "${DOCKER_SOCK}" ]]; then
  if [[ -S "/run/podman/podman.sock" ]]; then
    DOCKER_SOCK="/run/podman/podman.sock"
    log_info "Detected active Podman socket at ${DOCKER_SOCK}"
  else
    log_warn "Neither Docker socket (${DOCKER_SOCK}) nor Podman socket (/run/podman/podman.sock) was found."
    log_warn "Ensure Docker ('systemctl start docker') or Podman ('systemctl start podman.socket') is running."
  fi
else
  log_info "Found active container engine socket at ${DOCKER_SOCK}"
fi

if [[ "${DRY_RUN}" == "true" ]]; then
  log_info "[DRY-RUN] Would install PDSwarm to ${INSTALL_BIN}"
  log_info "[DRY-RUN] Would configure data directory at ${DATA_DIR}"
  log_info "[DRY-RUN] Would configure systemd service ${SERVICE_NAME} on port ${PORT}"
  if [[ "${ENABLE_AUTO_UPDATE}" == "true" ]]; then
    log_info "[DRY-RUN] Would configure daily auto-updater timer ${TIMER_NAME}"
  fi
  exit 0
fi

# Step 1: Install Binary
log_info "Installing PDSwarm binary..."
SCRIPT_DIR=""
if [[ -n "${BASH_SOURCE[0]:-}" ]]; then
  SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" 2>/dev/null && pwd || echo "")"
fi
LOCAL_BIN="${SCRIPT_DIR}/../pdswarm"

if [[ "${LOCAL_INSTALL}" == "true" || ( -n "${SCRIPT_DIR}" && -f "${LOCAL_BIN}" && -x "${LOCAL_BIN}" ) ]]; then
  log_info "Installing from local binary (${LOCAL_BIN})..."
  install -m 0755 "${LOCAL_BIN}" "${INSTALL_BIN}"
else
  # Query GitHub latest release
  RELEASE_API_URL="https://api.github.com/repos/${REPO}/releases/latest"
  log_info "Fetching latest release from ${RELEASE_API_URL}..."
  RELEASE_JSON="$(curl -fsSL -H "User-Agent: PDSwarm-Installer" "${RELEASE_API_URL}" 2>/dev/null || true)"
  
  DOWNLOAD_URL=""
  LATEST_TAG=""
  if [[ -n "${RELEASE_JSON}" ]]; then
    LATEST_TAG="$(echo "${RELEASE_JSON}" | grep -o '"tag_name": *"[^"]*"' | head -n1 | cut -d'"' -f4 || true)"
    DOWNLOAD_URL="$(echo "${RELEASE_JSON}" | grep -o "https://[^\"]*/releases/download/[^\"]*/pdswarm-linux-${TARGET_ARCH}" | head -n1 || true)"
    if [[ -z "${DOWNLOAD_URL}" ]]; then
      DOWNLOAD_URL="$(echo "${RELEASE_JSON}" | grep -o "https://[^\"]*/releases/download/[^\"]*/pdswarm-linux-${TARGET_ARCH}\.tar\.gz" | head -n1 || true)"
    fi
  fi

  if [[ -z "${DOWNLOAD_URL}" && -n "${LATEST_TAG}" ]]; then
    DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${LATEST_TAG}/pdswarm-linux-${TARGET_ARCH}"
  fi

  if [[ -z "${DOWNLOAD_URL}" ]]; then
    # Fallback if release assets not yet published: check if local binary is in current dir
    if [[ -f "./pdswarm" && -x "./pdswarm" ]]; then
      log_warn "No GitHub release asset found, using ./pdswarm from current working directory..."
      install -m 0755 "./pdswarm" "${INSTALL_BIN}"
    else
      log_error "Could not find a downloadable release asset for linux-${TARGET_ARCH} on GitHub."
      log_error "Please build the binary ('make build') and run 'sudo ./scripts/install.sh --local'."
      exit 1
    fi
  else
    log_info "Downloading ${DOWNLOAD_URL}..."
    TMP_DOWNLOAD="/tmp/pdswarm-${TARGET_ARCH}.$$"
    if ! curl -fsSL -H "User-Agent: PDSwarm-Installer" -o "${TMP_DOWNLOAD}" "${DOWNLOAD_URL}"; then
      rm -f "${TMP_DOWNLOAD}"
      log_error "Failed to download binary from ${DOWNLOAD_URL}"
      exit 1
    fi

    # If downloaded file is a tar gzip archive, extract the binary
    if [[ "${DOWNLOAD_URL}" == *.tar.gz ]] || (file "${TMP_DOWNLOAD}" 2>/dev/null | grep -qi "gzip compressed"); then
      TMP_DIR="/tmp/pdswarm-extract.$$"
      mkdir -p "${TMP_DIR}"
      tar -xzf "${TMP_DOWNLOAD}" -C "${TMP_DIR}"
      if [[ -f "${TMP_DIR}/pdswarm-linux-${TARGET_ARCH}" ]]; then
        install -m 0755 "${TMP_DIR}/pdswarm-linux-${TARGET_ARCH}" "${INSTALL_BIN}"
      elif [[ -f "${TMP_DIR}/pdswarm" ]]; then
        install -m 0755 "${TMP_DIR}/pdswarm" "${INSTALL_BIN}"
      else
        EXTRACTED="$(find "${TMP_DIR}" -type f -name "pdswarm*" -executable | head -n1)"
        if [[ -n "${EXTRACTED}" ]]; then
          install -m 0755 "${EXTRACTED}" "${INSTALL_BIN}"
        else
          rm -rf "${TMP_DIR}" "${TMP_DOWNLOAD}"
          log_error "Could not find pdswarm binary inside downloaded tar archive"
          exit 1
        fi
      fi
      rm -rf "${TMP_DIR}"
    else
      install -m 0755 "${TMP_DOWNLOAD}" "${INSTALL_BIN}"
    fi
    rm -f "${TMP_DOWNLOAD}"
  fi
fi

# Verify binary
INSTALLED_VER="$("${INSTALL_BIN}" -version 2>/dev/null || echo "PDSwarm")"
log_success "Installed ${INSTALLED_VER} to ${INSTALL_BIN}"

# Step 2: Create Data Directory
log_info "Creating data directory at ${DATA_DIR}..."
mkdir -p "${DATA_DIR}"
chmod 0750 "${DATA_DIR}"

# Step 3: Install Systemd Service
log_info "Configuring systemd service (${SERVICE_NAME})..."
cat <<EOF > "/etc/systemd/system/${SERVICE_NAME}"
[Unit]
Description=PDSwarm - Lightweight Docker & Compose Web Management
Documentation=https://github.com/podiax/pdswarm
After=network-online.target docker.service podman.socket
Wants=network-online.target docker.service podman.socket

[Service]
Type=simple
User=root
WorkingDirectory=${DATA_DIR}
ExecStart=${INSTALL_BIN} -data-dir ${DATA_DIR} -host ${HOST} -port ${PORT} -socket ${DOCKER_SOCK}
Restart=always
RestartSec=5s
LimitNOFILE=65535

# Process hardening
ProtectSystem=full
ProtectHome=true
ReadWritePaths=${DATA_DIR}

[Install]
WantedBy=multi-user.target
EOF

# Step 4: Install Auto-Updater
if [[ "${ENABLE_AUTO_UPDATE}" == "true" ]]; then
  log_info "Configuring automated daily updates..."

  # Install updater script
  cat <<'EOF' > "${UPDATER_BIN}"
#!/usr/bin/env bash
set -euo pipefail

REPO="podiax/pdswarm"
BIN_PATH="/usr/local/bin/pdswarm"
SERVICE_NAME="pdswarm.service"

ARCH="$(uname -m)"
case "${ARCH}" in
  x86_64|amd64) TARGET_ARCH="amd64" ;;
  aarch64|arm64) TARGET_ARCH="arm64" ;;
  armv7l|armhf) TARGET_ARCH="armv7" ;;
  *) echo "[ERROR] Unsupported architecture: ${ARCH}" >&2; exit 1 ;;
esac

CURRENT_VERSION="v0.0.0"
if [[ -f "${BIN_PATH}" && -x "${BIN_PATH}" ]]; then
  CURRENT_VERSION="$("${BIN_PATH}" -version 2>/dev/null | grep -o 'v[0-9]\+\.[0-9]\+\.[0-9]\+' || echo "v0.0.0")"
fi

RELEASE_API_URL="https://api.github.com/repos/${REPO}/releases/latest"
RELEASE_JSON="$(curl -fsSL -H "User-Agent: PDSwarm-Updater" "${RELEASE_API_URL}" 2>/dev/null || true)"
if [[ -z "${RELEASE_JSON}" ]]; then
  exit 0
fi

LATEST_TAG="$(echo "${RELEASE_JSON}" | grep -o '"tag_name": *"[^"]*"' | head -n1 | cut -d'"' -f4 || true)"
if [[ -z "${LATEST_TAG}" || "${CURRENT_VERSION}" == "${LATEST_TAG}" ]]; then
  exit 0
fi

DOWNLOAD_URL="$(echo "${RELEASE_JSON}" | grep -o "https://[^\"]*/releases/download/[^\"]*/pdswarm-linux-${TARGET_ARCH}" | head -n1 || true)"
if [[ -z "${DOWNLOAD_URL}" ]]; then
  DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${LATEST_TAG}/pdswarm-linux-${TARGET_ARCH}"
fi

TMP_BIN="${BIN_PATH}.tmp.$$"
if curl -fsSL -H "User-Agent: PDSwarm-Updater" -o "${TMP_BIN}" "${DOWNLOAD_URL}"; then
  chmod +x "${TMP_BIN}"
  if "${TMP_BIN}" -version 2>&1 | grep -q "PDSwarm"; then
    mv -f "${TMP_BIN}" "${BIN_PATH}"
    systemctl restart "${SERVICE_NAME}" 2>/dev/null || true
    logger -t pdswarm-update "PDSwarm successfully updated to ${LATEST_TAG}"
  else
    rm -f "${TMP_BIN}"
  fi
fi
EOF
  chmod 0755 "${UPDATER_BIN}"

  # Systemd Updater Service
  cat <<EOF > "/etc/systemd/system/pdswarm-update.service"
[Unit]
Description=PDSwarm Auto-Updater Service
Documentation=https://github.com/podiax/pdswarm
After=network-online.target

[Service]
Type=oneshot
ExecStart=${UPDATER_BIN}
EOF

  # Systemd Updater Timer (daily at 04:00 with random delay)
  cat <<EOF > "/etc/systemd/system/${TIMER_NAME}"
[Unit]
Description=Daily Auto-Update Check for PDSwarm
Documentation=https://github.com/podiax/pdswarm

[Timer]
OnCalendar=*-*-* 04:00:00
RandomizedDelaySec=1800
Persistent=true

[Install]
WantedBy=timers.target
EOF
fi

# Step 5: Reload and Enable Systemd Services
log_info "Reloading systemd and enabling services..."
systemctl daemon-reload
systemctl enable --now "${SERVICE_NAME}"

if [[ "${ENABLE_AUTO_UPDATE}" == "true" ]]; then
  systemctl enable --now "${TIMER_NAME}"
fi

# Get host IP for user guidance
HOST_IP="$(hostname -I 2>/dev/null | awk '{print $1}' || echo "localhost")"

echo ""
echo -e "${GREEN}${BOLD}================================================================${NC}"
echo -e "${GREEN}${BOLD}           PDSwarm Successfully Installed & Started!            ${NC}"
echo -e "${GREEN}${BOLD}================================================================${NC}"
echo ""
echo -e "  ${BOLD}Web UI:${NC}              ${CYAN}http://${HOST_IP}:${PORT}${NC}"
echo -e "  ${BOLD}Service Status:${NC}      sudo systemctl status ${SERVICE_NAME}"
echo -e "  ${BOLD}Live Logs:${NC}           sudo journalctl -u ${SERVICE_NAME} -f"
echo -e "  ${BOLD}Data Directory:${NC}      ${DATA_DIR}"
if [[ "${ENABLE_AUTO_UPDATE}" == "true" ]]; then
  echo -e "  ${BOLD}Auto-Updates:${NC}        Enabled (daily via ${TIMER_NAME})"
  echo -e "  ${BOLD}Manual Update:${NC}       sudo ${UPDATER_BIN}"
fi
echo -e "  ${BOLD}Uninstall:${NC}           sudo ${INSTALL_BIN} --uninstall (or ./install.sh --uninstall)"
echo ""
echo -e "${GREEN}Open ${CYAN}http://${HOST_IP}:${PORT}${GREEN} in your browser to get started!${NC}"
echo ""
