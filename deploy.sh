#!/usr/bin/env bash
set -euo pipefail

# Vesper BotHub Deployment Utility (Zero Docker · Single Binary)

ACTION="${1:-build}"

if [[ "${ACTION}" == "download" ]]; then
  TARGET_FILE="bothub-linux-amd64"
  echo "Downloading latest linux/amd64 static binary from GitHub Releases..."
  curl -fsSL -o "${TARGET_FILE}" "https://github.com/YN1753/BotHub/releases/latest/download/bothub-linux-amd64"
  chmod +x "${TARGET_FILE}"
  echo "Downloaded: $(pwd)/${TARGET_FILE}"
  exit 0
fi

if [[ "${ACTION}" == "build" && $# -lt 2 ]]; then
  echo "[1/2] Compiling static binary for linux/amd64 (Embedding Frontend)..."
  CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o bothub-linux-amd64 .
  echo "Built: $(pwd)/bothub-linux-amd64 ($(du -h bothub-linux-amd64 | awk '{print $1}'))"
  echo ""
  echo "Usage:"
  echo "  ./deploy.sh <user@vps_ip> [ssh_port]   # Build locally and upload to VPS"
  echo "  ./deploy.sh download                   # Download latest release binary from GitHub"
  echo ""
  echo "Remote VPS one-line upgrade via release:"
  echo "  curl -fsSL -o /opt/bothub/bothub https://github.com/YN1753/BotHub/releases/latest/download/bothub-linux-amd64 && chmod +x /opt/bothub/bothub && systemctl restart bothub"
  exit 0
fi

TARGET="$1"
PORT="${2:-22}"

if [[ ! -f "bothub-linux-amd64" ]]; then
  echo "Compiling static binary for linux/amd64..."
  CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o bothub-linux-amd64 .
fi

echo "[1/2] Uploading binary and systemd unit to ${TARGET}:${PORT}..."
ssh -p "${PORT}" "${TARGET}" "mkdir -p /opt/bothub/data && systemctl stop bothub 2>/dev/null || true"
scp -P "${PORT}" bothub-linux-amd64 "${TARGET}:/opt/bothub/bothub"
scp -P "${PORT}" bothub.service "${TARGET}:/etc/systemd/system/bothub.service"

echo "[2/2] Enabling and starting bothub.service..."
ssh -p "${PORT}" "${TARGET}" "chmod +x /opt/bothub/bothub && systemctl daemon-reload && systemctl enable --now bothub && systemctl status bothub --no-pager"
