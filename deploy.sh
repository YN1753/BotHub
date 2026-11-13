#!/usr/bin/env bash
set -euo pipefail

# Build static Linux amd64 binary (ZERO CGO, ZERO DOCKER)
echo "[1/3] Compiling static binary for linux/amd64..."
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o bothub-linux-amd64 .
echo "Built: $(pwd)/bothub-linux-amd64 ($(du -h bothub-linux-amd64 | awk '{print $1}'))"

if [[ $# -lt 1 ]]; then
  echo ""
  echo "Usage: ./deploy.sh <user@vps_ip> [ssh_port]"
  echo "Example: ./deploy.sh root@1.2.3.4 22"
  echo ""
  echo "Manual upload commands:"
  echo "  ssh root@<VPS_IP> 'mkdir -p /opt/bothub/data'"
  echo "  scp bothub-linux-amd64 root@<VPS_IP>:/opt/bothub/bothub"
  echo "  scp bothub.service root@<VPS_IP>:/etc/systemd/system/bothub.service"
  echo "  ssh root@<VPS_IP> 'chmod +x /opt/bothub/bothub && systemctl daemon-reload && systemctl enable --now bothub'"
  exit 0
fi

TARGET="$1"
PORT="${2:-22}"

echo "[2/3] Uploading binary and systemd unit to ${TARGET}:${PORT}..."
ssh -p "${PORT}" "${TARGET}" "mkdir -p /opt/bothub/data && systemctl stop bothub 2>/dev/null || true"
scp -P "${PORT}" bothub-linux-amd64 "${TARGET}:/opt/bothub/bothub"
scp -P "${PORT}" bothub.service "${TARGET}:/etc/systemd/system/bothub.service"

echo "[3/3] Enabling and starting bothub.service..."
ssh -p "${PORT}" "${TARGET}" "chmod +x /opt/bothub/bothub && systemctl daemon-reload && systemctl enable --now bothub && systemctl status bothub --no-pager"
