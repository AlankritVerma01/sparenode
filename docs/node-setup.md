# Node setup

This document describes the current prototype. SpareNode publishes versioned
binaries, but does not yet ship a node installer.

## Requirements

- A Linux host reachable through OpenSSH
- Docker Engine
- The SpareNode binary available as `spare` in the remote user's `PATH`
- For GPU jobs, a working NVIDIA driver and NVIDIA Container Toolkit

Install the matching Linux archive from
[GitHub Releases](https://github.com/AlankritVerma01/sparenode/releases), or
build and install the current checkout:

```console
make check
make build
sudo install -o root -g root -m 0755 bin/spare /usr/local/bin/spare
```

## Docker permission boundary

The account running `spare rpc` must be able to use the Docker daemon. On many
Linux installations that means membership in the `docker` group.

Docker access is effectively root access: a user who can create arbitrary
containers can mount host paths and modify the host. SpareNode therefore does
not add users to this group automatically. During the prototype phase, grant
this permission only to a node owner you already trust completely.

After changing group membership, start a new login session before testing.

## SSH

Use standard OpenSSH key authentication and host verification. SpareNode does
not alter `sshd_config`, generate permanent host keys, open firewall ports, or
enable password authentication.

Confirm ordinary SSH works first:

```console
ssh dev@gpu-node spare version
```

Then use the client transport:

```console
spare --host dev@gpu-node doctor --data-path /data
```

Workspace paths belong to the node, not the client. For example, a repository
at `/srv/project` on the node can back a long-running development container:

```console
spare --host dev@gpu-node run --name dev --image ubuntu:24.04 --workspace /srv/project sleep infinity
spare --host dev@gpu-node exec dev git status
```

For remote internet access, place the node on an authenticated private network
instead of forwarding a public SSH port.

## Acceptance test

Run the GPU smoke test directly on the node:

```console
./scripts/smoke-gpu.sh
```

Then exercise the same lifecycle remotely:

```console
spare --host dev@gpu-node run --name gpu-check --image ubuntu:24.04 --gpu nvidia-smi -L
spare --host dev@gpu-node logs gpu-check
spare --host dev@gpu-node stop gpu-check
spare --host dev@gpu-node remove gpu-check
```

The final `spare --host dev@gpu-node jobs` must show no leftover test jobs.
