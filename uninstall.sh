#!/bin/sh
set -eu

say() {
  printf '%s\n' "$*"
}

warn() {
  printf 'warning: %s\n' "$*" >&2
}

die() {
  printf 'error: %s\n' "$*" >&2
  exit 1
}

detect_os() {
  if [ -n "${LLAMASITTER_OS:-}" ]; then
    printf '%s' "${LLAMASITTER_OS}"
    return
  fi

  case "$(uname -s)" in
    Darwin) printf '%s' "darwin" ;;
    Linux) printf '%s' "linux" ;;
    MINGW*|MSYS*|CYGWIN*|Windows_NT) printf '%s' "windows" ;;
    *) printf '%s' "unsupported" ;;
  esac
}

prompt_yes_no() {
  prompt="$1"
  default_answer="$2"

  if [ -n "${3:-}" ]; then
    case "$3" in
      1|true|TRUE|yes|YES|y|Y) return 0 ;;
      0|false|FALSE|no|NO|n|N) return 1 ;;
      *) die "invalid non-interactive answer: $3" ;;
    esac
  fi

  answer=""
  if [ -t 0 ]; then
    printf '%s ' "${prompt}"
    IFS= read -r answer || true
  elif [ -r /dev/tty ]; then
    printf '%s ' "${prompt}" >/dev/tty
    IFS= read -r answer </dev/tty || true
  else
    warn "no interactive terminal available for prompt: ${prompt}"
  fi
  if [ -z "${answer:-}" ]; then
    answer="${default_answer}"
  fi
  case "${answer}" in
    y|Y|yes|YES|true|TRUE|1) return 0 ;;
    *) return 1 ;;
  esac
}

path_needs_sudo() {
  target="$1"
  probe="$target"
  if [ -e "${probe}" ]; then
    [ ! -w "${probe}" ]
    return
  fi

  probe="$(dirname "${target}")"
  while [ ! -e "${probe}" ] && [ "${probe}" != "/" ]; do
    probe="$(dirname "${probe}")"
  done
  [ ! -w "${probe}" ]
}

setup_sudo() {
  if [ "${LLAMASITTER_FORCE_NO_SUDO:-0}" = "1" ] || [ "$(id -u)" -eq 0 ]; then
    SUDO_CMD=""
    return
  fi

  if path_needs_sudo "${BIN_DEST_PATH}" || { [ "${OS}" = "darwin" ] && path_needs_sudo "${APP_DEST_PATH}"; }; then
    command -v sudo >/dev/null 2>&1 || die "uninstall requires sudo for ${BIN_DEST_PATH} or ${APP_DEST_PATH}, but sudo is not available"
    SUDO_CMD="sudo"
  else
    SUDO_CMD=""
  fi
}

run_privileged() {
  if [ -n "${SUDO_CMD}" ]; then
    sudo "$@"
  else
    "$@"
  fi
}

stop_processes() {
  if [ "${LLAMASITTER_SKIP_STOP:-0}" = "1" ]; then
    return
  fi

  if command -v pkill >/dev/null 2>&1; then
    pkill -x LlamaSitter >/dev/null 2>&1 || true
    pkill -x LlamaSitterMenu >/dev/null 2>&1 || true
    pkill -x llamasitter-backend >/dev/null 2>&1 || true
  fi
}

OS="$(detect_os)"
case "${OS}" in
  darwin|linux) ;;
  windows) die "Windows is not supported by the uninstall script yet." ;;
  *) die "unsupported operating system: ${OS}" ;;
esac

BIN_DEST_DIR="${LLAMASITTER_BIN_DIR:-/usr/local/bin}"
BIN_DEST_PATH="${BIN_DEST_DIR}/llamasitter"
APP_DEST_DIR="${LLAMASITTER_APP_DIR:-/Applications}"
APP_DEST_PATH="${APP_DEST_DIR}/LlamaSitter.app"

if [ "${OS}" = "darwin" ]; then
  DATA_PATHS="${LLAMASITTER_APP_SUPPORT_DIR:-${HOME}/Library/Application Support/LlamaSitter}
${LLAMASITTER_LOG_DIR:-${HOME}/Library/Logs/LlamaSitter}"
else
  DATA_PATHS="${LLAMASITTER_CONFIG_DIR:-${HOME}/.config/llamasitter}
${LLAMASITTER_LEGACY_DATA_DIR:-${HOME}/.llamasitter}
${LLAMASITTER_STATE_DIR:-${HOME}/.local/state/llamasitter}"
fi

if ! prompt_yes_no "Remove installed LlamaSitter app/binary? [y/N]" "n" "${LLAMASITTER_YES:-}"; then
  say "Aborted."
  exit 0
fi

setup_sudo
stop_processes

removed_any=0
if [ "${OS}" = "darwin" ] && [ -e "${APP_DEST_PATH}" ]; then
  run_privileged rm -rf "${APP_DEST_PATH}"
  say "Removed ${APP_DEST_PATH}"
  removed_any=1
fi
if [ -e "${BIN_DEST_PATH}" ]; then
  run_privileged rm -f "${BIN_DEST_PATH}"
  say "Removed ${BIN_DEST_PATH}"
  removed_any=1
fi

if [ "${removed_any}" -eq 0 ]; then
  warn "no installed app or CLI was found in the default install locations"
fi

if prompt_yes_no "Also remove local LlamaSitter data and logs? [y/N]" "n" "${LLAMASITTER_PURGE_DATA:-}"; then
  printf '%s\n' "${DATA_PATHS}" | while IFS= read -r path; do
    [ -n "${path}" ] || continue
    if [ -e "${path}" ]; then
      rm -rf "${path}"
      say "Removed ${path}"
    fi
  done
else
  say "Preserved local data:"
  printf '%s\n' "${DATA_PATHS}" | while IFS= read -r path; do
    [ -n "${path}" ] || continue
    say "  ${path}"
  done
fi

say "LlamaSitter uninstall complete."
