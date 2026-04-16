#!/usr/bin/env bash
set -euo pipefail

REPO_OWNER="${WGDDNS_REPO_OWNER:-Heartbeatc}"
REPO_NAME="${WGDDNS_REPO_NAME:-wg-ddns}"
REF="${WGDDNS_REF:-main}"
INSTALL_DIR="${WGDDNS_INSTALL_DIR:-}"
ALLOW_SOURCE_BUILD="${WGDDNS_ALLOW_SOURCE_BUILD:-0}"
TMP_DIR="$(mktemp -d)"

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

release_tag_for_ref() {
  if [ "${REF}" = "main" ]; then
    printf 'edge\n'
  else
    printf '%s\n' "${REF}"
  fi
}

detect_platform() {
  local os arch
  os="$(uname -s | tr '[:upper:]' '[:lower:]')"
  arch="$(uname -m)"

  case "${os}" in
    linux|darwin) ;;
    *)
      fail "暂不支持的平台: ${os}"
      ;;
  esac

  case "${arch}" in
    x86_64|amd64) arch="amd64" ;;
    arm64|aarch64) arch="arm64" ;;
    *)
      fail "暂不支持的架构: ${arch}"
      ;;
  esac

  printf '%s %s\n' "${os}" "${arch}"
}

download_prebuilt() {
  need_cmd curl || fail '需要 curl'
  need_cmd tar || fail '需要 tar'

  local os arch tag asset url
  read -r os arch <<<"$(detect_platform)"
  tag="$(release_tag_for_ref)"
  asset="wgstack_${tag}_${os}_${arch}.tar.gz"
  url="https://github.com/${REPO_OWNER}/${REPO_NAME}/releases/download/${tag}/${asset}"

  log "下载预编译二进制 ${REPO_OWNER}/${REPO_NAME}@${tag} (${os}/${arch})"
  if ! curl -fsSL "${url}" -o "${TMP_DIR}/wgstack.tar.gz"; then
    return 1
  fi

  tar -xzf "${TMP_DIR}/wgstack.tar.gz" -C "${TMP_DIR}"
  [ -f "${TMP_DIR}/wgstack" ] || fail '压缩包中未找到 wgstack'
  chmod 0755 "${TMP_DIR}/wgstack"
}

download_source() {
  local archive_url
  archive_url="https://codeload.github.com/${REPO_OWNER}/${REPO_NAME}/tar.gz/refs/heads/${REF}"
  log "下载源码 ${REPO_OWNER}/${REPO_NAME}@${REF}"
  curl -fsSL "${archive_url}" -o "${TMP_DIR}/src.tar.gz"
  tar -xzf "${TMP_DIR}/src.tar.gz" -C "${TMP_DIR}"
}

build_binary() {
  local src_dir
  src_dir="$(find "${TMP_DIR}" -maxdepth 1 -type d -name "${REPO_NAME}-*" | head -n 1)"
  [ -n "${src_dir}" ] || fail '解压源码失败'

  need_cmd go || fail '预编译二进制不可用，且本机未安装 Go；请稍后重试或使用已发布 Release'
  log '回退到源码编译'
  (
    cd "${src_dir}"
    GO111MODULE=on go build -o "${TMP_DIR}/wgstack" ./cmd/wgstack
  )
}

prepare_binary() {
  if download_prebuilt; then
    return
  fi

  if [ "${ALLOW_SOURCE_BUILD}" != "1" ]; then
    fail '未找到当前平台的预编译二进制。请稍后重试，或设置 WGDDNS_ALLOW_SOURCE_BUILD=1 允许源码编译'
  fi

  need_cmd curl || fail '需要 curl'
  need_cmd tar || fail '需要 tar'
  download_source
  build_binary
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
  prepare_binary
  install_binary
  log '安装完成！'
  log '运行 wgstack 开始部署向导。'
}

main "$@"
