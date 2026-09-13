# SpareNode

[![CI](https://github.com/AlankritVerma01/sparenode/actions/workflows/ci.yml/badge.svg)](https://github.com/AlankritVerma01/sparenode/actions/workflows/ci.yml)
[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)

Use a Linux computer you already own as a private Docker and NVIDIA GPU worker.
Run jobs from the machine itself or control them from a Mac or another computer
over SSH.

SpareNode is an early alpha for one owner and people they trust. It is a small
CLI, not a hosting platform or a security boundary for untrusted workloads.

```text
your computer  -- SSH -->  Linux node  -->  Docker  -->  CPU / NVIDIA GPU
```

SSH handles access, Docker runs containers, and NVIDIA Container Toolkit
exposes the GPU. There is no SpareNode daemon or account system.

## Install

Download the archive for your platform from
[GitHub Releases](https://github.com/AlankritVerma01/sparenode/releases), extract
it, and put `spare` somewhere in your `PATH`.

If you already have Go 1.27 or newer:

```console
go install github.com/AlankritVerma01/sparenode/cmd/spare@latest
```

The Linux node also needs Docker. GPU jobs need an NVIDIA driver and NVIDIA
Container Toolkit. Follow the short [node setup guide](docs/node-setup.md) before
connecting remotely.

## Try it on the node

Check that Docker, the GPU, and your data directory are ready:

```console
spare doctor --require-gpu --data-path "$HOME/Data"
```

Run a disposable GPU job:

```console
spare run --name gpu-check --image ubuntu:24.04 --gpu nvidia-smi -L
spare logs --follow gpu-check
spare remove gpu-check
```

Omit `--require-gpu` and `--gpu` on a CPU-only machine.

## Use it remotely

First make sure ordinary SSH works:

```console
ssh dev@gpu-node spare version
```

Then add `--host` to the same SpareNode commands:

```console
spare --host dev@gpu-node doctor --require-gpu --data-path /data
spare --host dev@gpu-node run --name gpu-check --image ubuntu:24.04 --gpu nvidia-smi -L
spare --host dev@gpu-node logs --follow gpu-check
spare --host dev@gpu-node remove gpu-check
```

Set `SPARENODE_HOST=dev@gpu-node` if you do not want to repeat `--host`.

For a development container, mount a repository that already exists on the
node:

```console
spare --host dev@gpu-node run \
  --name dev \
  --image ubuntu:24.04 \
  --workspace /data/projects/my-app \
  --cpus 2 \
  --memory 4g \
  sleep infinity

spare --host dev@gpu-node exec dev ls -la /workspace
spare --host dev@gpu-node stop dev
spare --host dev@gpu-node remove dev
```

Run `spare --help` for all commands and options.

## Safety

Docker access is effectively root access. Only let trusted people submit jobs,
and only mount directories a job should be able to change. SpareNode only lists
or modifies containers carrying its management label.

Report security problems privately through
[GitHub Security Advisories](https://github.com/AlankritVerma01/sparenode/security/advisories/new).

## Develop

```console
make check
make build
./scripts/smoke-cpu.sh
# On a configured NVIDIA node:
./scripts/smoke-gpu.sh
```

See [CONTRIBUTING.md](CONTRIBUTING.md) before proposing a large feature.

Apache-2.0 licensed.
