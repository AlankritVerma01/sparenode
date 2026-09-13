#!/usr/bin/env bash
set -euo pipefail

project_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
spare_bin="${SPARE_BIN:-$project_dir/bin/spare}"
job_name="sparenode-cpu-smoke-$$"
workspace_dir=""
follow_output=""
follow_pid=""

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

workspace_dir="$(mktemp -d)"
node_uid="$(id -u)"
node_gid="$(id -g)"
started=false
cleanup() {
  if [[ "$started" == true ]]; then
    "${spare[@]}" stop "$job_name" >/dev/null 2>&1 || true
    "${spare[@]}" remove "$job_name" >/dev/null 2>&1 || true
  fi
  if [[ -n "$follow_pid" ]]; then
    wait "$follow_pid" >/dev/null 2>&1 || true
  fi
  if [[ -n "$follow_output" ]]; then
    rm -f -- "$follow_output"
  fi
  rmdir "$workspace_dir" >/dev/null 2>&1 || true
}
trap cleanup EXIT

"${spare[@]}" run \
  --name "$job_name" \
  --image alpine:latest \
  --cpus 0.5 \
  --memory 64m \
  --user "$node_uid:$node_gid" \
  --workspace "$workspace_dir" \
  --env SPARENODE_SMOKE=ready \
  --publish :8080 \
  sh -c 'echo cpu-ready; exec nc -lk -p 8080 -e echo'
started=true

for _ in {1..20}; do
  output="$("${spare[@]}" logs "$job_name")"
  if [[ "$output" == *"cpu-ready"* ]]; then
    break
  fi
  sleep 0.25
done
if [[ "$output" != *"cpu-ready"* ]]; then
  echo "CPU smoke test did not observe the readiness message." >&2
  exit 1
fi

follow_output="$workspace_dir/follow.log"
"${spare[@]}" logs --follow "$job_name" >"$follow_output" 2>&1 &
follow_pid=$!

jobs="$("${spare[@]}" jobs --json)"
if [[ "$jobs" != *"\"name\": \"$job_name\""* ]]; then
  echo "Structured job list did not contain the smoke-test job." >&2
  exit 1
fi

workspace="$("${spare[@]}" exec "$job_name" pwd)"
if [[ "$workspace" != "/workspace" ]]; then
  echo "Unexpected container workspace: $workspace" >&2
  exit 1
fi

environment="$("${spare[@]}" exec "$job_name" printenv SPARENODE_SMOKE)"
if [[ "$environment" != "ready" ]]; then
  echo "Unexpected container environment: $environment" >&2
  exit 1
fi

identity="$("${spare[@]}" exec "$job_name" sh -c 'printf "%s:%s" "$(id -u)" "$(id -g)"')"
if [[ "$identity" != "$node_uid:$node_gid" ]]; then
  echo "Unexpected container identity: $identity" >&2
  exit 1
fi

if "${spare[@]}" exec "$job_name" sh -c 'exit 17' >/dev/null 2>&1; then
  exec_status=0
else
  exec_status=$?
fi
if [[ "$exec_status" -ne 17 ]]; then
  echo "Exec did not preserve the container command status: $exec_status" >&2
  exit 1
fi

host_ip="$("${docker_command[@]}" inspect --format '{{(index (index .NetworkSettings.Ports "8080/tcp") 0).HostIp}}' "$job_name")"
if [[ "$host_ip" != "127.0.0.1" ]]; then
  echo "Published port is not loopback-only: $host_ip" >&2
  exit 1
fi

limits="$("${docker_command[@]}" inspect --format '{{.HostConfig.NanoCpus}} {{.HostConfig.Memory}}' "$job_name")"
if [[ "$limits" != "500000000 67108864" ]]; then
  echo "Unexpected Docker resource limits: $limits" >&2
  exit 1
fi

"${spare[@]}" stop "$job_name" >/dev/null
wait "$follow_pid"
follow_pid=""
if ! grep -q 'cpu-ready' "$follow_output"; then
  echo "Live log stream did not contain the readiness output." >&2
  exit 1
fi
rm -f -- "$follow_output"
follow_output=""
wait_result="$("${spare[@]}" wait --json "$job_name")"
if [[ "$wait_result" != *"\"exit_code\":"* ]]; then
  echo "Wait result did not contain an exit code." >&2
  exit 1
fi
"${spare[@]}" remove "$job_name" >/dev/null
started=false
rmdir "$workspace_dir"
trap - EXIT

echo "CPU smoke test passed."
