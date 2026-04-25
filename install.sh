#!/bin/sh
set -eu

RELEASE_BASE_ROOT_DEFAULT="https://github.com/trevorashby/llamasitter/releases"
TMP_DIR="$(mktemp -d)"

cleanup() {
  rm -rf "${TMP_DIR}"
}
trap cleanup EXIT INT TERM

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

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || die "missing required command: $1"
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

detect_arch() {
  if [ -n "${LLAMASITTER_ARCH:-}" ]; then
    printf '%s' "${LLAMASITTER_ARCH}"
    return
  fi

  case "$(uname -m)" in
    x86_64|amd64) printf '%s' "amd64" ;;
    arm64|aarch64) printf '%s' "arm64" ;;
    *) printf '%s' "unsupported" ;;
  esac
}

normalize_tag() {
  case "$1" in
    v*) printf '%s' "$1" ;;
    *) printf 'v%s' "$1" ;;
  esac
}

checksum_line_for() {
  asset="$1"
  checksum_file="$2"
  awk -v target="${asset}" '$2 == target { print $1 }' "${checksum_file}"
}

sha256_of() {
  file="$1"
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "${file}" | awk '{print $1}'
  else
    shasum -a 256 "${file}" | awk '{print $1}'
  fi
}

download() {
  url="$1"
  out="$2"
  curl -fsSL "${url}" -o "${out}"
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

  if path_needs_sudo "${BIN_DEST_DIR}" || { [ "${OS}" = "darwin" ] && path_needs_sudo "${APP_DEST_DIR}"; }; then
    command -v sudo >/dev/null 2>&1 || die "installation requires sudo for ${BIN_DEST_DIR} or ${APP_DEST_DIR}, but sudo is not available"
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

stop_existing_macos_processes() {
  if [ "${LLAMASITTER_SKIP_STOP:-0}" = "1" ]; then
    return
  fi

  if command -v pkill >/dev/null 2>&1; then
    pkill -x LlamaSitter >/dev/null 2>&1 || true
    pkill -x LlamaSitterMenu >/dev/null 2>&1 || true
    pkill -x llamasitter-backend >/dev/null 2>&1 || true
  fi
}

verify_binary() {
  if ! "${BIN_DEST_DIR}/llamasitter" version >/dev/null 2>&1; then
    die "installed CLI failed to execute: ${BIN_DEST_DIR}/llamasitter"
  fi
}

print_path_notice() {
  if command -v llamasitter >/dev/null 2>&1; then
    say "CLI available at: $(command -v llamasitter)"
  else
    warn "installed ${BIN_DEST_DIR}/llamasitter but it is not on your current PATH"
  fi
}

OS="$(detect_os)"
ARCH="$(detect_arch)"

case "${OS}" in
  darwin|linux) ;;
  windows) die "Windows is not supported by the one-line installer yet. Build LlamaSitter manually from source." ;;
  *) die "unsupported operating system: ${OS}" ;;
esac

case "${ARCH}" in
  amd64|arm64) ;;
  *) die "unsupported architecture: ${ARCH}" ;;
esac

require_cmd curl
require_cmd tar
require_cmd mktemp
require_cmd dirname
if ! command -v sha256sum >/dev/null 2>&1 && ! command -v shasum >/dev/null 2>&1; then
  die "missing required checksum tool: sha256sum or shasum"
fi

ASSET="llamasitter-${OS}-${ARCH}.tar.gz"
RELEASE_BASE_ROOT="${LLAMASITTER_RELEASE_BASE_ROOT:-${RELEASE_BASE_ROOT_DEFAULT}}"
if [ -n "${LLAMASITTER_VERSION:-}" ]; then
  TAG="$(normalize_tag "${LLAMASITTER_VERSION}")"
  DOWNLOAD_BASE="${RELEASE_BASE_ROOT}/download/${TAG}"
else
  DOWNLOAD_BASE="${RELEASE_BASE_ROOT}/latest/download"
fi

ARCHIVE_PATH="${TMP_DIR}/${ASSET}"
CHECKSUM_PATH="${TMP_DIR}/SHA256SUMS"
EXTRACT_DIR="${TMP_DIR}/extract"
mkdir -p "${EXTRACT_DIR}"

say "Downloading ${ASSET}..."
download "${DOWNLOAD_BASE}/${ASSET}" "${ARCHIVE_PATH}"
download "${DOWNLOAD_BASE}/SHA256SUMS" "${CHECKSUM_PATH}"

EXPECTED_SUM="$(checksum_line_for "${ASSET}" "${CHECKSUM_PATH}")"
[ -n "${EXPECTED_SUM}" ] || die "unable to find checksum entry for ${ASSET}"
ACTUAL_SUM="$(sha256_of "${ARCHIVE_PATH}")"
[ "${EXPECTED_SUM}" = "${ACTUAL_SUM}" ] || die "checksum verification failed for ${ASSET}"
say "Checksum verified."

tar -xzf "${ARCHIVE_PATH}" -C "${EXTRACT_DIR}"

BIN_DEST_DIR="${LLAMASITTER_BIN_DIR:-/usr/local/bin}"
APP_DEST_DIR="${LLAMASITTER_APP_DIR:-/Applications}"
setup_sudo

if [ "${OS}" = "darwin" ]; then
  [ -d "${EXTRACT_DIR}/LlamaSitter.app" ] || die "archive did not contain LlamaSitter.app"
fi
[ -f "${EXTRACT_DIR}/llamasitter" ] || die "archive did not contain the llamasitter CLI"

if [ "${OS}" = "darwin" ]; then
  say "Installing macOS app to ${APP_DEST_DIR}/LlamaSitter.app..."
  stop_existing_macos_processes
  run_privileged mkdir -p "${APP_DEST_DIR}"
  run_privileged rm -rf "${APP_DEST_DIR}/LlamaSitter.app"
  run_privileged cp -R "${EXTRACT_DIR}/LlamaSitter.app" "${APP_DEST_DIR}/LlamaSitter.app"
fi

say "Installing CLI to ${BIN_DEST_DIR}/llamasitter..."
run_privileged mkdir -p "${BIN_DEST_DIR}"
run_privileged install -m 0755 "${EXTRACT_DIR}/llamasitter" "${BIN_DEST_DIR}/llamasitter"

verify_binary
print_path_notice

if [ "${OS}" = "darwin" ]; then
  say "Installed LlamaSitter.app to ${APP_DEST_DIR}/LlamaSitter.app"
  if [ "${LLAMASITTER_NO_LAUNCH:-0}" != "1" ]; then
    if command -v open >/dev/null 2>&1 && [ "${APP_DEST_DIR}" = "/Applications" ]; then
      open -a LlamaSitter >/dev/null 2>&1 || warn "installed the app but could not launch it automatically"
    else
      warn "installed the app but skipped automatic launch because the app path is not /Applications"
    fi
  fi
else
  say "LlamaSitter CLI installed successfully."
  say "The Linux desktop shell is distributed separately as native .deb and .rpm packages."
  say "Next steps:"
  say "  llamasitter config init"
  say "  llamasitter serve --config llamasitter.yaml"
fi
