#!/usr/bin/env bash
set -euo pipefail

# This installer only supports Ubuntu/Debian-like Linux (systemd based).
# It is intended to be called from `make install`.

BINARY_NAME="log-collector"
DIST_DIR="dist"
INSTALL_BIN="/usr/local/bin"
CONFIG_DIR="/etc/log-collector"
SERVICE_DIR="/etc/systemd/system"
SERVICE_NAME="log-collector"

if [[ "$(uname -s)" != "Linux" ]]; then
  echo "ERROR: install.sh only supports Ubuntu/Linux (systemd). Current OS: $(uname -s)" >&2
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
    echo "ERROR: Unsupported architecture $ARCH_RAW" >&2
    exit 1
    ;;
esac

BIN_PATH="${DIST_DIR}/${BINARY_NAME}-linux-${ARCH}"
if [[ ! -f "$BIN_PATH" ]]; then
  echo "Building ${BIN_PATH}..."
  make "build-linux-${ARCH}"
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

echo "Installing systemd unit to ${SERVICE_DIR}/${SERVICE_NAME}.service"
sudo install -m 644 packaging/log-collector.service "${SERVICE_DIR}/${SERVICE_NAME}.service"

sudo systemctl daemon-reload
sudo systemctl enable "${SERVICE_NAME}"

echo ""
echo "Install done. Config file locations (edit these before starting the service):"
echo "  - Config YAML:  ${CONFIG_DIR}/log-collector.yaml"
echo "  - Env vars:     ${CONFIG_DIR}/.env"
echo ""
echo "Then start the service:"
echo "  sudo systemctl start ${SERVICE_NAME}"
echo "  sudo systemctl status ${SERVICE_NAME}"
echo "  journalctl -u ${SERVICE_NAME} -f"
