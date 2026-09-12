# Contributing

SpareNode is intentionally early and narrow. Changes should strengthen the
existing Linux-node and OpenSSH workflow before adding new control planes,
runtimes, dashboards, or schedulers.

## Development checks

```console
make check
go test -race ./...
make build
./scripts/smoke-cpu.sh
```

GPU or Docker changes should also pass `./scripts/smoke-gpu.sh` on compatible
hardware. The smoke test must clean up every container it creates.

## Design constraints

- Prefer established interfaces such as OpenSSH, Docker, OCI/CDI, and the Dev
  Container specification over custom equivalents.
- Keep host command execution behind testable interfaces.
- Never interpolate user arguments into shell command strings.
- Never operate on an existing container unless it carries SpareNode's
  management label; resolve it to an immutable container ID before acting.
- Document any permission that grants effective root access.
- Do not add a dependency when the standard library is sufficient and clearer.

Please open an issue before implementing a large new subsystem. Report security
problems using the private process described in [SECURITY.md](SECURITY.md).
