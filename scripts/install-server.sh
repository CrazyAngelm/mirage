#!/usr/bin/env bash
set -euo pipefail

MIRAGE_RELEASE_BASE_URL="${MIRAGE_RELEASE_BASE_URL:-}"
MIRAGE_SERVER_URL="${MIRAGE_SERVER_URL:-}"
MIRAGE_SHA256SUMS_URL="${MIRAGE_SHA256SUMS_URL:-}"
AUTO_SETUP="no"
SETUP_HOST=""

while [ "$#" -gt 0 ]; do
  case "$1" in
    --auto) AUTO_SETUP="yes" ;;
    --host) shift; SETUP_HOST="${1:-}" ;;
    --release-base-url) shift; MIRAGE_RELEASE_BASE_URL="${1:-}" ;;
    *) echo "unknown argument: $1" >&2; exit 2 ;;
  esac
  shift
done

if [ -n "$MIRAGE_RELEASE_BASE_URL" ]; then
  MIRAGE_SERVER_URL="${MIRAGE_SERVER_URL:-$MIRAGE_RELEASE_BASE_URL/mirage-server-linux-amd64}"
  MIRAGE_SHA256SUMS_URL="${MIRAGE_SHA256SUMS_URL:-$MIRAGE_RELEASE_BASE_URL/SHA256SUMS}"
fi

if [ "$(id -u)" -ne 0 ]; then
  echo "run as root" >&2
  exit 1
fi

ensure_swap() {
  local total_mem total_swap
  total_mem=$(awk '/MemTotal/{print $2}' /proc/meminfo)
  total_swap=$(awk '/SwapTotal/{print $2}' /proc/meminfo)
  if [ "$total_swap" -lt 524288 ] && [ "$total_mem" -lt 2097152 ]; then
    echo "Low memory detected ($total_mem kB RAM, $total_swap kB swap). Creating 1 GB swapfile..."
    if [ ! -f /swapfile ]; then
      fallocate -l 1G /swapfile || dd if=/dev/zero of=/swapfile bs=1M count=1024
      chmod 600 /swapfile
      mkswap /swapfile
      swapon /swapfile
      echo "/swapfile none swap sw 0 0" >> /etc/fstab
    fi
  fi
}
ensure_swap

if ! command -v curl >/dev/null 2>&1; then
  apt-get update -qq
  apt-get install -y -qq curl ca-certificates
fi

install -d -m 0755 /opt/mirage/bin /opt/mirage/configs /opt/mirage/state /opt/mirage/logs /opt/mirage/web

verify_checksum() {
  local file="$1"
  local name="$2"
  if [ -z "$MIRAGE_SHA256SUMS_URL" ]; then
    return 0
  fi
  local sums line
  sums="$(mktemp)"
  curl -fsSL "$MIRAGE_SHA256SUMS_URL" -o "$sums"
  line="$(grep "  $name$" "$sums" || true)"
  if [ -z "$line" ]; then
    rm -f "$sums"
    echo "checksum for $name not found" >&2
    return 1
  fi
  echo "$line" | (cd "$(dirname "$file")" && sha256sum -c -)
  rm -f "$sums"
}

install_binary() {
  local name="$1"
  local url_var="$2"
  local sha_var="${3:-}"
  local target="/opt/mirage/bin/$name"
  local url="${!url_var:-}"
  local expected_sha="${!sha_var:-}"

  if [ -f "./$name" ]; then
    install -m 0755 "./$name" "$target"
    echo "$name installed from local file"
    return 0
  fi

  if [ -n "$url" ]; then
    tmp="$(mktemp)"
    curl -fsSL "$url" -o "$tmp"
    if [ -n "$expected_sha" ]; then
      echo "$expected_sha  $tmp" | sha256sum -c - || { rm -f "$tmp"; return 1; }
    fi
    install -m 0755 "$tmp" "$target"
    rm -f "$tmp"
    echo "$name downloaded from $url_var"
    return 0
  fi

  echo "$name missing; place ./$name next to script or set $url_var" >&2
  return 1
}

if [ -f ./mirage-server ]; then
  install -m 0755 ./mirage-server /opt/mirage/mirage-server
elif [ -f ./mirage-server-linux-amd64 ]; then
  install -m 0755 ./mirage-server-linux-amd64 /opt/mirage/mirage-server
elif [ -n "$MIRAGE_SERVER_URL" ]; then
  tmp="$(mktemp)"
  curl -fsSL "$MIRAGE_SERVER_URL" -o "$tmp"
  install -m 0755 "$tmp" /opt/mirage/mirage-server
  rm -f "$tmp"
  verify_checksum /opt/mirage/mirage-server mirage-server-linux-amd64
else
  echo "mirage-server missing; place ./mirage-server next to script or set MIRAGE_SERVER_URL" >&2
  exit 1
fi

missing=0
/opt/mirage/mirage-server download-sidecars || missing=1
install_binary amneziawg AMNEZIAWG_URL AMNEZIAWG_SHA256 || true

cat >/etc/systemd/system/mirage-server.service <<'SERVICE'
[Unit]
Description=MIRAGE server
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=/opt/mirage/mirage-server start
Restart=on-failure
RestartSec=5s
WorkingDirectory=/opt/mirage
LimitNOFILE=65536
MemoryMax=512M
MemorySwapMax=1G
TasksMax=256
CPUQuota=80%

[Install]
WantedBy=multi-user.target
SERVICE

open_port() {
  local proto="$1"
  local port="$2"
  if command -v ufw >/dev/null 2>&1; then
    ufw allow "$port/$proto" || true
  fi
  if iptables -C INPUT -p "$proto" --dport "$port" -j ACCEPT 2>/dev/null; then
    :
  else
    local reject_line
    reject_line=$(iptables -L INPUT --line-numbers -n 2>/dev/null | awk '/REJECT/ {print $1; exit}')
    if [ -n "$reject_line" ]; then
      iptables -I INPUT "$reject_line" -p "$proto" --dport "$port" -j ACCEPT
    else
      iptables -A INPUT -p "$proto" --dport "$port" -j ACCEPT
    fi
  fi
}

open_port tcp 443
open_port udp 443
open_port udp 51820

if command -v netfilter-persistent >/dev/null 2>&1; then
  netfilter-persistent save >/dev/null 2>&1 || true
fi

systemctl daemon-reload
systemctl enable mirage-server.service

if [ "$AUTO_SETUP" = "yes" ]; then
  if [ -n "$SETUP_HOST" ]; then
    /opt/mirage/mirage-server setup --auto --host "$SETUP_HOST"
  else
    /opt/mirage/mirage-server setup --auto
  fi
  systemctl restart mirage-server.service
  /opt/mirage/mirage-server show-link
else
  cat <<'NEXT'
installed.

Next:
  /opt/mirage/mirage-server setup --auto
  systemctl start mirage-server
  /opt/mirage/mirage-server show-link
NEXT
fi

if [ "$missing" -ne 0 ]; then
  exit 2
fi
