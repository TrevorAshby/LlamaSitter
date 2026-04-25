#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BUILD_DIR="${BUILD_DIR:-${ROOT_DIR}/build/linux-desktop-package}"
STAGE_DIR="${STAGE_DIR:-${BUILD_DIR}/stage}"
OUTPUT_DIR="${OUTPUT_DIR:-${ROOT_DIR}/dist/linux-desktop}"
VERSION="${VERSION:-${LLAMASITTER_VERSION:-0.1.0}}"
ARCH="${ARCH:-${NFPM_ARCH:-$(uname -m)}}"
NFPM_CONFIG="${ROOT_DIR}/packaging/linux/nfpm.yaml"

normalize_arch() {
  case "$1" in
    x86_64|amd64) printf '%s' "amd64" ;;
    arm64|aarch64) printf '%s' "arm64" ;;
    *) printf '%s' "$1" ;;
  esac
}

normalize_version() {
  case "$1" in
    v*) printf '%s' "${1#v}" ;;
    *) printf '%s' "$1" ;;
  esac
}

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || {
    printf 'missing required command: %s\n' "$1" >&2
    exit 1
  }
}

require_cmd nfpm

ARCH="$(normalize_arch "${ARCH}")"
VERSION="$(normalize_version "${VERSION}")"
mkdir -p "${OUTPUT_DIR}"

BUILD_DIR="${BUILD_DIR}" STAGE_DIR="${STAGE_DIR}" bash "${ROOT_DIR}/scripts/build-linux-app.sh"

for packager in deb rpm; do
  case "${packager}" in
    deb) target="${OUTPUT_DIR}/llamasitter-desktop_${VERSION}_${ARCH}.deb" ;;
    rpm) target="${OUTPUT_DIR}/llamasitter-desktop-${VERSION}-1.${ARCH}.rpm" ;;
  esac
  NFPM_ARCH="${ARCH}" \
  VERSION="${VERSION}" \
  STAGE_DIR="${STAGE_DIR}" \
  nfpm package \
    --config "${NFPM_CONFIG}" \
    --packager "${packager}" \
    --target "${target}"
done

echo
echo "Packaged Linux desktop artifacts in:"
echo "${OUTPUT_DIR}"
