# Set up a node

A SpareNode node is a Linux machine running Docker. The `spare` binary uses the
Docker daemon locally; remote clients reach that binary through SSH.

## 1. Install the existing tools

Install:

- [Docker Engine](https://docs.docker.com/engine/install/)
- an NVIDIA driver and
  [NVIDIA Container Toolkit](https://docs.nvidia.com/datacenter/cloud-native/container-toolkit/latest/install-guide.html)
  if you want GPU jobs
- the matching Linux archive from
  [SpareNode releases](https://github.com/AlankritVerma01/sparenode/releases)

Place `spare` in the remote user's `PATH`. To install a local build instead:

```console
make check
make build
sudo install -o root -g root -m 0755 bin/spare /usr/local/bin/spare
```

## 2. Allow the node owner to use Docker

On Docker installations that use the standard Unix socket:

```console
sudo usermod -aG docker "$USER"
```

Log out completely and log back in, then run:

```console
docker info
```

Membership in the Docker group is effectively root access. Grant it only to
the trusted owner account that runs SpareNode.

## 3. Make SSH private

Use an SSH setup you already trust. Regular OpenSSH with key authentication is
supported. Do not expose password-based SSH to the internet for SpareNode.

[Tailscale SSH](https://tailscale.com/docs/features/tailscale-ssh) is a simple
option that does not require a public port. SpareNode does not install or manage
Tailscale; it still calls the standard `ssh` client.

On Omarchy:

```console
omarchy pkg add tailscale
```

On other Arch Linux systems:

```console
sudo pacman -S tailscale
```

Then:

```console
sudo systemctl enable --now tailscaled
sudo tailscale up --ssh
```

Open the authentication URL, join the client computer to the same tailnet, and
confirm the connection:

```console
ssh dev@gpu-node spare version
```

Use the node's Tailscale name or private IP in place of `gpu-node`.

## 4. Check the node

Choose a data directory owned by the node account and run:

```console
spare doctor --require-gpu --data-path /data
```

For a CPU-only node, omit `--require-gpu`.

From the client, repeat the check through SSH:

```console
spare --host dev@gpu-node doctor --require-gpu --data-path /data
```

## Files and ports

SpareNode does not copy repositories. Use Git for committed work or `rsync` for
local changes:

```console
rsync -az --exclude .git/ ./ dev@gpu-node:/data/projects/my-app/
```

`--workspace /data/projects/my-app` mounts that node directory at `/workspace`
inside a job. The container can change everything in the mounted directory.

Ports published with `--publish` bind only to the node's loopback interface.
Reach them through an SSH tunnel:

```console
ssh -N -L 3000:127.0.0.1:3000 dev@gpu-node
```

## Final test

Run a complete GPU job from the client:

```console
spare --host dev@gpu-node run --name gpu-check --image ubuntu:24.04 --gpu nvidia-smi -L
spare --host dev@gpu-node logs --follow gpu-check
spare --host dev@gpu-node remove gpu-check
spare --host dev@gpu-node jobs
```

The last command should show no remaining `gpu-check` job.
