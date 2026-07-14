#!/bin/sh
set -eu

INSTALL_DIR=${INSTALL_DIR:-/opt/llm-api-uptime}
SERVICE_NAME=${SERVICE_NAME:-llm-api-uptime}
SERVICE_USER=${SERVICE_USER:-llm-api-uptime}
SERVICE_GROUP=${SERVICE_GROUP:-llm-api-uptime}

die() {
    printf 'Error: %s\n' "$*" >&2
    exit 1
}

[ "$(uname -s)" = "Linux" ] || die "this uninstaller supports Linux only"
[ "$(id -u)" -eq 0 ] || die "run as root (for example: curl ... | sudo sh)"
case "$INSTALL_DIR" in
    /opt/*) ;;
    *) die "INSTALL_DIR must be under /opt" ;;
esac

if [ ! -r /dev/tty ] || [ ! -w /dev/tty ]; then
    die "an interactive terminal is required so data is never deleted by assumption"
fi

printf '%s\n' \
    'Uninstall LLM API Uptime:' \
    '  1) Remove service and binary; keep configuration and data' \
    '  2) Purge service, binary, configuration, database, and system user' \
    '  3) Cancel' > /dev/tty
printf 'Choose [1-3]: ' > /dev/tty
IFS= read -r CHOICE < /dev/tty

case "$CHOICE" in
    1|2) ;;
    3) printf 'Cancelled.\n'; exit 0 ;;
    *) die "invalid choice" ;;
esac

systemctl disable --now "$SERVICE_NAME" >/dev/null 2>&1 || true
rm -f "/etc/systemd/system/$SERVICE_NAME.service"
systemctl daemon-reload
systemctl reset-failed "$SERVICE_NAME" >/dev/null 2>&1 || true

if [ "$CHOICE" = "1" ]; then
    rm -f "$INSTALL_DIR/llm-api-uptime" "$INSTALL_DIR/llm-api-uptime.new"
    printf 'Service and binary removed. Configuration and data remain in %s.\n' "$INSTALL_DIR"
    exit 0
fi

rm -rf "$INSTALL_DIR"
if id "$SERVICE_USER" >/dev/null 2>&1; then
    userdel "$SERVICE_USER" >/dev/null 2>&1 || true
fi
if getent group "$SERVICE_GROUP" >/dev/null 2>&1; then
    groupdel "$SERVICE_GROUP" >/dev/null 2>&1 || true
fi
printf 'Service, application, configuration, data, and system user removed.\n'
