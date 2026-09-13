## What changed

Describe the user-visible problem and the focused change that solves it.

## Verification

- [ ] `make check`
- [ ] `go test -race ./...`
- [ ] `make build`
- [ ] Docker or GPU behavior was smoke-tested when affected
- [ ] Documentation was updated when behavior changed

## Safety and scope

- [ ] User input is passed as arguments, not interpolated into a shell command
- [ ] Container lifecycle changes operate only on SpareNode-managed containers
- [ ] New permissions and dependencies are necessary and documented
