#!/usr/bin/env bash
set -euo pipefail

# ==============================================================================
# PDSwarm Standalone Auto-Updater
# Checks GitHub Releases for new PDSwarm versions, verifies, and updates.
# ==============================================================================

REPO="podiax/pdswarm"
BIN_PATH="${BIN_PATH:-/usr/local/bin/pdswarm}"
SERVICE_NAME="pdswarm.service"
FORCE_UPDATE="${FORCE_UPDATE:-false}"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

log_info() { echo -e "${BLUE}[INFO]${NC} $*"; }
log_success() { echo -e "${GREEN}[SUCCESS]${NC} $*"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $*"; }
log_error() { echo -e "${RED}[ERROR]${NC} $*" >&2; }

# Parse arguments
while [[ $# -gt 0 ]]; do
  case "$1" in
    -f|--force)
      FORCE_UPDATE="true"
      shift
      ;;
    -h|--help)
      echo "PDSwarm Auto-Updater"
      echo "Usage: $0 [options]"
      echo "  -f, --force    Force update even if version is already up to date"
      echo "  -h, --help     Show this help"
      exit 0
      ;;
    *)
      log_warn "Unknown argument: $1"
      shift
      ;;
  esac
done

# Check root permissions
if [[ $EUID -ne 0 ]]; then
  log_error "This script requires root privileges. Please run with sudo: sudo $0"
  exit 1
fi

# Detect architecture
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

# Check current installed binary
CURRENT_VERSION="unknown"
if [[ -f "${BIN_PATH}" && -x "${BIN_PATH}" ]]; then
  CURRENT_VERSION="$("${BIN_PATH}" -version 2>/dev/null | grep -o 'v[0-9]\+\.[0-9]\+\.[0-9]\+' || echo "v0.0.0")"
fi
log_info "Current PDSwarm version: ${CURRENT_VERSION} (${TARGET_ARCH})"

# Fetch latest release info from GitHub API
RELEASE_API_URL="https://api.github.com/repos/${REPO}/releases/latest"
log_info "Checking for updates at ${RELEASE_API_URL}..."

RELEASE_JSON="$(curl -fsSL -H "User-Agent: PDSwarm-Updater" "${RELEASE_API_URL}" 2>/dev/null || true)"
if [[ -z "${RELEASE_JSON}" ]]; then
  log_warn "Unable to reach GitHub Releases API or repository has no releases yet."
  exit 0
fi

# Extract tag_name
LATEST_TAG="$(echo "${RELEASE_JSON}" | grep -o '"tag_name": *"[^"]*"' | head -n1 | cut -d'"' -f4 || true)"
if [[ -z "${LATEST_TAG}" ]]; then
  log_warn "Could not parse latest release tag from GitHub."
  exit 0
fi

log_info "Latest PDSwarm release: ${LATEST_TAG}"

# Compare versions
if [[ "${FORCE_UPDATE}" != "true" && "${CURRENT_VERSION}" != "unknown" && "${CURRENT_VERSION}" == "${LATEST_TAG}" ]]; then
  log_success "PDSwarm is already up to date (${CURRENT_VERSION})."
  exit 0
fi

log_info "Upgrading PDSwarm: ${CURRENT_VERSION} -> ${LATEST_TAG}..."

# Look for binary download URL
DOWNLOAD_URL="$(echo "${RELEASE_JSON}" | grep -o "https://[^\"]*/releases/download/[^\"]*/pdswarm-linux-${TARGET_ARCH}" | head -n1 || true)"

# Fallback pattern if assets are named pdswarm-linux-amd64
if [[ -z "${DOWNLOAD_URL}" ]]; then
  DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${LATEST_TAG}/pdswarm-linux-${TARGET_ARCH}"
fi

TMP_BIN="${BIN_PATH}.tmp.$$"
log_info "Downloading ${DOWNLOAD_URL}..."

if ! curl -fsSL -H "User-Agent: PDSwarm-Updater" -o "${TMP_BIN}" "${DOWNLOAD_URL}"; then
  rm -f "${TMP_BIN}"
  log_error "Failed to download update binary from ${DOWNLOAD_URL}"
  exit 1
fi

chmod +x "${TMP_BIN}"

# Validate downloaded binary
TEST_OUTPUT="$("${TMP_BIN}" -version 2>&1 || true)"
if ! echo "${TEST_OUTPUT}" | grep -q "PDSwarm"; then
  rm -f "${TMP_BIN}"
  log_error "Downloaded file is not a valid PDSwarm binary: ${TEST_OUTPUT}"
  exit 1
fi

# Atomic replacement
mv -f "${TMP_BIN}" "${BIN_PATH}"
log_success "PDSwarm binary updated successfully to ${LATEST_TAG} at ${BIN_PATH}."

# Restart systemd service if running
if command -v systemctl >/dev/null 2>&1; then
  if systemctl is-active --quiet "${SERVICE_NAME}" 2>/dev/null; then
    log_info "Restarting ${SERVICE_NAME}..."
    systemctl restart "${SERVICE_NAME}"
    log_success "${SERVICE_NAME} restarted successfully."
  elif systemctl is-enabled --quiet "${SERVICE_NAME}" 2>/dev/null; then
    log_info "Starting ${SERVICE_NAME}..."
    systemctl start "${SERVICE_NAME}"
    log_success "${SERVICE_NAME} started successfully."
  fi
fi

log_success "PDSwarm update to ${LATEST_TAG} complete!"
