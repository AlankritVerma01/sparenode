package execx

import (
	"context"
	"io"
	"os/exec"
	"strings"
)

// Runner makes host command execution replaceable in tests.
type Runner interface {
	LookPath(file string) (string, error)
	Run(ctx context.Context, name string, args ...string) (string, error)
}

type StreamingRunner interface {
	Runner
	Stream(ctx context.Context, stdout, stderr io.Writer, name string, args ...string) error
}

type OSRunner struct{}

func (OSRunner) LookPath(file string) (string, error) {
	return exec.LookPath(file)
}

func (OSRunner) Run(ctx context.Context, name string, args ...string) (string, error) {
	out, err := exec.CommandContext(ctx, name, args...).CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

func (OSRunner) Stream(ctx context.Context, stdout, stderr io.Writer, name string, args ...string) error {
	command := exec.CommandContext(ctx, name, args...)
	command.Stdout = stdout
	command.Stderr = stderr
	return command.Run()
}
