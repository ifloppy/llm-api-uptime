#!/bin/sh
set -eu

REPO=${REPO:-ifloppy/llm-api-uptime}
INSTALL_DIR=${INSTALL_DIR:-/opt/llm-api-uptime}
SERVICE_NAME=${SERVICE_NAME:-llm-api-uptime}
SERVICE_USER=${SERVICE_USER:-llm-api-uptime}
SERVICE_GROUP=${SERVICE_GROUP:-llm-api-uptime}
WEB_PORT=${WEB_PORT:-8080}

die() {
    printf 'Error: %s\n' "$*" >&2
    exit 1
}

need() {
    command -v "$1" >/dev/null 2>&1 || die "required command not found: $1"
}

cleanup() {
    if [ -n "${TMP_DIR:-}" ] && [ -d "$TMP_DIR" ]; then
        rm -rf "$TMP_DIR"
    fi
}
trap cleanup EXIT HUP INT TERM

[ "$(uname -s)" = "Linux" ] || die "this installer supports Linux only"
[ "$(id -u)" -eq 0 ] || die "run as root (for example: curl ... | sudo sh)"
case "$INSTALL_DIR" in
    /opt/*) ;;
    *) die "INSTALL_DIR must be under /opt" ;;
esac

need curl
need sha256sum
need systemctl
need install
need getent
need groupadd
need useradd
need od
need tr
need grep

NOLOGIN=$(command -v nologin || true)
[ -n "$NOLOGIN" ] || die "nologin shell not found"

case "$(uname -m)" in
    x86_64|amd64) ARCH=amd64 ;;
    aarch64|arm64) ARCH=arm64 ;;
    *) die "unsupported architecture: $(uname -m)" ;;
esac

if [ -n "${VERSION:-}" ]; then
    TAG=$VERSION
else
    LATEST_URL=$(curl -fsSL -o /dev/null -w '%{url_effective}' "https://github.com/$REPO/releases/latest")
    TAG=${LATEST_URL##*/}
fi
printf '%s' "$TAG" | grep -Eq '^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$' \
    || die "could not resolve a stable release tag (got '$TAG')"

ASSET="llm-api-uptime_${TAG}_linux_${ARCH}"
BASE_URL="https://github.com/$REPO/releases/download/$TAG"
TMP_DIR=$(mktemp -d)

curl -fL --retry 3 --retry-delay 2 -o "$TMP_DIR/$ASSET" "$BASE_URL/$ASSET"
curl -fL --retry 3 --retry-delay 2 -o "$TMP_DIR/checksums.txt" "$BASE_URL/checksums.txt"
(
    cd "$TMP_DIR"
    grep "  $ASSET\$" checksums.txt > checksums.asset || die "checksum entry missing for $ASSET"
    sha256sum -c checksums.asset
)

if ! getent group "$SERVICE_GROUP" >/dev/null 2>&1; then
    groupadd --system "$SERVICE_GROUP"
fi
if ! id "$SERVICE_USER" >/dev/null 2>&1; then
    useradd --system --gid "$SERVICE_GROUP" --home-dir "$INSTALL_DIR" --shell "$NOLOGIN" "$SERVICE_USER"
fi

install -d -m 0750 -o "$SERVICE_USER" -g "$SERVICE_GROUP" "$INSTALL_DIR"
install -d -m 0750 -o "$SERVICE_USER" -g "$SERVICE_GROUP" "$INSTALL_DIR/data"
install -m 0755 -o "$SERVICE_USER" -g "$SERVICE_GROUP" "$TMP_DIR/$ASSET" "$INSTALL_DIR/llm-api-uptime.new"
mv -f "$INSTALL_DIR/llm-api-uptime.new" "$INSTALL_DIR/llm-api-uptime"

GENERATED_PASSWORD=
if [ ! -f "$INSTALL_DIR/.env" ]; then
    PASSWORD=${WEB_PASSWORD:-}
    if [ -z "$PASSWORD" ]; then
        PASSWORD=$(od -An -N24 -tx1 /dev/urandom | tr -d ' \n')
        GENERATED_PASSWORD=$PASSWORD
    fi
    printf '%s' "$PASSWORD" | LC_ALL=C grep -Eq '^[-A-Za-z0-9_.,:@%+=!/]+$' \
        || die "WEB_PASSWORD contains unsupported characters"
    cat > "$INSTALL_DIR/.env" <<EOF
PROBE_INTERVAL=${PROBE_INTERVAL:-5m}
PROBE_TIMEOUT=${PROBE_TIMEOUT:-30s}
PROBE_CONCURRENCY=${PROBE_CONCURRENCY:-3}
DB_PATH=$INSTALL_DIR/data/uptime.db
DATA_RETENTION=${DATA_RETENTION:-720h}
WEB_ENABLED=true
WEB_PORT=$WEB_PORT
WEB_PUBLIC=true
WEB_PASSWORD="$PASSWORD"
WEB_GUEST_ENABLED=${WEB_GUEST_ENABLED:-false}
LOG_LEVEL=${LOG_LEVEL:-info}
UPDATE_CHECK_ENABLED=${UPDATE_CHECK_ENABLED:-true}
UPDATE_CHECK_INTERVAL=${UPDATE_CHECK_INTERVAL:-24h}
UPDATE_AUTO_STAGE=${UPDATE_AUTO_STAGE:-true}
EOF
    chown "$SERVICE_USER:$SERVICE_GROUP" "$INSTALL_DIR/.env"
    chmod 0600 "$INSTALL_DIR/.env"
else
    printf 'Preserving existing configuration: %s/.env\n' "$INSTALL_DIR"
fi
chown "$SERVICE_USER:$SERVICE_GROUP" "$INSTALL_DIR/.env"
chmod 0600 "$INSTALL_DIR/.env"
chown -R "$SERVICE_USER:$SERVICE_GROUP" "$INSTALL_DIR/data"

cat > "/etc/systemd/system/$SERVICE_NAME.service" <<EOF
[Unit]
Description=LLM API Uptime Monitor
Wants=network-online.target
After=network-online.target

[Service]
Type=simple
User=$SERVICE_USER
Group=$SERVICE_GROUP
WorkingDirectory=$INSTALL_DIR
EnvironmentFile=$INSTALL_DIR/.env
ExecStart=$INSTALL_DIR/llm-api-uptime --server
Restart=on-failure
RestartSec=5s
TimeoutStopSec=30s
UMask=0027
NoNewPrivileges=true
PrivateTmp=true
ProtectHome=true
ProtectSystem=full
ReadWritePaths=$INSTALL_DIR

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable "$SERVICE_NAME" >/dev/null
systemctl restart "$SERVICE_NAME"
if ! systemctl is-active --quiet "$SERVICE_NAME"; then
    systemctl status "$SERVICE_NAME" --no-pager || true
    die "service failed to start"
fi

printf '\nInstalled %s to %s\n' "$TAG" "$INSTALL_DIR"
printf 'Web UI: http://SERVER_IP:%s\n' "$WEB_PORT"
if [ -n "$GENERATED_PASSWORD" ]; then
    printf 'Generated WEB_PASSWORD: %s\n' "$GENERATED_PASSWORD"
    printf 'Store this password now; it is also in %s/.env.\n' "$INSTALL_DIR"
fi
printf 'Status: systemctl status %s\n' "$SERVICE_NAME"
printf 'Logs:   journalctl -u %s -f\n' "$SERVICE_NAME"
