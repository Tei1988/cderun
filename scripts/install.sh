#!/usr/bin/env bash
set -euo pipefail

REPO="Tei1988/cderun"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.config/cderun/.bin}"
SYMLINK_PATH="${SYMLINK_PATH:-/usr/local/bin/cderun}"

auth_header=()
[[ -n "${GITHUB_TOKEN:-}" ]] && auth_header=("-H" "Authorization: Bearer ${GITHUB_TOKEN}")
# Resolve latest version if not specified
VERSION="${VERSION:-${1:-}}"
[[ -n "$VERSION" && "$VERSION" != v* ]] && VERSION="v${VERSION}"
if [[ -z "$VERSION" ]]; then
  api_response=$(curl -sSL -H "Accept: application/vnd.github+json" ${auth_header[@]+"${auth_header[@]}"} "https://api.github.com/repos/${REPO}/releases/latest" 2>&1) || {
    echo "Error: Failed to fetch latest release from GitHub API for ${REPO}"
    echo "Context: $api_response"
    exit 1
  }
  VERSION=$(echo "$api_response" | grep '"tag_name"' | sed 's/.*"tag_name": *"\([^"]*\)".*/\1/')
  if [[ -z "$VERSION" ]]; then
    echo "Error: VERSION resolution failed for ${REPO}"
    echo "Context: $api_response"
    exit 1
  fi
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
  local checksum_url="https://github.com/${REPO}/releases/download/${VERSION}/checksums.txt"
  local dest="${INSTALL_DIR}/cderun_${target_os}_${target_arch}"

  echo "Downloading and verifying ${archive} ..."
  local tmp_dir
  tmp_dir=$(mktemp -d)
  # shellcheck disable=SC2064
  trap "rm -rf \"$tmp_dir\"" RETURN

  curl -fsSL ${auth_header[@]+"${auth_header[@]}"} "$url" -o "${tmp_dir}/${archive}"
  curl -fsSL ${auth_header[@]+"${auth_header[@]}"} "$checksum_url" -o "${tmp_dir}/checksums.txt"

  (cd "$tmp_dir" && sha256sum --check --ignore-missing "checksums.txt" > /dev/null 2>&1) || {
    echo "Error: Checksum verification failed for ${archive}"
    return 1
  }

  tar -xz -C "$tmp_dir" -f "${tmp_dir}/${archive}" cderun
  mv "${tmp_dir}/cderun" "$dest"
  chmod +x "$dest"
}

mkdir -p "$INSTALL_DIR"
INSTALL_DIR=$(cd "$INSTALL_DIR" && pwd)

mapped_arch=$(map_arch "$arch")
download "$os" "$mapped_arch"
ln -sf "${INSTALL_DIR}/cderun_${os}_${mapped_arch}" "${INSTALL_DIR}/cderun"

# On macOS, also download the linux binary for container use
if [[ "$os" == "darwin" ]]; then
  download "linux" "$mapped_arch"

  if [[ -z "${SKIP_CONFIG:-}" ]]; then
    config="$HOME/.config/cderun/.cderun.yaml"
    if [[ ! -f "$config" ]]; then
      mkdir -p "$(dirname "$config")"
      cat > "$config" <<EOF
defaults:
  mountCderunPath: ${INSTALL_DIR}/cderun_linux_${mapped_arch}
EOF
    fi
  fi
fi

# Create symlink (use sudo if needed)
if [[ -w "$(dirname "$SYMLINK_PATH")" ]]; then
  rm -f "$SYMLINK_PATH" && ln -s "${INSTALL_DIR}/cderun" "$SYMLINK_PATH"
else
  sudo rm -f "$SYMLINK_PATH" && sudo ln -s "${INSTALL_DIR}/cderun" "$SYMLINK_PATH"
fi

echo "cderun ${VERSION} installed to ${INSTALL_DIR}"
echo "Symlink created: ${SYMLINK_PATH} -> ${INSTALL_DIR}/cderun"
