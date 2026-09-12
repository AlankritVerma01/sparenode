package main

import (
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
