package container

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

type fakeRunner struct {
	name   string
	args   []string
	output string
	err    error
}

type runResult struct {
	output string
	err    error
}

type scriptedRunner struct {
	results []runResult
	calls   [][]string
}

func (s *scriptedRunner) LookPath(file string) (string, error) { return file, nil }

func (s *scriptedRunner) Run(_ context.Context, name string, args ...string) (string, error) {
	s.calls = append(s.calls, append([]string{name}, args...))
	if len(s.results) == 0 {
		return "", errors.New("unexpected call")
	}
	result := s.results[0]
	s.results = s.results[1:]
	return result.output, result.err
}

func (f *fakeRunner) LookPath(file string) (string, error) { return file, nil }

func (f *fakeRunner) Run(_ context.Context, name string, args ...string) (string, error) {
	f.name = name
	f.args = append([]string(nil), args...)
	return f.output, f.err
}

func TestStartUsesDockerWithoutShellExpansion(t *testing.T) {
	containerID := "1b2e6485a0f717036eb2172b8549d8ab3a34e531c9a29c5102a47370843c0aab"
	runner := &fakeRunner{output: "Pulling image\n" + containerID}
	id, err := Start(context.Background(), runner, Job{
		Name: "safe-job", Image: "alpine", Command: []string{"printf", "$HOME; rm -rf /"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if id != containerID || runner.name != "docker" {
		t.Fatalf("unexpected result: id=%q command=%q", id, runner.name)
	}
	if got := runner.args[len(runner.args)-1]; got != "$HOME; rm -rf /" {
		t.Fatalf("argument was changed: %q", got)
	}
}

func TestParseContainerIDRejectsUnexpectedOutput(t *testing.T) {
	if _, err := ParseContainerID("container started maybe"); err == nil {
		t.Fatal("expected an error")
	}
}

func TestStartIncludesRunnerFailure(t *testing.T) {
	runner := &fakeRunner{output: "daemon unavailable", err: errors.New("exit 1")}
	_, err := Start(context.Background(), runner, Job{Name: "job", Image: "alpine"})
	if err == nil || err.Error() != "start job: daemon unavailable: exit 1" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestListSelectsOnlyManagedContainers(t *testing.T) {
	runner := &fakeRunner{output: "abc\tjob\tUp\talpine"}
	output, err := List(context.Background(), runner)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"ps", "--all", "--filter", "label=dev.sparenode.managed=true", "--format", "{{.ID}}\t{{.Names}}\t{{.Status}}\t{{.Image}}"}
	if output != runner.output || !reflect.DeepEqual(runner.args, want) {
		t.Fatalf("unexpected list call: output=%q args=%#v", output, runner.args)
	}
}

func TestRemoveRefusesUnmanagedContainer(t *testing.T) {
	runner := &scriptedRunner{results: []runResult{{output: ""}}}
	err := Remove(context.Background(), runner, "database")
	if err == nil || err.Error() != `container "database" is not managed by SpareNode` {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(runner.calls) != 1 || runner.calls[0][1] != "inspect" {
		t.Fatalf("remove should stop after inspection: %#v", runner.calls)
	}
}

func TestRemoveManagedContainer(t *testing.T) {
	runner := &scriptedRunner{results: []runResult{{output: "true"}, {output: "job"}}}
	if err := Remove(context.Background(), runner, "job"); err != nil {
		t.Fatal(err)
	}
	if len(runner.calls) != 2 || runner.calls[1][1] != "rm" {
		t.Fatalf("unexpected calls: %#v", runner.calls)
	}
}

func TestLogsManagedContainer(t *testing.T) {
	runner := &scriptedRunner{results: []runResult{{output: "true"}, {output: "GPU ready"}}}
	output, err := Logs(context.Background(), runner, "job")
	if err != nil {
		t.Fatal(err)
	}
	if output != "GPU ready" || len(runner.calls) != 2 || runner.calls[1][1] != "logs" {
		t.Fatalf("unexpected result: output=%q calls=%#v", output, runner.calls)
	}
}
