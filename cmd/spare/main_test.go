package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"runtime/debug"
	"testing"
)

type testExitError int

func (err testExitError) Error() string { return fmt.Sprintf("exit status %d", err) }
func (err testExitError) ExitCode() int { return int(err) }

func TestParseFlagsTreatsHelpAsSuccess(t *testing.T) {
	flags := flag.NewFlagSet("test", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	help, err := parseFlags(flags, []string{"--help"})
	if err != nil || !help {
		t.Fatalf("expected successful help, got help=%v err=%v", help, err)
	}
}

func TestParseFlagsReturnsOrdinaryErrors(t *testing.T) {
	flags := flag.NewFlagSet("test", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	help, err := parseFlags(flags, []string{"--unknown"})
	if err == nil || help {
		t.Fatalf("expected a non-help error, got help=%v err=%v", help, err)
	}
}

func TestAllCommandsAcceptHelp(t *testing.T) {
	commands := []string{"doctor", "run", "jobs", "logs", "exec", "wait", "stop", "remove", "version"}
	for _, command := range commands {
		t.Run(command, func(t *testing.T) {
			if err := runLocal(context.Background(), []string{command, "--help"}); err != nil {
				t.Fatalf("help returned an error: %v", err)
			}
		})
	}
}

func TestDoctorRejectsPositionalArguments(t *testing.T) {
	err := runDoctor(context.Background(), nil, []string{"unexpected"})
	if err == nil || err.Error() != "doctor does not accept positional arguments" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestProcessExitCodePreservesWrappedStatus(t *testing.T) {
	err := fmt.Errorf("remote command failed: %w", testExitError(17))
	if code := processExitCode(err); code != 17 {
		t.Fatalf("got exit code %d, want 17", code)
	}
}

func TestProcessExitCodeDefaultsToOne(t *testing.T) {
	for _, err := range []error{errors.New("ordinary failure"), testExitError(-1), testExitError(256)} {
		if code := processExitCode(err); code != 1 {
			t.Fatalf("got exit code %d for %v, want 1", code, err)
		}
	}
}

func TestResolvedVersionUsesModuleVersionForGoInstall(t *testing.T) {
	info := &debug.BuildInfo{Main: debug.Module{Version: "v0.1.0-alpha.8"}}
	if got := resolvedVersion("dev", info); got != "0.1.0-alpha.8" {
		t.Fatalf("got version %q", got)
	}
}

func TestResolvedVersionPreservesLinkerVersion(t *testing.T) {
	info := &debug.BuildInfo{Main: debug.Module{Version: "v9.9.9"}}
	if got := resolvedVersion("0.1.0-alpha.8", info); got != "0.1.0-alpha.8" {
		t.Fatalf("got version %q", got)
	}
}

func TestResolvedVersionLeavesDevelopmentBuildAlone(t *testing.T) {
	for _, info := range []*debug.BuildInfo{nil, {}, {Main: debug.Module{Version: "(devel)"}}} {
		if got := resolvedVersion("dev", info); got != "dev" {
			t.Fatalf("got version %q", got)
		}
	}
}
