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

### Optional private access with Tailscale SSH

[Tailscale SSH](https://tailscale.com/docs/features/tailscale-ssh) is one way to
reach a node without opening SSH to the public internet or maintaining a
custom VPN. It is optional; SpareNode still uses the system `ssh` client and
does not link to or manage Tailscale.

On Arch Linux and Omarchy, install the official
[distribution package](https://archlinux.org/packages/extra/x86_64/tailscale/)
and enable its service:

```console
sudo pacman -S tailscale
sudo systemctl enable --now tailscaled
sudo tailscale up --ssh
```

The final command prints an authentication URL. Join the client machine to the
same tailnet, review the tailnet's SSH access policy, and verify the connection
before using SpareNode:

```console
ssh dev@gpu-node spare version
spare --host dev@gpu-node doctor --data-path /data
```

Tailscale SSH runs its own SSH server for tailnet traffic, so a separate public
OpenSSH listener is not required for this setup. Regular OpenSSH over a private
Tailscale address remains a supported alternative.

Workspace paths belong to the node, not the client. For example, a repository
at `/srv/project` on the node can back a long-running development container:

```console
spare --host dev@gpu-node run --name dev --image ubuntu:24.04 --cpus 2 --memory 4g --workspace /srv/project --publish 3000:3000 sleep infinity
spare --host dev@gpu-node exec dev git status
```

Published ports listen only on the node's loopback interface. Keep a separate
OpenSSH tunnel running on the client to reach one:

```console
ssh -N -L 3000:127.0.0.1:3000 dev@gpu-node
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
