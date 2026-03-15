#!/usr/bin/env bash
set -euo pipefail

# This installer supports:
# - Ubuntu/Debian-like Linux (systemd based)
# - macOS (Darwin) via launchd LaunchAgent
# It is intended to be called from `make install`.

BINARY_NAME="log-collector"
DIST_DIR="dist"
INSTALL_BIN="/usr/local/bin"
CONFIG_DIR="/etc/log-collector"
SERVICE_DIR="/etc/systemd/system"
SERVICE_NAME="log-collector"

OS_RAW="$(uname -s)"
case "$OS_RAW" in
  Linux)
    TARGET_OS="linux"
    ;;
  Darwin)
    TARGET_OS="darwin"
    # macOS 上更推荐把配置放到 /usr/local/etc 下
    CONFIG_DIR="/usr/local/etc/log-collector"
    ;;
  *)
    echo "ERROR: install.sh only supports Ubuntu/Linux or macOS. Current OS: ${OS_RAW}" >&2
    exit 1
    ;;
esac

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

BIN_PATH="${DIST_DIR}/${BINARY_NAME}-${TARGET_OS}-${ARCH}"
if [[ ! -f "$BIN_PATH" ]]; then
  echo "Building ${BIN_PATH}..."
  make "build-${TARGET_OS}-${ARCH}"
fi

echo "Installing binary to ${INSTALL_BIN}/${BINARY_NAME}"
sudo install -m 755 "$BIN_PATH" "${INSTALL_BIN}/${BINARY_NAME}"

echo "Creating config directory ${CONFIG_DIR}"
sudo mkdir -p "${CONFIG_DIR}"

echo "Installing config template to ${CONFIG_DIR}/log-collector.yaml"
sudo install -m 644 log-collector.example.yaml "${CONFIG_DIR}/log-collector.yaml"

if [[ -f .env.example ]]; then
  if [[ ! -f "${CONFIG_DIR}/.env" ]]; then
    echo "Installing env template to ${CONFIG_DIR}/.env"
    sudo install -m 600 .env.example "${CONFIG_DIR}/.env"
  else
    echo "Keeping existing ${CONFIG_DIR}/.env"
  fi
else
  echo "No .env.example found, skip .env"
fi

if [[ "${TARGET_OS:-}" == "linux" ]]; then
  echo "Installing systemd unit to ${SERVICE_DIR}/${SERVICE_NAME}.service"
  sudo install -m 644 packaging/log-collector.service "${SERVICE_DIR}/${SERVICE_NAME}.service"

  sudo systemctl daemon-reload
  sudo systemctl enable "${SERVICE_NAME}"
else
  # macOS: 使用 per-user LaunchAgent，通过 launchctl 管理
  LAUNCH_AGENTS_DIR="${HOME}/Library/LaunchAgents"
  PLIST_PATH="${LAUNCH_AGENTS_DIR}/com.my-hq-log-collector.${SERVICE_NAME}.plist"

  echo "Creating LaunchAgents directory at ${LAUNCH_AGENTS_DIR}"
  mkdir -p "${LAUNCH_AGENTS_DIR}"

  echo "Installing launchd plist to ${PLIST_PATH}"
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
  <string>${HOME}/Library/Logs/log-collector.out.log</string>

  <key>StandardErrorPath</key>
  <string>${HOME}/Library/Logs/log-collector.err.log</string>
</dict>
</plist>
EOF

  echo "Loading LaunchAgent via launchctl"
  launchctl unload "${PLIST_PATH}" >/dev/null 2>&1 || true
  launchctl load -w "${PLIST_PATH}"
fi

echo ""
echo "Install done. Config file locations (edit these before starting the service):"
echo "  - Config YAML:  ${CONFIG_DIR}/log-collector.yaml"
echo "  - Env vars:     ${CONFIG_DIR}/.env"
echo ""
if [[ "${TARGET_OS:-}" == "linux" ]]; then
  echo "Then manage the service with systemd, for example:"
  echo "  sudo systemctl start ${SERVICE_NAME}"
  echo "  sudo systemctl status ${SERVICE_NAME}"
  echo "  journalctl -u ${SERVICE_NAME} -f"
else
  echo "Then manage the service with launchctl, for example:"
  echo "  launchctl list | grep ${SERVICE_NAME}"
  echo "  launchctl unload ${PLIST_PATH}"
  echo "  launchctl load -w ${PLIST_PATH}"
fi
