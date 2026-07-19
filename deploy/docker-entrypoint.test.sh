#!/usr/bin/env bash
# docker-entrypoint.sh 的 CA 注入回归测试；不触碰真实服务或镜像。

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPT_UNDER_TEST="${SCRIPT_DIR}/docker-entrypoint.sh"
DOCKERFILE_UNDER_TEST="${SCRIPT_DIR}/../Dockerfile"

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

assert_contains() {
  local haystack="$1"
  local needle="$2"
  [[ "${haystack}" == *"${needle}"* ]] || fail "expected output to contain: ${needle}"
}

assert_not_contains() {
  local haystack="$1"
  local needle="$2"
  [[ "${haystack}" != *"${needle}"* ]] || fail "expected output not to contain: ${needle}"
}

make_mocks() {
  local mock_dir="$1"
  cat > "${mock_dir}/id" <<'MOCK'
#!/usr/bin/env bash
echo 0
MOCK
  cat > "${mock_dir}/install" <<'MOCK'
#!/usr/bin/env bash
printf 'install %s\n' "$*" >> "${MOCK_LOG}"
exit 0
MOCK
  cat > "${mock_dir}/update-ca-certificates" <<'MOCK'
#!/usr/bin/env bash
printf 'update-ca-certificates\n' >> "${MOCK_LOG}"
exit 0
MOCK
  cat > "${mock_dir}/su-exec" <<'MOCK'
#!/usr/bin/env bash
printf 'su-exec %s\n' "$*" >> "${MOCK_LOG}"
exit 0
MOCK
  cat > "${mock_dir}/mkdir" <<'MOCK'
#!/usr/bin/env bash
exit 0
MOCK
  cat > "${mock_dir}/chown" <<'MOCK'
#!/usr/bin/env bash
exit 0
MOCK
  chmod +x "${mock_dir}"/*
}

test_imports_ca_when_configured() {
  local temp_dir mock_dir log_file calls
  temp_dir="$(mktemp -d)"
  trap 'rm -rf "${temp_dir}"' RETURN
  mock_dir="${temp_dir}/mocks"
  log_file="${temp_dir}/calls.log"
  mkdir -p "${mock_dir}"
  make_mocks "${mock_dir}"
  printf '%s\n' '-----BEGIN CERTIFICATE-----' 'test' '-----END CERTIFICATE-----' > "${temp_dir}/ca.crt"

  PATH="${mock_dir}:${PATH}" MOCK_LOG="${log_file}" CLIPROXY_CA_CERT_FILE="${temp_dir}/ca.crt" \
    bash "${SCRIPT_UNDER_TEST}" /bin/true

  calls="$(cat "${log_file}")"
  assert_contains "${calls}" "install"
  assert_contains "${calls}" "update-ca-certificates"
}

test_skips_ca_when_unconfigured() {
  local temp_dir mock_dir log_file calls
  temp_dir="$(mktemp -d)"
  trap 'rm -rf "${temp_dir}"' RETURN
  mock_dir="${temp_dir}/mocks"
  log_file="${temp_dir}/calls.log"
  mkdir -p "${mock_dir}"
  make_mocks "${mock_dir}"

  PATH="${mock_dir}:${PATH}" MOCK_LOG="${log_file}" CLIPROXY_CA_CERT_FILE= \
    bash "${SCRIPT_UNDER_TEST}" /bin/true

  calls="$(cat "${log_file}")"
  assert_not_contains "${calls}" "update-ca-certificates"
}

test_rejects_missing_or_malformed_ca() {
  local temp_dir mock_dir log_file
  temp_dir="$(mktemp -d)"
  trap 'rm -rf "${temp_dir}"' RETURN
  mock_dir="${temp_dir}/mocks"
  log_file="${temp_dir}/calls.log"
  mkdir -p "${mock_dir}"
  make_mocks "${mock_dir}"
  printf '%s\n' 'not-a-certificate' > "${temp_dir}/bad.crt"

  if PATH="${mock_dir}:${PATH}" MOCK_LOG="${log_file}" CLIPROXY_CA_CERT_FILE="${temp_dir}/missing.crt" \
    bash "${SCRIPT_UNDER_TEST}" /bin/true; then
    fail "missing CA certificate should fail"
  fi
  if PATH="${mock_dir}:${PATH}" MOCK_LOG="${log_file}" CLIPROXY_CA_CERT_FILE="${temp_dir}/bad.crt" \
    bash "${SCRIPT_UNDER_TEST}" /bin/true; then
    fail "malformed CA certificate should fail"
  fi
}

test_dockerfile_registers_legacy_ca() {
  local dockerfile
  dockerfile="$(cat "${DOCKERFILE_UNDER_TEST}")"
  assert_contains "${dockerfile}" "COPY backend/resources/certs/tls.crt /usr/local/share/ca-certificates/cliproxy-legacy-ca.crt"
  assert_contains "${dockerfile}" "RUN update-ca-certificates"
  assert_not_contains "${dockerfile}" "cat /tmp/cli-proxy-ca.crt >> /etc/ssl/certs/ca-certificates.crt"
}

test_imports_ca_when_configured
test_skips_ca_when_unconfigured
test_rejects_missing_or_malformed_ca
test_dockerfile_registers_legacy_ca
