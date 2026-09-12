# SpareNode

[![CI](https://github.com/AlankritVerma01/sparenode/actions/workflows/ci.yml/badge.svg)](https://github.com/AlankritVerma01/sparenode/actions/workflows/ci.yml)
[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)

SpareNode is an open-source tool for turning a spare Linux computer into a
private development and GPU compute node.

The project is deliberately small today. It validates a node, launches
explicitly managed Docker jobs, and controls them through an existing OpenSSH
connection. Sharing and a graphical control plane come after this CLI workflow
is reliable across separate machines.

## Install

Download the archive for your Linux node or Mac client from
[GitHub Releases](https://github.com/AlankritVerma01/sparenode/releases). Extract
it, then place `spare` somewhere in your `PATH`. The project does not publish a
remote install script.

Build from source with Go 1.27 or newer:

```console
make check
make build
sudo install -o root -g root -m 0755 bin/spare /usr/local/bin/spare
```

## Current commands

```console
spare version --json
spare doctor --data-path "$HOME/Data"
spare doctor --json --data-path "$HOME/Data"
spare run --name hello --image alpine:latest echo hello
spare run --name cuda-check --image ubuntu:24.04 --gpu nvidia-smi -L
spare jobs
spare logs cuda-check
spare stop cuda-check
spare remove cuda-check
```

Run the same commands on a node already configured in OpenSSH:

```console
spare --host dev@gpu-node doctor --data-path /data
spare --host dev@gpu-node run --name hello --image alpine:latest echo hello
spare --host dev@gpu-node logs hello
```

SpareNode delegates authentication, host verification, proxies, and private
network routing to OpenSSH and the user's existing SSH configuration. Command
arguments are sent as a versioned JSON request over standard input rather than
interpolated into a remote shell command. `SPARENODE_HOST` can set the default
destination; `SPARENODE_SSH_CONFIG` can select a non-default SSH config file.

See [Node setup](docs/node-setup.md) before granting a remote account access to
Docker. That permission is intentionally not automated by SpareNode.

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
validated end to end on the reference Linux node. OpenSSH transport and
prerelease automation are implemented; automated node installation and a
stable release are not.

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
2. Validate the OpenSSH transport from a separate client machine.
3. Add a conservative node installer and Docker-access diagnostics.
4. Add repository checkout and Dev Container compatibility.
5. Add invitations, resource limits, and multi-node discovery.

## License

Apache-2.0.
