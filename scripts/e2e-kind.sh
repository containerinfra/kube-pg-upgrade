#!/usr/bin/env bash
# Bootstrap a kind cluster for kube-pg-upgrade E2E tests (Kubernetes v1.37.0).
#
# kind must match the host CPU. An amd64 node on Apple Silicon runs under qemu
# and fails kubeadm (apiserver connection refused / rate-limiter deadlines).
# We pin the arch-specific kindest/node digest so a polluted amd64 cache cannot
# be selected.
set -euo pipefail

CLUSTER_NAME="${E2E_KIND_CLUSTER_NAME:-kube-pg-upgrade-e2e}"
HOST_ARCH="$(uname -m)"
MIN_KIND_VERSION="0.33.0"

# Digests for kindest/node:v1.37.0 from kind v0.33.0 release (per-arch, not the index).
NODE_IMAGE_AMD64="kindest/node:v1.37.0@sha256:1aac8018c42eb5d48fae5be507caaee789fc6d9b9b3f560359dd4157ac00d4f6"
NODE_IMAGE_ARM64="kindest/node:v1.37.0@sha256:6a3345f517a7df05185c648f83d18d464f2a94ea4a69eaee39b7e1de02611e59"

usage() {
  echo "Usage: $0 {up|down|kubeconfig}" >&2
  exit 1
}

require_kind_version() {
  local version
  version="$(kind version -q 2>/dev/null || kind version | awk '{print $2}' | sed 's/^v//')"
  version="${version#v}"
  if [[ -z "${version}" ]]; then
    echo "kind is required but not found in PATH" >&2
    exit 1
  fi
  if ! printf '%s\n%s\n' "${MIN_KIND_VERSION}" "${version}" | sort -V -C; then
    echo "kind v${version} is too old for Kubernetes 1.37 (need >= v${MIN_KIND_VERSION})." >&2
    echo "Upgrade, e.g.: brew upgrade kind" >&2
    exit 1
  fi
  echo "Using kind v${version}"
}

cluster_exists() {
  kind get clusters 2>/dev/null | grep -qx "${CLUSTER_NAME}"
}

node_image_for_host() {
  case "${HOST_ARCH}" in
    arm64|aarch64) echo "${NODE_IMAGE_ARM64}" ;;
    x86_64|amd64) echo "${NODE_IMAGE_AMD64}" ;;
    *)
      echo "unsupported host architecture: ${HOST_ARCH}" >&2
      exit 1
      ;;
  esac
}

docker_platform_for_host() {
  case "${HOST_ARCH}" in
    arm64|aarch64) echo "linux/arm64" ;;
    x86_64|amd64) echo "linux/amd64" ;;
  esac
}

# Drop cross-arch platform overrides that force the wrong kindest/node variant.
clear_cross_platform() {
  if [[ -n "${DOCKER_DEFAULT_PLATFORM:-}" ]]; then
    echo "Unsetting DOCKER_DEFAULT_PLATFORM=${DOCKER_DEFAULT_PLATFORM} for kind"
    unset DOCKER_DEFAULT_PLATFORM
  fi
}

cmd_up() {
  require_kind_version
  clear_cross_platform

  local node_image platform
  node_image="$(node_image_for_host)"
  platform="$(docker_platform_for_host)"

  if cluster_exists; then
    if kind export kubeconfig --name "${CLUSTER_NAME}" >/dev/null 2>&1 \
      && kubectl --context "kind-${CLUSTER_NAME}" get nodes >/dev/null 2>&1; then
      echo "kind cluster ${CLUSTER_NAME} already exists and is healthy; reusing"
    else
      echo "kind cluster ${CLUSTER_NAME} exists but is unhealthy; recreating..."
      kind delete cluster --name "${CLUSTER_NAME}"
    fi
  fi

  if ! cluster_exists; then
    echo "Ensuring native ${platform} node image is present..."
    # Pull by platform so we do not reuse an amd64 image cached from earlier attempts.
    docker pull --platform="${platform}" "${node_image}"

    echo "Creating kind cluster ${CLUSTER_NAME} (Kubernetes v1.37.0, ${platform})..."
    # env -u: kind inherits no DOCKER_DEFAULT_PLATFORM even if the parent re-exported it.
    env -u DOCKER_DEFAULT_PLATFORM kind create cluster \
      --name "${CLUSTER_NAME}" \
      --image "${node_image}" \
      --wait 120s
  fi

  kind export kubeconfig --name "${CLUSTER_NAME}"
  kubectl cluster-info --context "kind-${CLUSTER_NAME}"
  kubectl version --short 2>/dev/null || kubectl version

  # Sanity: node Architecture should match the host (arm64 vs amd64).
  local node_arch
  node_arch="$(kubectl --context "kind-${CLUSTER_NAME}" get nodes -o jsonpath='{.items[0].status.nodeInfo.architecture}')"
  echo "Cluster node architecture: ${node_arch} (host ${HOST_ARCH})"
  echo "E2E kind cluster ready: ${CLUSTER_NAME}"
}

cmd_down() {
  if cluster_exists; then
    echo "Deleting kind cluster ${CLUSTER_NAME}..."
    kind delete cluster --name "${CLUSTER_NAME}"
  else
    echo "kind cluster ${CLUSTER_NAME} does not exist"
  fi
}

cmd_kubeconfig() {
  if ! cluster_exists; then
    echo "kind cluster ${CLUSTER_NAME} does not exist; run '$0 up' first" >&2
    exit 1
  fi
  kind get kubeconfig --name "${CLUSTER_NAME}"
}

main() {
  if [[ $# -lt 1 ]]; then
    usage
  fi
  case "$1" in
    up) cmd_up ;;
    down) cmd_down ;;
    kubeconfig) cmd_kubeconfig ;;
    *) usage ;;
  esac
}

main "$@"
