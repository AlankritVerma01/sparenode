# SpareNode

[![CI](https://github.com/AlankritVerma01/sparenode/actions/workflows/ci.yml/badge.svg)](https://github.com/AlankritVerma01/sparenode/actions/workflows/ci.yml)
[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)

SpareNode is an open-source tool for turning a spare Linux computer into a
private development and GPU compute node.

The project is deliberately small today. The first milestone validates the
host and launches explicitly managed Docker jobs. Remote access, sharing, and
a graphical control plane come after the local execution boundary is reliable.

## Current commands

```console
spare doctor --data-path "$HOME/Data"
spare doctor --json --data-path "$HOME/Data"
spare run --name hello --image alpine:latest echo hello
spare run --name cuda-check --image ubuntu:24.04 --gpu nvidia-smi -L
spare jobs
spare logs cuda-check
spare stop cuda-check
spare remove cuda-check
```

Only containers carrying the `dev.sparenode.managed=true` label appear in
`spare jobs`.

## Build and test

Requirements: Go 1.27+ and, for real jobs, a running Docker daemon.

```console
make check
make build
./bin/spare doctor --data-path "$HOME/Data"
./scripts/smoke-gpu.sh
```

The unit tests do not require Docker or NVIDIA hardware. A real GPU smoke test
requires an NVIDIA driver and NVIDIA Container Toolkit on the node.

### Testing strategy

Every change should pass four layers:

1. Unit tests with fake host commands; these must run without Docker or a GPU.
2. `go vet` and race-detector tests.
3. Cross-builds for the supported client platforms, starting with macOS arm64.
4. A disposable hardware acceptance job that runs `nvidia-smi` inside a
   SpareNode-managed container, reads its logs, stops it, and removes it.

The acceptance test must finish with `spare jobs` reporting no leftover jobs.
Tests must never inspect, stop, or remove containers that do not carry the
SpareNode management label.

## Status

SpareNode is an early working prototype. Local NVIDIA GPU jobs have been
validated end to end on the reference Linux node. The remote node agent and
stable release process are not implemented yet.

## Product boundary

SpareNode is intended initially for hardware controlled by one owner and for
explicitly trusted collaborators. Containers are not a sufficient isolation
boundary for running hostile public workloads.

### In scope for the first usable release

- Linux node readiness and hardware inventory
- Docker-based CPU and NVIDIA GPU jobs
- Persistent job metadata, logs, and artifacts
- Secure remote operation over an existing SSH or private-network connection
- Reusable, repository-owned environment definitions

### Not in scope yet

- Kubernetes
- Public GPU rental or billing
- Hostile multi-tenant execution
- A custom editor
- A custom VPN
- A web dashboard before the CLI workflow is proven

## Roadmap

1. Prove local CPU and GPU jobs on the reference laptop.
2. Add a small node agent and authenticated remote CLI protocol.
3. Add repository checkout and Dev Container compatibility.
4. Add invitations, resource limits, and multi-node discovery.

## License

Apache-2.0.
