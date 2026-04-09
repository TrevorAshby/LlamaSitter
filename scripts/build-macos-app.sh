#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BUILD_DIR="${ROOT_DIR}/build/macos"
APP_NAME="LlamaSitter"
APP_BUNDLE="${BUILD_DIR}/${APP_NAME}.app"
APP_CONTENTS="${APP_BUNDLE}/Contents"
APP_MACOS="${APP_CONTENTS}/MacOS"
APP_RESOURCES="${APP_CONTENTS}/Resources"
APP_LIBRARY="${APP_CONTENTS}/Library"
APP_LOGIN_ITEMS="${APP_LIBRARY}/LoginItems"
MENU_AGENT_BUNDLE="${APP_LOGIN_ITEMS}/LlamaSitterMenu.app"
MENU_AGENT_CONTENTS="${MENU_AGENT_BUNDLE}/Contents"
MENU_AGENT_MACOS="${MENU_AGENT_CONTENTS}/MacOS"
MENU_AGENT_RESOURCES="${MENU_AGENT_CONTENTS}/Resources"
DESKTOP_BINARY="${BUILD_DIR}/${APP_NAME}"
BACKEND_BINARY="${BUILD_DIR}/llamasitter-backend"
ICONSET_DIR="${BUILD_DIR}/AppIcon.iconset"
ICON_FILE="${BUILD_DIR}/AppIcon.icns"
ICON_SOURCE_PNG="${BUILD_DIR}/AppIcon-1024.png"
ICON_GENERATOR="${BUILD_DIR}/generate-placeholder-icon"
STATUS_ICON_FILE="${BUILD_DIR}/MenuBarIcon.png"
STATUS_ICON_GENERATOR="${BUILD_DIR}/generate-status-item-icon"
SYSTEM_PLACEHOLDER_ICON="/System/Library/CoreServices/CoreTypes.bundle/Contents/Resources/GenericApplicationIcon.icns"
DEFAULT_GOCACHE="${ROOT_DIR}/.gocache"
DEFAULT_GOMODCACHE="${ROOT_DIR}/.gomodcache"
SDKROOT="$(xcrun --show-sdk-path --sdk macosx)"
CLANG="$(xcrun --find clang)"
MIN_VERSION="13.0"

export GOCACHE="${GOCACHE:-${DEFAULT_GOCACHE}}"
export GOMODCACHE="${GOMODCACHE:-${DEFAULT_GOMODCACHE}}"

mkdir -p "${BUILD_DIR}"
mkdir -p "${GOCACHE}" "${GOMODCACHE}"
rm -rf "${APP_BUNDLE}" "${ICONSET_DIR}" "${ICON_FILE}" "${ICON_SOURCE_PNG}" "${STATUS_ICON_FILE}" "${DESKTOP_BINARY}" "${BACKEND_BINARY}" "${ICON_GENERATOR}" "${STATUS_ICON_GENERATOR}"

render_icon() {
  local size="$1"
  local filename="$2"
  sips -z "${size}" "${size}" "${ICON_SOURCE_PNG}" --out "${ICONSET_DIR}/${filename}" >/dev/null
}

echo "Building Go backend..."
go build -o "${BACKEND_BINARY}" ./cmd/llamasitter

echo "Generating placeholder dock icon..."
"${CLANG}" \
  -fobjc-arc \
  -fblocks \
  -isysroot "${SDKROOT}" \
  -mmacosx-version-min="${MIN_VERSION}" \
  -o "${ICON_GENERATOR}" \
  "${ROOT_DIR}/desktop/macos/Tools/generate_placeholder_icon.m" \
  -framework Cocoa
"${ICON_GENERATOR}" "${ICON_SOURCE_PNG}"
mkdir -p "${ICONSET_DIR}"
render_icon 16 "icon_16x16.png"
render_icon 32 "icon_16x16@2x.png"
render_icon 32 "icon_32x32.png"
render_icon 64 "icon_32x32@2x.png"
render_icon 128 "icon_128x128.png"
render_icon 256 "icon_128x128@2x.png"
render_icon 256 "icon_256x256.png"
render_icon 512 "icon_256x256@2x.png"
render_icon 512 "icon_512x512.png"
cp "${ICON_SOURCE_PNG}" "${ICONSET_DIR}/icon_512x512@2x.png"
if ! iconutil --convert icns "${ICONSET_DIR}" --output "${ICON_FILE}"; then
  echo "Falling back to the system placeholder app icon..."
  cp "${SYSTEM_PLACEHOLDER_ICON}" "${ICON_FILE}"
fi

echo "Generating menu bar icon..."
"${CLANG}" \
  -fobjc-arc \
  -fblocks \
  -isysroot "${SDKROOT}" \
  -mmacosx-version-min="${MIN_VERSION}" \
  -o "${STATUS_ICON_GENERATOR}" \
  "${ROOT_DIR}/desktop/macos/Tools/generate_status_item_icon.m" \
  -framework Cocoa
"${STATUS_ICON_GENERATOR}" "${STATUS_ICON_FILE}"

echo "Compiling macOS desktop shell..."
"${CLANG}" \
  -fobjc-arc \
  -fblocks \
  -isysroot "${SDKROOT}" \
  -mmacosx-version-min="${MIN_VERSION}" \
  -I "${ROOT_DIR}/desktop/macos/Sources" \
  -o "${DESKTOP_BINARY}" \
  "${ROOT_DIR}"/desktop/macos/Sources/*.m \
  -framework Cocoa \
  -framework WebKit

echo "Assembling app bundle..."
mkdir -p "${APP_MACOS}" "${APP_RESOURCES}" "${MENU_AGENT_MACOS}" "${MENU_AGENT_RESOURCES}"
cp "${ROOT_DIR}/desktop/macos/Resources/Info.plist" "${APP_CONTENTS}/Info.plist"
printf 'APPL????' > "${APP_CONTENTS}/PkgInfo"
cp "${DESKTOP_BINARY}" "${APP_MACOS}/${APP_NAME}"
cp "${BACKEND_BINARY}" "${APP_RESOURCES}/llamasitter-backend"
cp "${ICON_FILE}" "${APP_RESOURCES}/AppIcon.icns"
cp "${STATUS_ICON_FILE}" "${APP_RESOURCES}/MenuBarIcon.png"
chmod +x "${APP_MACOS}/${APP_NAME}" "${APP_RESOURCES}/llamasitter-backend"

cp "${ROOT_DIR}/desktop/macos/Resources/MenuAgentInfo.plist" "${MENU_AGENT_CONTENTS}/Info.plist"
printf 'APPL????' > "${MENU_AGENT_CONTENTS}/PkgInfo"
cp "${DESKTOP_BINARY}" "${MENU_AGENT_MACOS}/LlamaSitterMenu"
cp "${BACKEND_BINARY}" "${MENU_AGENT_RESOURCES}/llamasitter-backend"
cp "${STATUS_ICON_FILE}" "${MENU_AGENT_RESOURCES}/MenuBarIcon.png"
chmod +x "${MENU_AGENT_MACOS}/LlamaSitterMenu" "${MENU_AGENT_RESOURCES}/llamasitter-backend"

if command -v codesign >/dev/null 2>&1; then
  codesign --force --deep --sign - "${APP_BUNDLE}" >/dev/null 2>&1 || true
fi

echo
echo "Built app bundle:"
echo "${APP_BUNDLE}"
echo
echo "Launch with:"
echo "open \"${APP_BUNDLE}\""
