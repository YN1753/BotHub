#!/usr/bin/env bash
# Vesper BotHub 2.0 · 单目录原生打包与 1 分钟迁机脚本 (Zero Docker + SQLite WAL + systemd)
set -euo pipefail

INSTALL_DIR="/opt/bothub"
SERVICE_FILE="/etc/systemd/system/bothub.service"
BUNDLE_OUT="/tmp/bothub_migration_$(date +%Y%m%d_%H%M%S).tar.gz"

usage() {
  cat <<EOF
用法:
  $0 pack                     打包当前节点 /opt/bothub (含二进制与 SQLite 数据库) 到 tar.gz
  $0 restore <bundle.tar.gz>  在目标主力机上一键解压恢复 /opt/bothub 并注册启动 systemd 服务
EOF
}

cmd="${1:-}"

case "$cmd" in
  pack)
    echo "[1/3] 正在停止 bothub 服务以确保 SQLite WAL 完整落盘..."
    systemctl stop bothub || true

    echo "[2/3] 正在打包 ${INSTALL_DIR}..."
    tar -czf "${BUNDLE_OUT}" -C / opt/bothub

    echo "[3/3] 重新启动当前节点 bothub 服务..."
    systemctl start bothub || true

    echo "打包完成: ${BUNDLE_OUT}"
    echo "传输至新主力机后执行: sudo bash migrate_vps.sh restore ${BUNDLE_OUT}"
    ;;

  restore)
    bundle="${2:-}"
    if [[ -z "$bundle" || ! -f "$bundle" ]]; then
      echo "错误: 请指定有效的备份归档文件路径"
      usage
      exit 1
    fi

    echo "[1/3] 正在解压归档至 /opt/bothub..."
    mkdir -p /opt/bothub/data
    tar -xzf "$bundle" -C /
    chmod +x /opt/bothub/bothub

    echo "[2/3] 正在写入 systemd 单元文件 ${SERVICE_FILE}..."
    cat > "${SERVICE_FILE}" <<'UNIT'
[Unit]
Description=Vesper BotHub 2.0 Intelligence & Native Push Hub
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
WorkingDirectory=/opt/bothub
ExecStart=/opt/bothub/bothub -addr 127.0.0.1:8091 -db /opt/bothub/data/bothub.db
Restart=always
RestartSec=3
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
UNIT

    echo "[3/3] 正在重载 systemd 并启动 bothub 服务..."
    systemctl daemon-reload
    systemctl enable bothub
    systemctl restart bothub
    sleep 2
    systemctl is-active --quiet bothub && echo "迁机成功: Vesper BotHub 2.0 已在新节点正常运行!"
    ;;

  *)
    usage
    exit 1
    ;;
esac
