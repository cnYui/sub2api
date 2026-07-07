#!/usr/bin/env bash

deploy_log() {
  printf '[INFO] %s\n' "$*"
}

deploy_warn() {
  printf '[WARN] %s\n' "$*" >&2
}

deploy_die() {
  printf '[ERROR] %s\n' "$*" >&2
  return 1
}

deploy_format_cmd() {
  local arg rendered quoted
  rendered=""
  for arg in "$@"; do
    printf -v quoted '%q' "${arg}"
    if [[ -z "${rendered}" ]]; then
      rendered="${quoted}"
    else
      rendered="${rendered} ${quoted}"
    fi
  done
  printf '%s' "${rendered}"
}

deploy_run_cmd() {
  if [[ "${DRY_RUN:-false}" == true ]]; then
    printf '[DRY-RUN] %s\n' "$(deploy_format_cmd "$@")"
    return 0
  fi

  printf '+ %s\n' "$(deploy_format_cmd "$@")"
  "$@"
}

deploy_repo_root() {
  local source_dir
  source_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
  printf '%s\n' "${source_dir}"
}
