#!/bin/bash
# build-pkg.sh — Build a signed, notarized Valet installer package.
#
# Usage:
#   ./scripts/build-pkg.sh                    # local build (certs from Keychain)
#   ./scripts/build-pkg.sh --no-notarize      # sign but skip notarization
#
# Environment variables (CI):
#   APPLE_APP_IDENTITY       "Developer ID Application: ..." (codesign)
#   APPLE_INSTALLER_IDENTITY "Developer ID Installer: ..."   (productsign)
#   APPLE_TEAM_ID            Team ID for notarytool
#   APPLE_ID                 Apple ID email for notarytool
#   APPLE_NOTARY_PASSWORD    App-specific password for notarytool
#   VERSION                  Semver string (default: 0.1.0)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

VERSION="${VERSION:-0.1.0}"
APPLE_APP_IDENTITY="${APPLE_APP_IDENTITY:-Developer ID Application: Richard Clayton (87HG9537ZK)}"
APPLE_INSTALLER_IDENTITY="${APPLE_INSTALLER_IDENTITY:-Developer ID Installer: Richard Clayton (87HG9537ZK)}"
NOTARIZE=true
ENTITLEMENTS="${SCRIPT_DIR}/pkg/entitlements.plist"

for arg in "$@"; do
    case $arg in
        --no-notarize) NOTARIZE=false ;;
    esac
done

APP_DIR="${ROOT_DIR}/valetapp/build/bin"
APP_BUNDLE="${APP_DIR}/Valet.app"
PKG_ROOT="${ROOT_DIR}/build/pkg-root"
PKG_SCRIPTS="${ROOT_DIR}/build/pkg-scripts"
COMPONENT_PKG="${ROOT_DIR}/build/Valet-component.pkg"
OUTPUT_PKG="${ROOT_DIR}/build/Valet-${VERSION}.pkg"

echo "==> Building Valet ${VERSION}"

# ---- Step 1: Build everything ----
echo "--- Building Wails app"
(cd "${ROOT_DIR}/valetapp" && wails build)

echo "--- Building valetd"
(cd "${ROOT_DIR}/valetd" && go build -o "${APP_BUNDLE}/Contents/MacOS/valetd" ./cmd/valetd)

echo "--- Building valet CLI"
# CLI goes in Resources/ to avoid case collision with the Wails binary (Valet vs valet)
# on macOS's case-insensitive filesystem. postinstall symlinks it to /usr/local/bin/valet.
mkdir -p "${APP_BUNDLE}/Contents/Resources/bin"
(cd "${ROOT_DIR}/valetd" && go build -o "${APP_BUNDLE}/Contents/Resources/bin/valet" ./cmd/valet)

# Copy LaunchAgent plist into app bundle Resources (postinstall copies it out)
mkdir -p "${APP_BUNDLE}/Contents/Resources"
cp "${SCRIPT_DIR}/pkg/run.loa.valetd.plist" "${APP_BUNDLE}/Contents/Resources/"

# ---- Step 2: Sign binaries (inside-out) ----
echo "--- Signing binaries"

# Sign each embedded binary individually first
codesign --force --timestamp --options=runtime \
    --entitlements "${ENTITLEMENTS}" \
    -s "${APPLE_APP_IDENTITY}" \
    "${APP_BUNDLE}/Contents/MacOS/valetd"

codesign --force --timestamp --options=runtime \
    --entitlements "${ENTITLEMENTS}" \
    -s "${APPLE_APP_IDENTITY}" \
    "${APP_BUNDLE}/Contents/Resources/bin/valet"

# Sign the main Wails binary
codesign --force --timestamp --options=runtime \
    --entitlements "${ENTITLEMENTS}" \
    -s "${APPLE_APP_IDENTITY}" \
    "${APP_BUNDLE}/Contents/MacOS/Valet"

# Sign the overall .app bundle
codesign --force --timestamp --options=runtime \
    --entitlements "${ENTITLEMENTS}" \
    -s "${APPLE_APP_IDENTITY}" \
    "${APP_BUNDLE}"

# Verify
echo "--- Verifying signature"
codesign --verify --deep --strict "${APP_BUNDLE}"
echo "    Signature OK"

# ---- Step 3: Build .pkg ----
echo "--- Building installer package"
rm -rf "${PKG_ROOT}" "${PKG_SCRIPTS}" "${COMPONENT_PKG}" "${OUTPUT_PKG}"
mkdir -p "${PKG_ROOT}/Applications" "${PKG_SCRIPTS}" "${ROOT_DIR}/build"

# Stage the signed app into the pkg root
cp -R "${APP_BUNDLE}" "${PKG_ROOT}/Applications/"

# Stage installer scripts
cp "${SCRIPT_DIR}/pkg/preinstall" "${PKG_SCRIPTS}/"
cp "${SCRIPT_DIR}/pkg/postinstall" "${PKG_SCRIPTS}/"

# Build the component package
pkgbuild \
    --root "${PKG_ROOT}" \
    --scripts "${PKG_SCRIPTS}" \
    --identifier "run.loa.valet" \
    --version "${VERSION}" \
    --install-location "/" \
    "${COMPONENT_PKG}"

# Build the distribution XML from template
DIST_XML="${ROOT_DIR}/build/distribution.xml"
INSTALL_KB=$(du -sk "${PKG_ROOT}" | cut -f1)
sed -e "s/__VERSION__/${VERSION}/g" -e "s/__SIZE__/${INSTALL_KB}/g" \
    "${SCRIPT_DIR}/pkg/distribution.xml" > "${DIST_XML}"

# Build the product archive (distribution pkg) and sign it
productbuild \
    --distribution "${DIST_XML}" \
    --package-path "${ROOT_DIR}/build" \
    --sign "${APPLE_INSTALLER_IDENTITY}" \
    "${OUTPUT_PKG}"

echo "--- Package built: ${OUTPUT_PKG}"

# Clean up intermediate files
rm -rf "${PKG_ROOT}" "${PKG_SCRIPTS}" "${COMPONENT_PKG}" "${DIST_XML}"

# ---- Step 4: Notarize ----
if [ "${NOTARIZE}" = true ]; then
    echo "--- Submitting for notarization"

    if [ -n "${APPLE_NOTARY_PASSWORD:-}" ]; then
        # CI: use explicit credentials
        xcrun notarytool submit "${OUTPUT_PKG}" \
            --apple-id "${APPLE_ID}" \
            --team-id "${APPLE_TEAM_ID}" \
            --password "${APPLE_NOTARY_PASSWORD}" \
            --wait
    else
        # Local: use keychain profile (set up with: xcrun notarytool store-credentials valet-notary)
        xcrun notarytool submit "${OUTPUT_PKG}" \
            --keychain-profile "valet-notary" \
            --wait
    fi

    echo "--- Stapling notarization ticket"
    xcrun stapler staple "${OUTPUT_PKG}"
fi

echo ""
echo "==> Done: ${OUTPUT_PKG}"
echo "    Install with: sudo installer -pkg ${OUTPUT_PKG} -target /"
