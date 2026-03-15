#!/usr/bin/env bash
set -euo pipefail

# Install log-collector on macOS as a system-level LaunchDaemon (runs as root),
# suitable for monitoring logs from multiple users.
#
# Usage:
#   sudo ./scripts/install-to-system-level-in-macos.sh

BINARY_NAME="log-collector"
DIST_DIR="dist"
INSTALL_BIN="/usr/local/bin"
CONFIG_DIR="/etc/log-collector"
LAUNCH_DAEMONS_DIR="/Library/LaunchDaemons"
SERVICE_NAME="log-collector"
PLIST_PATH="${LAUNCH_DAEMONS_DIR}/com.my-hq-log-collector.${SERVICE_NAME}.plist"

OS_RAW="$(uname -s)"
if [[ "${OS_RAW}" != "Darwin" ]]; then
  echo "ERROR: This script only supports macOS (Darwin). Current OS: ${OS_RAW}" >&2
  exit 1
fi

if [[ "${EUID}" -ne 0 ]]; then
  echo "ERROR: This script must be run as root (use sudo)." >&2
  exit 1
fi

ARCH_RAW="$(uname -m)"
case "$ARCH_RAW" in
  x86_64)
    ARCH="amd64"
    ;;
  aarch64|arm64)
    ARCH="arm64"
    ;;
  *)
    echo "ERROR: Unsupported architecture ${ARCH_RAW}" >&2
    exit 1
    ;;
esac

BIN_PATH="${DIST_DIR}/${BINARY_NAME}-darwin-${ARCH}"
if [[ ! -f "$BIN_PATH" ]]; then
  echo "Building ${BIN_PATH}..."
  make "build-darwin-${ARCH}"
fi

echo "Installing binary to ${INSTALL_BIN}/${BINARY_NAME}"
install -m 755 "$BIN_PATH" "${INSTALL_BIN}/${BINARY_NAME}"

echo "Creating config directory ${CONFIG_DIR}"
mkdir -p "${CONFIG_DIR}"

echo "Installing config template to ${CONFIG_DIR}/log-collector.yaml"
install -m 644 log-collector.example.yaml "${CONFIG_DIR}/log-collector.yaml"

if [[ -f .env.example ]]; then
  if [[ ! -f "${CONFIG_DIR}/.env" ]]; then
    echo "Installing env template to ${CONFIG_DIR}/.env"
    install -m 600 .env.example "${CONFIG_DIR}/.env"
  else
    echo "Keeping existing ${CONFIG_DIR}/.env"
  fi
else
  echo "No .env.example found, skip .env"
fi

echo "Creating LaunchDaemons directory at ${LAUNCH_DAEMONS_DIR}"
mkdir -p "${LAUNCH_DAEMONS_DIR}"

echo "Installing launchd system-level plist to ${PLIST_PATH}"
cat > "${PLIST_PATH}" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>com.my-hq-log-collector.${SERVICE_NAME}</string>

  <key>ProgramArguments</key>
  <array>
    <string>${INSTALL_BIN}/${BINARY_NAME}</string>
    <string>-config</string>
    <string>${CONFIG_DIR}/log-collector.yaml</string>
  </array>

  <key>WorkingDirectory</key>
  <string>${CONFIG_DIR}</string>

  <key>RunAtLoad</key>
  <true/>

  <key>KeepAlive</key>
  <true/>

  <key>StandardOutPath</key>
  <string>/var/log/log-collector.out.log</string>

  <key>StandardErrorPath</key>
  <string>/var/log/log-collector.err.log</string>
</dict>
</plist>
EOF

chmod 644 "${PLIST_PATH}"
chown root:wheel "${PLIST_PATH}"

echo "Loading LaunchDaemon via launchctl (system-level)"
launchctl unload "${PLIST_PATH}" >/dev/null 2>&1 || true
launchctl load -w "${PLIST_PATH}"

echo ""
echo "System-level install on macOS is done."
echo "Config file locations:"
echo "  - Config YAML:  ${CONFIG_DIR}/log-collector.yaml"
echo "  - Env vars:     ${CONFIG_DIR}/.env"
echo ""
echo "Manage the system service with launchctl, for example:"
echo "  sudo launchctl list | grep ${SERVICE_NAME}"
echo "  sudo launchctl unload ${PLIST_PATH}"
echo "  sudo launchctl load -w ${PLIST_PATH}"

