#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUTPUT_DIR="${OUTPUT_DIR:-${ROOT_DIR}/dist/release}"
VERSION="${LLAMASITTER_VERSION:-}"
TARGET="${LLAMASITTER_TARGET:-}"
COMMIT="${LLAMASITTER_COMMIT:-$(git -C "${ROOT_DIR}" rev-parse --short HEAD 2>/dev/null || echo unknown)}"
DATE_VALUE="${LLAMASITTER_DATE:-$(date -u +%Y-%m-%dT%H:%M:%SZ)}"
DEFAULT_GOCACHE="${ROOT_DIR}/.gocache"
DEFAULT_GOMODCACHE="${ROOT_DIR}/.gomodcache"

export GOCACHE="${GOCACHE:-${DEFAULT_GOCACHE}}"
export GOMODCACHE="${GOMODCACHE:-${DEFAULT_GOMODCACHE}}"

usage() {
  cat <<'EOF'
Usage:
  bash ./scripts/package-release.sh package --version v0.1.0 --target linux/amd64 [--output-dir dist/release]
  bash ./scripts/package-release.sh checksums [--output-dir dist/release]

Commands:
  package    Build one release archive for the selected target
  checksums  Generate SHA256SUMS for all .tar.gz archives in the output directory
EOF
}

die() {
  printf '%s\n' "$*" >&2
  exit 1
}

require_arg() {
  local name="$1"
  local value="$2"
  if [[ -z "${value}" ]]; then
    die "missing required ${name}"
  fi
}

MODE=""

parse_args() {
  [[ $# -ge 1 ]] || { usage; exit 2; }
  MODE="$1"
  shift

  while [[ $# -gt 0 ]]; do
    case "$1" in
      --version)
        VERSION="$2"
        shift 2
        ;;
      --target)
        TARGET="$2"
        shift 2
        ;;
      --output-dir)
        OUTPUT_DIR="$2"
        shift 2
        ;;
      -h|--help)
        usage
        exit 0
        ;;
      *)
        die "unknown argument: $1"
        ;;
    esac
  done

}

build_ldflags() {
  printf '%s' "-X github.com/trevorashby/llamasitter/internal/buildinfo.Version=${VERSION} -X github.com/trevorashby/llamasitter/internal/buildinfo.Commit=${COMMIT} -X github.com/trevorashby/llamasitter/internal/buildinfo.Date=${DATE_VALUE}"
}

sha256_file() {
  local file="$1"
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "${file}"
  else
    shasum -a 256 "${file}"
  fi
}

package_linux() {
  local arch="$1"
  local stage_dir archive
  archive="llamasitter-linux-${arch}.tar.gz"
  stage_dir="$(mktemp -d)"

  GOOS=linux GOARCH="${arch}" go build -ldflags "$(build_ldflags)" -o "${stage_dir}/llamasitter" ./cmd/llamasitter
  cp "${ROOT_DIR}/LICENSE" "${stage_dir}/LICENSE"
  chmod +x "${stage_dir}/llamasitter"

  mkdir -p "${OUTPUT_DIR}"
  tar -C "${stage_dir}" -czf "${OUTPUT_DIR}/${archive}" llamasitter LICENSE
  rm -rf "${stage_dir}"
  printf 'Packaged %s\n' "${OUTPUT_DIR}/${archive}"
}

package_darwin() {
  local arch="$1"
  local stage_dir archive build_dir host_arch cli_arch

  host_arch="$(uname -m)"
  case "${host_arch}" in
    arm64) cli_arch="arm64" ;;
    x86_64) cli_arch="amd64" ;;
    *) die "unsupported macOS host architecture: ${host_arch}" ;;
  esac

  if [[ "${cli_arch}" != "${arch}" ]]; then
    die "darwin/${arch} packaging must run on a native ${arch} macOS runner"
  fi
  [[ "$(uname -s)" == "Darwin" ]] || die "darwin packaging must run on macOS"

  archive="llamasitter-darwin-${arch}.tar.gz"
  stage_dir="$(mktemp -d)"
  build_dir="${ROOT_DIR}/build/release-${arch}"

  rm -rf "${build_dir}"
  GO_LDFLAGS="$(build_ldflags)" BUILD_DIR="${build_dir}" bash "${ROOT_DIR}/scripts/build-macos-app.sh"
  GOOS=darwin GOARCH="${arch}" go build -ldflags "$(build_ldflags)" -o "${stage_dir}/llamasitter" ./cmd/llamasitter

  cp -R "${build_dir}/LlamaSitter.app" "${stage_dir}/LlamaSitter.app"
  cp "${ROOT_DIR}/LICENSE" "${stage_dir}/LICENSE"
  chmod +x "${stage_dir}/llamasitter"

  mkdir -p "${OUTPUT_DIR}"
  tar -C "${stage_dir}" -czf "${OUTPUT_DIR}/${archive}" LlamaSitter.app llamasitter LICENSE
  rm -rf "${stage_dir}"
  printf 'Packaged %s\n' "${OUTPUT_DIR}/${archive}"
}

generate_checksums() {
  mkdir -p "${OUTPUT_DIR}"
  local checksum_file="${OUTPUT_DIR}/SHA256SUMS"
  : > "${checksum_file}"

  local found=0
  local file
  for file in "${OUTPUT_DIR}"/*.tar.gz; do
    [[ -e "${file}" ]] || continue
    sha256_file "${file}" | awk -v name="$(basename "${file}")" '{print $1"  "name}' >> "${checksum_file}"
    found=1
  done

  [[ "${found}" -eq 1 ]] || die "no .tar.gz archives found in ${OUTPUT_DIR}"
  printf 'Wrote %s\n' "${checksum_file}"
}

package_target() {
  require_arg "--version" "${VERSION}"
  require_arg "--target" "${TARGET}"

  mkdir -p "${OUTPUT_DIR}" "${GOCACHE}" "${GOMODCACHE}"
  case "${TARGET}" in
    linux/amd64) package_linux amd64 ;;
    linux/arm64) package_linux arm64 ;;
    darwin/amd64) package_darwin amd64 ;;
    darwin/arm64) package_darwin arm64 ;;
    *) die "unsupported target: ${TARGET}" ;;
  esac
}

main() {
  parse_args "$@"
  case "${MODE}" in
    package) package_target ;;
    checksums) generate_checksums ;;
    *) usage; exit 2 ;;
  esac
}

main "$@"
