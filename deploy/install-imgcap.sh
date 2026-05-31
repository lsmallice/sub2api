#!/usr/bin/env bash
#
# Sub2API imgcap binary/systemd installer
#
# Usage:
#   curl -sSL https://raw.githubusercontent.com/lsmallice/sub2api/image-capability/deploy/install-imgcap.sh | sudo bash
#   curl -sSL https://raw.githubusercontent.com/lsmallice/sub2api/image-capability/deploy/install-imgcap.sh | sudo IMG_CAP_REPO=lsmallice/sub2api bash
#   curl -sSL https://raw.githubusercontent.com/lsmallice/sub2api/image-capability/deploy/install-imgcap.sh | sudo bash -s -- --version imgcap-0.1.133
#
# This installer intentionally does not install PostgreSQL or Redis. Use existing
# services, such as PostgreSQL/Redis installed by 1Panel, and point Sub2API at
# them via the DATABASE_* and REDIS_* settings below.

set -Eeuo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

DEFAULT_RELEASE_REPO="lsmallice/sub2api"
INSTALL_DIR="${INSTALL_DIR:-/opt/sub2api}"
CONFIG_DIR="${CONFIG_DIR:-/etc/sub2api}"
SERVICE_NAME="${SERVICE_NAME:-sub2api}"
SERVICE_USER="${SERVICE_USER:-sub2api}"
SERVICE_GROUP="${SERVICE_GROUP:-sub2api}"
BACKUP_DIR="${BACKUP_DIR:-/opt/sub2api/backups}"

RELEASE_REPO="${IMG_CAP_REPO:-${SUB2API_RELEASE_REPO:-${GITHUB_REPO:-$DEFAULT_RELEASE_REPO}}}"
RELEASE_VERSION="${IMG_CAP_VERSION:-${SUB2API_VERSION:-}}"
ASSET_VERSION="${IMG_CAP_BASE_VERSION:-${BASE_VERSION:-}}"
ARCHIVE_URL="${IMG_CAP_ARCHIVE_URL:-}"
ARCHIVE_PATH="${IMG_CAP_ARCHIVE:-}"
BINARY_PATH="${IMG_CAP_BINARY:-}"

SERVER_HOST="${SERVER_HOST:-0.0.0.0}"
SERVER_PORT="${SERVER_PORT:-8080}"
SERVER_MODE="${SERVER_MODE:-release}"
DATA_DIR="${DATA_DIR:-$INSTALL_DIR/data}"
TZ_VALUE="${TZ:-${TIMEZONE:-Asia/Shanghai}}"

DATABASE_HOST="${DATABASE_HOST:-127.0.0.1}"
DATABASE_PORT="${DATABASE_PORT:-5432}"
DATABASE_USER="${DATABASE_USER:-postgres}"
DATABASE_PASSWORD="${DATABASE_PASSWORD:-}"
DATABASE_DBNAME="${DATABASE_DBNAME:-sub2api}"
DATABASE_SSLMODE="${DATABASE_SSLMODE:-disable}"

REDIS_HOST="${REDIS_HOST:-127.0.0.1}"
REDIS_PORT="${REDIS_PORT:-6379}"
REDIS_PASSWORD="${REDIS_PASSWORD:-}"
REDIS_DB="${REDIS_DB:-0}"
REDIS_ENABLE_TLS="${REDIS_ENABLE_TLS:-false}"

JWT_SECRET="${JWT_SECRET:-}"
TOTP_ENCRYPTION_KEY="${TOTP_ENCRYPTION_KEY:-}"
AUTO_SETUP="${AUTO_SETUP:-false}"
ADMIN_EMAIL="${ADMIN_EMAIL:-admin@sub2api.local}"
ADMIN_PASSWORD="${ADMIN_PASSWORD:-}"

FORCE_ENV=false
NO_START=false
NO_ENABLE=false
COMMAND="install"
OS=""
ARCH=""
TEMP_DIR=""
DOWNLOADED_VERSION=""

print_info() {
  printf "${BLUE}[INFO]${NC} %s\n" "$1"
}

print_success() {
  printf "${GREEN}[SUCCESS]${NC} %s\n" "$1"
}

print_warning() {
  printf "${YELLOW}[WARNING]${NC} %s\n" "$1"
}

print_error() {
  printf "${RED}[ERROR]${NC} %s\n" "$1" >&2
}

usage() {
  cat <<'USAGE'
Sub2API imgcap binary/systemd installer

Commands:
  install                  Install or reinstall Sub2API imgcap (default)
  upgrade                  Download a release and replace the existing binary
  list-versions            List recent GitHub release tags
  uninstall                Remove systemd service and binary files

Release source:
  --repo OWNER/REPO        GitHub release repo (default: lsmallice/sub2api)
  --version VERSION        Release tag, for example imgcap-0.1.133 or v0.1.133
  --base-version VERSION   Release asset version, for example 0.1.133.
                           Defaults to VERSION with v/imgcap prefixes removed.
  --archive-url URL        Direct tar.gz URL. Skips GitHub release discovery
  --archive PATH           Local release tar.gz archive
  --binary PATH            Local sub2api binary

Install options:
  --install-dir DIR        Install directory (default: /opt/sub2api)
  --config-dir DIR         Config directory (default: /etc/sub2api)
  --service-name NAME      systemd service name (default: sub2api)
  --service-user USER      system user (default: sub2api)
  --server-host HOST       Listen host (default: 0.0.0.0)
  --server-port PORT       Listen port (default: 8080)
  --force-env              Rewrite /etc/sub2api/sub2api.env if it already exists
  --no-start               Do not start/restart the service
  --no-enable              Do not enable the service on boot
  -h, --help               Show this help

Environment configuration:
  DATABASE_HOST            Default: 127.0.0.1
  DATABASE_PORT            Default: 5432
  DATABASE_USER            Default: postgres
  DATABASE_PASSWORD        Default: empty
  DATABASE_DBNAME          Default: sub2api
  DATABASE_SSLMODE         Default: disable
  REDIS_HOST               Default: 127.0.0.1
  REDIS_PORT               Default: 6379
  REDIS_PASSWORD           Default: empty
  REDIS_DB                 Default: 0
  REDIS_ENABLE_TLS         Default: false
  AUTO_SETUP               Default: false. Set true only when DB/Redis/admin
                           credentials are ready for automatic setup.
  ADMIN_EMAIL              Used only by AUTO_SETUP=true
  ADMIN_PASSWORD           Used only by AUTO_SETUP=true
  JWT_SECRET               Generated when missing
  TOTP_ENCRYPTION_KEY      Generated when missing

1Panel example:
  curl -sSL https://raw.githubusercontent.com/lsmallice/sub2api/image-capability/deploy/install-imgcap.sh \
    | sudo DATABASE_USER=sub2api DATABASE_PASSWORD='***' DATABASE_DBNAME=sub2api REDIS_PASSWORD='***' bash

The script installs only the Sub2API binary and systemd service. It never
installs or exposes PostgreSQL/Redis and never deploys Docker Compose.
USAGE
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    install|upgrade|uninstall|list-versions)
      COMMAND="$1"
      shift
      ;;
    --repo)
      RELEASE_REPO="${2:-}"
      shift 2
      ;;
    --version)
      RELEASE_VERSION="${2:-}"
      shift 2
      ;;
    --archive-url)
      ARCHIVE_URL="${2:-}"
      shift 2
      ;;
    --base-version)
      ASSET_VERSION="${2:-}"
      shift 2
      ;;
    --archive)
      ARCHIVE_PATH="${2:-}"
      shift 2
      ;;
    --binary)
      BINARY_PATH="${2:-}"
      shift 2
      ;;
    --install-dir)
      INSTALL_DIR="${2:-}"
      DATA_DIR="${DATA_DIR:-$INSTALL_DIR/data}"
      BACKUP_DIR="${BACKUP_DIR:-$INSTALL_DIR/backups}"
      shift 2
      ;;
    --config-dir)
      CONFIG_DIR="${2:-}"
      shift 2
      ;;
    --service-name)
      SERVICE_NAME="${2:-}"
      shift 2
      ;;
    --service-user)
      SERVICE_USER="${2:-}"
      SERVICE_GROUP="$SERVICE_USER"
      shift 2
      ;;
    --server-host)
      SERVER_HOST="${2:-}"
      shift 2
      ;;
    --server-port)
      SERVER_PORT="${2:-}"
      shift 2
      ;;
    --force-env)
      FORCE_ENV=true
      shift
      ;;
    --no-start)
      NO_START=true
      shift
      ;;
    --no-enable)
      NO_ENABLE=true
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      print_error "Unknown argument: $1"
      usage
      exit 2
      ;;
  esac
done

cleanup() {
  if [[ -n "${TEMP_DIR:-}" && -d "$TEMP_DIR" ]]; then
    rm -rf "$TEMP_DIR"
  fi
}
trap cleanup EXIT

command_exists() {
  command -v "$1" >/dev/null 2>&1
}

check_root() {
  if [[ "$(id -u)" -ne 0 ]]; then
    print_error "Please run as root, for example with sudo."
    exit 1
  fi
}

validate_port() {
  local port="$1"
  [[ "$port" =~ ^[0-9]+$ ]] && [[ "$port" -ge 1 ]] && [[ "$port" -le 65535 ]]
}

check_release_repo() {
  if [[ -z "$RELEASE_REPO" && -z "$ARCHIVE_URL" && -z "$ARCHIVE_PATH" && -z "$BINARY_PATH" ]]; then
    print_error "Missing GitHub release repo. Set IMG_CAP_REPO=owner/repo or pass --repo owner/repo."
    exit 2
  fi

  if [[ "$RELEASE_REPO" == "Wei-Shaw/sub2api" || "$RELEASE_REPO" == "weishaw/sub2api" ]]; then
    print_warning "Release repo is official upstream: $RELEASE_REPO"
    print_warning "For the customized imgcap build, prefer --repo lsmallice/sub2api or IMG_CAP_REPO=lsmallice/sub2api."
  fi
}

detect_platform() {
  OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
  ARCH="$(uname -m)"

  if [[ "$OS" != "linux" ]]; then
    print_error "Unsupported OS for systemd install: $OS"
    exit 1
  fi

  case "$ARCH" in
    x86_64|amd64)
      ARCH="amd64"
      ;;
    aarch64|arm64)
      ARCH="arm64"
      ;;
    *)
      print_error "Unsupported architecture: $ARCH"
      exit 1
      ;;
  esac

  print_info "Detected platform: ${OS}_${ARCH}"
}

check_dependencies() {
  local missing=()
  local cmd

  for cmd in curl tar systemctl; do
    if ! command_exists "$cmd"; then
      missing+=("$cmd")
    fi
  done

  if ! command_exists sha256sum && ! command_exists shasum; then
    missing+=("sha256sum or shasum")
  fi

  if [[ ${#missing[@]} -gt 0 ]]; then
    print_error "Missing dependencies: ${missing[*]}"
    exit 1
  fi
}

normalize_version() {
  local version="$1"
  if [[ -n "$version" && "$version" != v* ]]; then
    version="v$version"
  fi
  printf '%s' "$version"
}

derive_asset_version() {
  local release_tag="$1"
  local version="${ASSET_VERSION:-}"

  if [[ -z "$version" ]]; then
    version="$release_tag"
    version="${version#refs/tags/}"
    version="${version#imgcap-}"
    version="${version#imgcap-v}"
    version="${version#v}"
  fi

  if [[ ! "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+ ]]; then
    print_error "Cannot derive release asset version from tag '$release_tag'."
    print_error "Pass --base-version 0.1.133 or set IMG_CAP_BASE_VERSION=0.1.133."
    exit 2
  fi

  printf '%s' "$version"
}

get_latest_version() {
  if [[ -n "$RELEASE_VERSION" ]]; then
    DOWNLOADED_VERSION="$(normalize_version "$RELEASE_VERSION")"
    return
  fi

  print_info "Fetching latest release from $RELEASE_REPO ..."
  DOWNLOADED_VERSION="$(curl -fsSL --connect-timeout 10 --max-time 30 "https://api.github.com/repos/${RELEASE_REPO}/releases/latest" \
    | grep '"tag_name"' \
    | sed -E 's/.*"([^"]+)".*/\1/' \
    | head -1 || true)"

  if [[ -z "$DOWNLOADED_VERSION" ]]; then
    print_error "Failed to fetch latest release from $RELEASE_REPO."
    print_error "Pass --version, --archive-url, --archive, or --binary if the release is private or unavailable."
    exit 1
  fi

  print_info "Latest release: $DOWNLOADED_VERSION"
}

list_versions() {
  check_release_repo
  print_info "Fetching recent releases from $RELEASE_REPO ..."
  curl -fsSL --connect-timeout 10 --max-time 30 "https://api.github.com/repos/${RELEASE_REPO}/releases" \
    | grep '"tag_name"' \
    | sed -E 's/.*"([^"]+)".*/\1/' \
    | head -20
}

checksum_file() {
  local file="$1"
  if command_exists sha256sum; then
    sha256sum "$file" | awk '{print $1}'
  else
    shasum -a 256 "$file" | awk '{print $1}'
  fi
}

download_release_archive() {
  TEMP_DIR="$(mktemp -d)"

  if [[ -n "$BINARY_PATH" ]]; then
    if [[ ! -f "$BINARY_PATH" ]]; then
      print_error "Binary not found: $BINARY_PATH"
      exit 1
    fi
    cp "$BINARY_PATH" "$TEMP_DIR/sub2api"
    chmod +x "$TEMP_DIR/sub2api"
    return
  fi

  if [[ -n "$ARCHIVE_PATH" ]]; then
    if [[ ! -f "$ARCHIVE_PATH" ]]; then
      print_error "Archive not found: $ARCHIVE_PATH"
      exit 1
    fi
    cp "$ARCHIVE_PATH" "$TEMP_DIR/sub2api.tar.gz"
  else
    local archive_name
    local download_url

    if [[ -n "$ARCHIVE_URL" ]]; then
      download_url="$ARCHIVE_URL"
      archive_name="${ARCHIVE_URL##*/}"
      [[ -n "$archive_name" ]] || archive_name="sub2api.tar.gz"
    else
      check_release_repo
      get_latest_version
      local version_num
      version_num="$(derive_asset_version "$DOWNLOADED_VERSION")"
      archive_name="sub2api_${version_num}_${OS}_${ARCH}.tar.gz"
      download_url="https://github.com/${RELEASE_REPO}/releases/download/${DOWNLOADED_VERSION}/${archive_name}"
    fi

    print_info "Downloading $archive_name ..."
    curl -fL --connect-timeout 10 --max-time 300 "$download_url" -o "$TEMP_DIR/sub2api.tar.gz"

    if [[ -z "$ARCHIVE_URL" && -n "$DOWNLOADED_VERSION" ]]; then
      local checksum_url="https://github.com/${RELEASE_REPO}/releases/download/${DOWNLOADED_VERSION}/checksums.txt"
      if curl -fsSL --connect-timeout 10 --max-time 30 "$checksum_url" -o "$TEMP_DIR/checksums.txt"; then
        local expected
        local actual
        expected="$(grep "$archive_name" "$TEMP_DIR/checksums.txt" | awk '{print $1}' | head -1 || true)"
        actual="$(checksum_file "$TEMP_DIR/sub2api.tar.gz")"
        if [[ -n "$expected" ]]; then
          if [[ "$expected" != "$actual" ]]; then
            print_error "Checksum failed for $archive_name"
            print_error "Expected: $expected"
            print_error "Actual:   $actual"
            exit 1
          fi
          print_success "Checksum verified"
        else
          print_warning "checksums.txt does not contain $archive_name; skipping checksum validation."
        fi
      else
        print_warning "checksums.txt not found; skipping checksum validation."
      fi
    fi
  fi

  print_info "Extracting release archive ..."
  tar -xzf "$TEMP_DIR/sub2api.tar.gz" -C "$TEMP_DIR"

  if [[ ! -f "$TEMP_DIR/sub2api" ]]; then
    local found
    found="$(find "$TEMP_DIR" -type f -name sub2api -perm -111 | head -1 || true)"
    if [[ -z "$found" ]]; then
      print_error "Release archive did not contain an executable sub2api binary."
      exit 1
    fi
    cp "$found" "$TEMP_DIR/sub2api"
  fi

  chmod +x "$TEMP_DIR/sub2api"
}

random_hex() {
  if command_exists openssl; then
    openssl rand -hex 32
  else
    LC_ALL=C tr -dc 'a-f0-9' </dev/urandom | head -c 64
    printf '\n'
  fi
}

ensure_secrets() {
  if [[ -z "$JWT_SECRET" ]]; then
    JWT_SECRET="$(random_hex)"
  fi
  if [[ -z "$TOTP_ENCRYPTION_KEY" ]]; then
    TOTP_ENCRYPTION_KEY="$(random_hex)"
  fi
}

env_quote() {
  local value="$1"
  value="${value//\\/\\\\}"
  value="${value//\"/\\\"}"
  printf '"%s"' "$value"
}

write_env_line() {
  local key="$1"
  local value="$2"
  printf '%s=%s\n' "$key" "$(env_quote "$value")"
}

write_environment_file() {
  local env_file="$CONFIG_DIR/sub2api.env"

  if [[ -f "$env_file" && "$FORCE_ENV" != "true" ]]; then
    print_info "Keeping existing environment file: $env_file"
    print_info "Use --force-env to rewrite it."
    return
  fi

  if [[ -f "$env_file" ]]; then
    local ts
    ts="$(date +%Y%m%d-%H%M%S)"
    cp "$env_file" "$env_file.bak-$ts"
    print_success "Backed up existing environment file to $env_file.bak-$ts"
  fi

  ensure_secrets

  {
    printf '# Sub2API imgcap systemd environment\n'
    printf '# Generated by deploy/install-imgcap.sh on %s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
    write_env_line GIN_MODE "$SERVER_MODE"
    write_env_line SERVER_MODE "$SERVER_MODE"
    write_env_line SERVER_HOST "$SERVER_HOST"
    write_env_line SERVER_PORT "$SERVER_PORT"
    write_env_line DATA_DIR "$DATA_DIR"
    write_env_line TZ "$TZ_VALUE"
    write_env_line DATABASE_HOST "$DATABASE_HOST"
    write_env_line DATABASE_PORT "$DATABASE_PORT"
    write_env_line DATABASE_USER "$DATABASE_USER"
    write_env_line DATABASE_PASSWORD "$DATABASE_PASSWORD"
    write_env_line DATABASE_DBNAME "$DATABASE_DBNAME"
    write_env_line DATABASE_SSLMODE "$DATABASE_SSLMODE"
    write_env_line REDIS_HOST "$REDIS_HOST"
    write_env_line REDIS_PORT "$REDIS_PORT"
    write_env_line REDIS_PASSWORD "$REDIS_PASSWORD"
    write_env_line REDIS_DB "$REDIS_DB"
    write_env_line REDIS_ENABLE_TLS "$REDIS_ENABLE_TLS"
    write_env_line JWT_SECRET "$JWT_SECRET"
    write_env_line TOTP_ENCRYPTION_KEY "$TOTP_ENCRYPTION_KEY"
    write_env_line AUTO_SETUP "$AUTO_SETUP"
    write_env_line ADMIN_EMAIL "$ADMIN_EMAIL"
    write_env_line ADMIN_PASSWORD "$ADMIN_PASSWORD"
  } > "$env_file"

  chmod 0600 "$env_file"
  chown root:"$SERVICE_GROUP" "$env_file" 2>/dev/null || chown root:root "$env_file"
  print_success "Environment file written: $env_file"
}

create_user() {
  if ! getent group "$SERVICE_GROUP" >/dev/null 2>&1; then
    print_info "Creating system group: $SERVICE_GROUP"
    groupadd --system "$SERVICE_GROUP"
  fi

  if id "$SERVICE_USER" >/dev/null 2>&1; then
    print_info "System user exists: $SERVICE_USER"
    local current_shell
    current_shell="$(getent passwd "$SERVICE_USER" 2>/dev/null | cut -d: -f7 || true)"
    if [[ "$current_shell" == "/bin/false" || "$current_shell" == "/sbin/nologin" ]]; then
      usermod -s /bin/sh "$SERVICE_USER" || true
    fi
    return
  fi

  print_info "Creating system user: $SERVICE_USER"
  useradd -r -g "$SERVICE_GROUP" -s /bin/sh -d "$INSTALL_DIR" "$SERVICE_USER"
}

setup_directories() {
  mkdir -p "$INSTALL_DIR" "$DATA_DIR" "$CONFIG_DIR" "$BACKUP_DIR"
  chown -R "$SERVICE_USER:$SERVICE_GROUP" "$INSTALL_DIR"
  chown -R "$SERVICE_USER:$SERVICE_GROUP" "$DATA_DIR"
  chmod 0750 "$INSTALL_DIR" "$DATA_DIR" "$BACKUP_DIR"
  chmod 0750 "$CONFIG_DIR"
}

backup_existing_binary() {
  if [[ -f "$INSTALL_DIR/sub2api" ]]; then
    local ts
    ts="$(date +%Y%m%d-%H%M%S)"
    mkdir -p "$BACKUP_DIR"
    cp "$INSTALL_DIR/sub2api" "$BACKUP_DIR/sub2api-$ts"
    chmod 0750 "$BACKUP_DIR/sub2api-$ts"
    print_success "Existing binary backed up: $BACKUP_DIR/sub2api-$ts"
  fi
}

install_binary() {
  download_release_archive
  backup_existing_binary
  install -m 0755 -o "$SERVICE_USER" -g "$SERVICE_GROUP" "$TEMP_DIR/sub2api" "$INSTALL_DIR/sub2api"
  print_success "Binary installed: $INSTALL_DIR/sub2api"

  local version_file
  version_file="$(mktemp)"
  if "$INSTALL_DIR/sub2api" --version >"$version_file" 2>/dev/null; then
    print_info "Installed version: $(head -1 "$version_file")"
  fi
  rm -f "$version_file"
}

install_service() {
  local service_file="/etc/systemd/system/${SERVICE_NAME}.service"

  cat > "$service_file" <<EOF
[Unit]
Description=Sub2API imgcap - AI API Gateway Platform
Documentation=https://github.com/${RELEASE_REPO}
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=${SERVICE_USER}
Group=${SERVICE_GROUP}
WorkingDirectory=${INSTALL_DIR}
EnvironmentFile=-${CONFIG_DIR}/sub2api.env
ExecStart=${INSTALL_DIR}/sub2api
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal
SyslogIdentifier=${SERVICE_NAME}

NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
PrivateTmp=true
ReadWritePaths=${INSTALL_DIR} ${DATA_DIR}

[Install]
WantedBy=multi-user.target
EOF

  systemctl daemon-reload
  print_success "systemd service installed: $service_file"
}

start_service() {
  if [[ "$NO_ENABLE" != "true" ]]; then
    systemctl enable "$SERVICE_NAME" >/dev/null
    print_success "Enabled auto-start: $SERVICE_NAME"
  fi

  if [[ "$NO_START" == "true" ]]; then
    print_info "Skipping service start because --no-start was set."
    return
  fi

  print_info "Starting service: $SERVICE_NAME"
  if systemctl restart "$SERVICE_NAME"; then
    print_success "Service started: $SERVICE_NAME"
  else
    print_error "Service failed to start. Check logs with:"
    print_error "  journalctl -u $SERVICE_NAME -n 100 --no-pager"
    exit 1
  fi
}

print_completion() {
  echo ""
  print_success "Sub2API imgcap installation completed."
  echo ""
  echo "Install dir: $INSTALL_DIR"
  echo "Config env:  $CONFIG_DIR/sub2api.env"
  echo "Service:     $SERVICE_NAME"
  echo "Listen:      ${SERVER_HOST}:${SERVER_PORT}"
  echo "Database:    ${DATABASE_HOST}:${DATABASE_PORT}/${DATABASE_DBNAME}"
  echo "Redis:       ${REDIS_HOST}:${REDIS_PORT} db=${REDIS_DB}"
  echo ""
  echo "Useful commands:"
  echo "  systemctl status $SERVICE_NAME"
  echo "  journalctl -u $SERVICE_NAME -f"
  echo "  systemctl restart $SERVICE_NAME"
  echo ""
  echo "Open setup/admin page:"
  echo "  http://<server-ip>:${SERVER_PORT}"
}

install_or_upgrade() {
  check_root
  check_release_repo
  detect_platform
  check_dependencies

  if ! validate_port "$SERVER_PORT"; then
    print_error "Invalid SERVER_PORT: $SERVER_PORT"
    exit 2
  fi

  create_user
  setup_directories
  write_environment_file

  if systemctl list-unit-files "${SERVICE_NAME}.service" >/dev/null 2>&1; then
    systemctl stop "$SERVICE_NAME" >/dev/null 2>&1 || true
  fi

  install_binary
  install_service
  start_service
  print_completion
}

uninstall() {
  check_root
  print_warning "This will remove the $SERVICE_NAME service and $INSTALL_DIR."
  print_warning "It will keep $CONFIG_DIR by default."
  read -r -p "Continue? [y/N] " answer < /dev/tty || answer="n"
  case "$answer" in
    y|Y|yes|YES)
      ;;
    *)
      print_info "Uninstall cancelled."
      exit 0
      ;;
  esac

  systemctl stop "$SERVICE_NAME" >/dev/null 2>&1 || true
  systemctl disable "$SERVICE_NAME" >/dev/null 2>&1 || true
  rm -f "/etc/systemd/system/${SERVICE_NAME}.service"
  systemctl daemon-reload
  rm -rf "$INSTALL_DIR"
  if id "$SERVICE_USER" >/dev/null 2>&1; then
    userdel "$SERVICE_USER" >/dev/null 2>&1 || true
  fi
  print_success "Uninstalled. Config directory kept: $CONFIG_DIR"
}

case "$COMMAND" in
  install|upgrade)
    install_or_upgrade
    ;;
  list-versions)
    list_versions
    ;;
  uninstall)
    uninstall
    ;;
esac
