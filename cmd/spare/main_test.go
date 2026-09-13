package main

import (
	"context"
	"flag"
	"io"
	"testing"
)

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
