#!/usr/bin/env bash
set -euo pipefail

project_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
spare_bin="${SPARE_BIN:-$project_dir/bin/spare}"
job_name="sparenode-gpu-smoke-$$"

if [[ ! -x "$spare_bin" ]]; then
  echo "SpareNode binary not found; run 'make build' first." >&2
  exit 1
fi

spare=("$spare_bin")
docker_command=(docker)
if ! docker info >/dev/null 2>&1; then
  if sudo -n docker info >/dev/null 2>&1; then
    spare=(sudo -n "$spare_bin")
    docker_command=(sudo -n docker)
  else
    echo "Docker is unavailable to the current user." >&2
    exit 1
  fi
fi

if "${docker_command[@]}" inspect "$job_name" >/dev/null 2>&1; then
  echo "Refusing to reuse existing container: $job_name" >&2
  exit 1
fi

started=false
cleanup() {
  if [[ "$started" == true ]]; then
    "${spare[@]}" stop "$job_name" >/dev/null 2>&1 || true
    "${spare[@]}" remove "$job_name" >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT

"${spare[@]}" run --name "$job_name" --image ubuntu:24.04 --gpu nvidia-smi -L
started=true

for _ in {1..20}; do
  output="$("${spare[@]}" logs "$job_name")"
  if [[ "$output" == *"GPU 0:"* ]]; then
    printf '%s\n' "$output"
    echo "GPU smoke test passed."
    exit 0
  fi
  sleep 0.25
done

echo "GPU smoke test did not observe an NVIDIA GPU." >&2
exit 1
