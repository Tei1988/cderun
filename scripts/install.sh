#!/usr/bin/env bash -xe
set -euo pipefail

REPO="Tei1988/cderun"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.config/cderun/.bin}"
SYMLINK_PATH="${SYMLINK_PATH:-/usr/local/bin/cderun}"

# Resolve latest version if not specified
VERSION="${VERSION:-}"
if [[ -z "$VERSION" ]]; then
  VERSION=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
    | grep '"tag_name"' | sed 's/.*"tag_name": *"\(.*\)".*/\1/')
fi

os=$(uname -s | tr '[:upper:]' '[:lower:]')
arch=$(uname -m)

map_arch() {
  case "$1" in
    x86_64)  echo "x86_64" ;;
    aarch64|arm64) echo "arm64" ;;
    armv7l)  echo "armv7" ;;
    armv6l)  echo "armv6" ;;
    riscv64) echo "riscv64" ;;
    *) echo "Unsupported architecture: $1" >&2; exit 1 ;;
  esac
}

download() {
  local target_os="$1" target_arch="$2"
  local archive="cderun_${target_os}_${target_arch}.tar.gz"
  local url="https://github.com/${REPO}/releases/download/${VERSION}/${archive}"
  local dest="${INSTALL_DIR}/cderun_${target_os}_${target_arch}"

  echo "Downloading ${url} ..."
  curl -fsSL "$url" | tar -xz -O cderun > "$dest"
  chmod +x "$dest"
}

mkdir -p "$INSTALL_DIR"

mapped_arch=$(map_arch "$arch")
download "$os" "$mapped_arch"
ln -sf "${INSTALL_DIR}/cderun_${os}_${mapped_arch}" "${INSTALL_DIR}/cderun"

# On macOS, also download the linux binary for container use
if [[ "$os" == "darwin" ]]; then
  download "linux" "$mapped_arch"

  config="$HOME/.config/cderun/.cderun.yaml"
  if [[ ! -f "$config" ]]; then
    cat > "$config" <<EOF
defaults:
  mountCderunPath: ${INSTALL_DIR}/cderun_linux_${mapped_arch}
EOF
  fi
fi

# Create symlink (use sudo if needed)
_symlink() { rm -f "$SYMLINK_PATH" && ln -s "${INSTALL_DIR}/cderun" "$SYMLINK_PATH"; }
if [[ -w "$(dirname "$SYMLINK_PATH")" ]]; then
  _symlink
else
  sudo bash -c "$(declare -f _symlink); SYMLINK_PATH='$SYMLINK_PATH' INSTALL_DIR='$INSTALL_DIR' _symlink"
fi

echo "cderun ${VERSION} installed to ${INSTALL_DIR}"
echo "Symlink created: ${SYMLINK_PATH} -> ${INSTALL_DIR}/cderun"
