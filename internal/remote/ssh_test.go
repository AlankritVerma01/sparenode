package remote

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestSplitHost(t *testing.T) {
	tests := []struct {
		name string
		args []string
		env  string
		host string
		rest []string
	}{
		{name: "flag", args: []string{"--host", "dev@node", "doctor"}, host: "dev@node", rest: []string{"doctor"}},
		{name: "short flag", args: []string{"-H", "node", "jobs"}, host: "node", rest: []string{"jobs"}},
		{name: "equals", args: []string{"--host=node", "jobs"}, host: "node", rest: []string{"jobs"}},
		{name: "environment", args: []string{"jobs"}, env: "node", host: "node", rest: []string{"jobs"}},
		{name: "local", args: []string{"jobs"}, rest: []string{"jobs"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			host, rest, err := SplitHost(test.args, test.env)
			if err != nil {
				t.Fatal(err)
			}
			if host != test.host || strings.Join(rest, "|") != strings.Join(test.rest, "|") {
				t.Fatalf("got host=%q rest=%q", host, rest)
			}
		})
	}
}

func TestSSHCommandArgs(t *testing.T) {
	args, err := SSHCommandArgs("dev@node", "/tmp/ssh-config")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"-T", "-F", "/tmp/ssh-config", "dev@node", "spare", "rpc"}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("got %#v, want %#v", args, want)
	}
}

func TestSplitHostRejectsOptionInjection(t *testing.T) {
	if _, _, err := SplitHost([]string{"--host", "-oProxyCommand=bad", "jobs"}, ""); err == nil {
		t.Fatal("expected an error")
	}
}

func TestRequestRoundTripPreservesArguments(t *testing.T) {
	args := []string{"run", "--name", "safe", "--image", "alpine", "printf", "$HOME; rm -rf /"}
	encoded, err := EncodeRequest(args)
	if err != nil {
		t.Fatal(err)
	}
	request, err := DecodeRequest(bytes.NewReader(encoded))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(request.Args, "\x00") != strings.Join(args, "\x00") {
		t.Fatalf("arguments changed: %#v", request.Args)
	}
}

func TestDecodeRequestRejectsUnknownVersion(t *testing.T) {
	_, err := DecodeRequest(strings.NewReader(`{"version":2,"args":["jobs"]}`))
	if err == nil || !strings.Contains(err.Error(), "unsupported protocol version") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDecodeRequestRejectsOversizedInput(t *testing.T) {
	_, err := DecodeRequest(strings.NewReader(strings.Repeat("x", maxRequestBytes+1)))
	if err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDecodeRequestRejectsTrailingDocument(t *testing.T) {
	_, err := DecodeRequest(strings.NewReader("{\"version\":1,\"args\":[\"jobs\"]}\n{}"))
	if err == nil || !strings.Contains(err.Error(), "trailing data") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunSSHStreamsRequestAndOutput(t *testing.T) {
	argsPath, requestPath := installFakeSSH(t, 0)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command := []string{"exec", "job", "printf", "$HOME; still data"}

	if err := RunSSH(context.Background(), "dev@node", command, "/tmp/ssh-config", &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	if stdout.String() != "remote stdout\n" || stderr.String() != "remote stderr\n" {
		t.Fatalf("unexpected streams: stdout=%q stderr=%q", stdout.String(), stderr.String())
	}

	encoded, err := os.ReadFile(requestPath)
	if err != nil {
		t.Fatal(err)
	}
	request, err := DecodeRequest(bytes.NewReader(encoded))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(request.Args, command) {
		t.Fatalf("request arguments changed: %#v", request.Args)
	}

	encodedArgs, err := os.ReadFile(argsPath)
	if err != nil {
		t.Fatal(err)
	}
	wantArgs := "-T\n-F\n/tmp/ssh-config\ndev@node\nspare\nrpc\n"
	if string(encodedArgs) != wantArgs {
		t.Fatalf("unexpected SSH arguments: %q", encodedArgs)
	}
}

func TestRunSSHPreservesRemoteExitStatus(t *testing.T) {
	installFakeSSH(t, 17)
	err := RunSSH(context.Background(), "node", []string{"jobs"}, "", bytes.NewBuffer(nil), bytes.NewBuffer(nil))
	var exitError *exec.ExitError
	if !errors.As(err, &exitError) || exitError.ExitCode() != 17 {
		t.Fatalf("unexpected error: %v", err)
	}
}

func installFakeSSH(t *testing.T, exitCode int) (string, string) {
	t.Helper()
	directory := t.TempDir()
	sshPath := filepath.Join(directory, "ssh")
	argsPath := filepath.Join(directory, "args")
	requestPath := filepath.Join(directory, "request")
	script := `#!/bin/sh
printf '%s\n' "$@" > "$SPARENODE_TEST_ARGS"
cat > "$SPARENODE_TEST_REQUEST"
printf 'remote stdout\n'
printf 'remote stderr\n' >&2
exit "$SPARENODE_TEST_EXIT"
`
	if err := os.WriteFile(sshPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", directory+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("SPARENODE_TEST_ARGS", argsPath)
	t.Setenv("SPARENODE_TEST_REQUEST", requestPath)
	t.Setenv("SPARENODE_TEST_EXIT", strconv.Itoa(exitCode))
	return argsPath, requestPath
}
