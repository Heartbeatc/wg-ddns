#!/usr/bin/env bash
set -euo pipefail

REPO_OWNER="${WGDDNS_REPO_OWNER:-Heartbeatc}"
REPO_NAME="${WGDDNS_REPO_NAME:-wg-ddns}"
REF="${WGDDNS_REF:-main}"
INSTALL_DIR="${WGDDNS_INSTALL_DIR:-}"
TMP_DIR="$(mktemp -d)"
ARCHIVE_URL="https://codeload.github.com/${REPO_OWNER}/${REPO_NAME}/tar.gz/refs/heads/${REF}"

cleanup() {
  rm -rf "${TMP_DIR}"
}
trap cleanup EXIT

log() {
  printf '[wg-ddns] %s\n' "$*"
}

fail() {
  printf '[wg-ddns] %s\n' "$*" >&2
  exit 1
}

need_cmd() {
  command -v "$1" >/dev/null 2>&1
}

detect_install_dir() {
  if [ -n "${INSTALL_DIR}" ]; then
    printf '%s\n' "${INSTALL_DIR}"
    return
  fi

  if [ -w /usr/local/bin ]; then
    printf '/usr/local/bin\n'
    return
  fi

  printf '%s/.local/bin\n' "${HOME}"
}

install_go() {
  log 'Go 未安装，尝试自动安装'

  if need_cmd apt-get; then
    export DEBIAN_FRONTEND=noninteractive
    apt-get update
    apt-get install -y golang-go curl tar
    return
  fi

  if need_cmd dnf; then
    dnf install -y golang curl tar
    return
  fi

  if need_cmd yum; then
    yum install -y golang curl tar
    return
  fi

  if need_cmd apk; then
    apk add --no-cache go curl tar
    return
  fi

  if need_cmd brew; then
    brew install go
    return
  fi

  fail '未找到可用的包管理器来安装 Go，请先手动安装 Go 1.19+ 后重试'
}

ensure_requirements() {
  need_cmd curl || fail '需要 curl'
  need_cmd tar || fail '需要 tar'

  if ! need_cmd go; then
    install_go
  fi

  need_cmd go || fail 'Go 安装失败，请手动安装 Go 1.19+'
}

download_source() {
  log "下载源码 ${REPO_OWNER}/${REPO_NAME}@${REF}"
  curl -fsSL "${ARCHIVE_URL}" -o "${TMP_DIR}/src.tar.gz"
  tar -xzf "${TMP_DIR}/src.tar.gz" -C "${TMP_DIR}"
}

build_binary() {
  local src_dir
  src_dir="$(find "${TMP_DIR}" -maxdepth 1 -type d -name "${REPO_NAME}-*" | head -n 1)"
  [ -n "${src_dir}" ] || fail '解压源码失败'

  log '编译 wgstack'
  (
    cd "${src_dir}"
    GO111MODULE=on go build -o "${TMP_DIR}/wgstack" ./cmd/wgstack
  )
}

install_binary() {
  local target_dir
  target_dir="$(detect_install_dir)"
  mkdir -p "${target_dir}"

  if [ -w "${target_dir}" ]; then
    install -m 0755 "${TMP_DIR}/wgstack" "${target_dir}/wgstack"
  else
    sudo install -d "${target_dir}"
    sudo install -m 0755 "${TMP_DIR}/wgstack" "${target_dir}/wgstack"
  fi

  log "已安装到 ${target_dir}/wgstack"
  case ":${PATH}:" in
    *":${target_dir}:"*) ;;
    *)
      log "注意：${target_dir} 不在当前 PATH 中"
      ;;
  esac
}

main() {
  ensure_requirements
  download_source
  build_binary
  install_binary
  log '完成，执行 `wgstack --help` 查看命令'
}

main "$@"
