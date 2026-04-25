#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BUILD_DIR="${BUILD_DIR:-${ROOT_DIR}/build/linux-app}"
STAGE_DIR="${STAGE_DIR:-${BUILD_DIR}/stage}"
OBJ_DIR="${BUILD_DIR}/obj"
BIN_DIR="${STAGE_DIR}/usr/bin"
APPLICATIONS_DIR="${STAGE_DIR}/usr/share/applications"
ICON_DIR="${STAGE_DIR}/usr/share/icons/hicolor/1024x1024/apps"
RESOURCE_DIR="${ROOT_DIR}/desktop/linux/resources"
ICON_SOURCE="${ROOT_DIR}/desktop/macos/Resources/AppIcon-1024.png"
CC_BIN="${CC:-cc}"
DEFAULT_GOCACHE="${ROOT_DIR}/.gocache"
DEFAULT_GOMODCACHE="${ROOT_DIR}/.gomodcache"

export GOCACHE="${GOCACHE:-${DEFAULT_GOCACHE}}"
export GOMODCACHE="${GOMODCACHE:-${DEFAULT_GOMODCACHE}}"

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || {
    printf 'missing required command: %s\n' "$1" >&2
    exit 1
  }
}

pkg_config_exists() {
  pkg-config --exists "$1"
}

require_cmd "${CC_BIN}"
require_cmd pkg-config
require_cmd go

mkdir -p "${BUILD_DIR}" "${OBJ_DIR}" "${BIN_DIR}" "${APPLICATIONS_DIR}" "${ICON_DIR}" "${GOCACHE}" "${GOMODCACHE}"

COMMON_PACKAGES=(glib-2.0 gio-2.0 json-glib-1.0 libsoup-3.0)
DASH_PACKAGES=(gtk4 libadwaita-1)
TRAY_PACKAGES=(gtk+-3.0)
APPINDICATOR_PACKAGE=""
WEBKIT_PACKAGE=""
WEBKIT_DEFINE=""
APPINDICATOR_DEFINE=""

if pkg_config_exists webkitgtk-6.0; then
  WEBKIT_PACKAGE="webkitgtk-6.0"
  WEBKIT_DEFINE="-DLS_USE_WEBKITGTK_6=1"
elif pkg_config_exists webkit2gtk-4.1; then
  WEBKIT_PACKAGE="webkit2gtk-4.1"
else
  printf 'missing required WebKitGTK package: webkitgtk-6.0 or webkit2gtk-4.1\n' >&2
  exit 1
fi

if pkg_config_exists ayatana-appindicator3-0.1; then
  APPINDICATOR_PACKAGE="ayatana-appindicator3-0.1"
  APPINDICATOR_DEFINE="-DLS_HAVE_APPINDICATOR=1 -DLS_USE_AYATANA_APPINDICATOR=1"
elif pkg_config_exists appindicator3-0.1; then
  APPINDICATOR_PACKAGE="appindicator3-0.1"
  APPINDICATOR_DEFINE="-DLS_HAVE_APPINDICATOR=1"
fi

COMMON_CFLAGS="$(pkg-config --cflags "${COMMON_PACKAGES[@]}")"
COMMON_LIBS="$(pkg-config --libs "${COMMON_PACKAGES[@]}")"
DASH_CFLAGS="$(pkg-config --cflags "${DASH_PACKAGES[@]}" "${WEBKIT_PACKAGE}") ${WEBKIT_DEFINE}"
DASH_LIBS="$(pkg-config --libs "${DASH_PACKAGES[@]}" "${WEBKIT_PACKAGE}")"

if [[ -n "${APPINDICATOR_PACKAGE}" ]]; then
  TRAY_CFLAGS="$(pkg-config --cflags "${TRAY_PACKAGES[@]}" "${APPINDICATOR_PACKAGE}") ${APPINDICATOR_DEFINE}"
  TRAY_LIBS="$(pkg-config --libs "${TRAY_PACKAGES[@]}" "${APPINDICATOR_PACKAGE}")"
else
  printf 'warning: AppIndicator package not found; building tray fallback without a native indicator\n' >&2
  TRAY_CFLAGS="$(pkg-config --cflags "${TRAY_PACKAGES[@]}")"
  TRAY_LIBS="$(pkg-config --libs "${TRAY_PACKAGES[@]}")"
fi

echo "Building LlamaSitter CLI..."
GOOS=linux GOARCH="${GOARCH:-$(go env GOARCH)}" go build -o "${BIN_DIR}/llamasitter" ./cmd/llamasitter

echo "Compiling Linux desktop shell..."
"${CC_BIN}" -std=c11 -O2 -Wall -Wextra ${COMMON_CFLAGS} -c "${ROOT_DIR}/desktop/linux/common.c" -o "${OBJ_DIR}/common.o"
"${CC_BIN}" -std=c11 -O2 -Wall -Wextra ${COMMON_CFLAGS} -c "${ROOT_DIR}/desktop/linux/main.c" -o "${OBJ_DIR}/main.o"
"${CC_BIN}" -std=c11 -O2 -Wall -Wextra ${COMMON_CFLAGS} ${DASH_CFLAGS} -c "${ROOT_DIR}/desktop/linux/dashboard.c" -o "${OBJ_DIR}/dashboard.o"
"${CC_BIN}" -std=c11 -O2 -Wall -Wextra ${COMMON_CFLAGS} ${TRAY_CFLAGS} -c "${ROOT_DIR}/desktop/linux/tray.c" -o "${OBJ_DIR}/tray.o"
"${CC_BIN}" -o "${BIN_DIR}/llamasitter-desktop" \
  "${OBJ_DIR}/main.o" \
  "${OBJ_DIR}/common.o" \
  "${OBJ_DIR}/dashboard.o" \
  "${OBJ_DIR}/tray.o" \
  ${COMMON_LIBS} \
  ${DASH_LIBS} \
  ${TRAY_LIBS}

echo "Staging desktop resources..."
install -m 0644 "${RESOURCE_DIR}/llamasitter.desktop" "${APPLICATIONS_DIR}/llamasitter.desktop"
install -m 0644 "${RESOURCE_DIR}/llamasitter-tray.desktop" "${APPLICATIONS_DIR}/llamasitter-tray.desktop"
install -m 0644 "${ICON_SOURCE}" "${ICON_DIR}/llamasitter.png"

echo
echo "Staged Linux desktop app under:"
echo "${STAGE_DIR}"
