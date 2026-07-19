#!/bin/sh
set -e

install_cli_proxy_ca() {
    cert_file="${CLIPROXY_CA_CERT_FILE:-}"
    if [ -z "${cert_file}" ]; then
        return 0
    fi

    # 只有本地集成显式提供 CA 时才更新信任链，避免改变生产默认镜像行为。
    if [ ! -r "${cert_file}" ] || ! grep -q "BEGIN CERTIFICATE" "${cert_file}" || ! grep -q "END CERTIFICATE" "${cert_file}"; then
        echo "CLIPROXY_CA_CERT_FILE is not a readable PEM certificate: ${cert_file}" >&2
        exit 1
    fi

    install -m 0644 "${cert_file}" /usr/local/share/ca-certificates/cliproxy-local-ca.crt
    update-ca-certificates >/dev/null
}

# Fix data directory permissions when running as root.
# Docker named volumes / host bind-mounts may be owned by root,
# preventing the non-root sub2api user from writing files.
if [ "$(id -u)" = "0" ]; then
    install_cli_proxy_ca
    mkdir -p /app/data
    # Use || true to avoid failure on read-only mounted files (e.g. config.yaml:ro)
    chown -R sub2api:sub2api /app/data 2>/dev/null || true
    # Re-invoke this script as sub2api so the flag-detection below
    # also runs under the correct user.
    exec su-exec sub2api "$0" "$@"
fi

# Compatibility: if the first arg looks like a flag (e.g. --help),
# prepend the default binary so it behaves the same as the old
# ENTRYPOINT ["/app/sub2api"] style.
if [ "${1#-}" != "$1" ]; then
    set -- /app/sub2api "$@"
fi

exec "$@"
