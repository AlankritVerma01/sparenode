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
	runner := &fakeRunner{output: "abc\tjob\tUp 2 minutes\talpine:latest\ndef\tdone\tExited (0)\tubuntu:24.04"}
	jobs, err := List(context.Background(), runner)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"ps", "--all", "--filter", "label=dev.sparenode.managed=true", "--format", "{{.ID}}\t{{.Names}}\t{{.Status}}\t{{.Image}}"}
	wantJobs := []Summary{
		{ID: "abc", Name: "job", Status: "Up 2 minutes", Image: "alpine:latest"},
		{ID: "def", Name: "done", Status: "Exited (0)", Image: "ubuntu:24.04"},
	}
	if !reflect.DeepEqual(jobs, wantJobs) || !reflect.DeepEqual(runner.args, want) {
		t.Fatalf("unexpected list call: jobs=%#v args=%#v", jobs, runner.args)
	}
}

func TestListReturnsEmptySlice(t *testing.T) {
	jobs, err := List(context.Background(), &fakeRunner{})
	if err != nil {
		t.Fatal(err)
	}
	if jobs == nil || len(jobs) != 0 {
		t.Fatalf("expected a non-nil empty list, got %#v", jobs)
	}
}

func TestListRejectsMalformedDockerOutput(t *testing.T) {
	if _, err := List(context.Background(), &fakeRunner{output: "missing fields"}); err == nil {
		t.Fatal("expected an error")
	}
}

func TestRemoveRefusesUnmanagedContainer(t *testing.T) {
	id := "1b2e6485a0f717036eb2172b8549d8ab3a34e531c9a29c5102a47370843c0aab"
	runner := &scriptedRunner{results: []runResult{{output: id + " false"}}}
	err := Remove(context.Background(), runner, "database")
	if err == nil || err.Error() != `container "database" is not managed by SpareNode` {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(runner.calls) != 1 || runner.calls[0][1] != "inspect" {
		t.Fatalf("remove should stop after inspection: %#v", runner.calls)
	}
}

func TestRemoveManagedContainer(t *testing.T) {
	id := "1b2e6485a0f717036eb2172b8549d8ab3a34e531c9a29c5102a47370843c0aab"
	runner := &scriptedRunner{results: []runResult{{output: id + " true"}, {output: "job"}}}
	if err := Remove(context.Background(), runner, "job"); err != nil {
		t.Fatal(err)
	}
	if len(runner.calls) != 2 || runner.calls[1][1] != "rm" || runner.calls[1][2] != id {
		t.Fatalf("unexpected calls: %#v", runner.calls)
	}
}

func TestLogsManagedContainer(t *testing.T) {
	id := "1b2e6485a0f717036eb2172b8549d8ab3a34e531c9a29c5102a47370843c0aab"
	runner := &scriptedRunner{results: []runResult{{output: id + " true"}, {output: "GPU ready"}}}
	output, err := Logs(context.Background(), runner, "job")
	if err != nil {
		t.Fatal(err)
	}
	if output != "GPU ready" || len(runner.calls) != 2 || runner.calls[1][1] != "logs" || runner.calls[1][2] != id {
		t.Fatalf("unexpected result: output=%q calls=%#v", output, runner.calls)
	}
}

func TestExecManagedContainerWithoutShellExpansion(t *testing.T) {
	id := "1b2e6485a0f717036eb2172b8549d8ab3a34e531c9a29c5102a47370843c0aab"
	runner := &scriptedRunner{results: []runResult{{output: id + " true"}, {output: "$HOME; still data"}}}
	command := []string{"printf", "%s", "$HOME; rm -rf /"}
	output, err := Exec(context.Background(), runner, "job", command)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"docker", "exec", id, "printf", "%s", "$HOME; rm -rf /"}
	if output != "$HOME; still data" || len(runner.calls) != 2 || !reflect.DeepEqual(runner.calls[1], want) {
		t.Fatalf("unexpected result: output=%q calls=%#v", output, runner.calls)
	}
}

func TestExecRequiresCommandBeforeInspecting(t *testing.T) {
	runner := &scriptedRunner{}
	if _, err := Exec(context.Background(), runner, "job", nil); err == nil {
		t.Fatal("expected an error")
	}
	if len(runner.calls) != 0 {
		t.Fatalf("exec should not inspect without a command: %#v", runner.calls)
	}
}

func TestExecRefusesUnmanagedContainer(t *testing.T) {
	id := "1b2e6485a0f717036eb2172b8549d8ab3a34e531c9a29c5102a47370843c0aab"
	runner := &scriptedRunner{results: []runResult{{output: id + " false"}}}
	if _, err := Exec(context.Background(), runner, "database", []string{"true"}); err == nil {
		t.Fatal("expected an error")
	}
	if len(runner.calls) != 1 {
		t.Fatalf("exec should stop after inspection: %#v", runner.calls)
	}
}

func TestWaitReturnsContainerExitCode(t *testing.T) {
	id := "1b2e6485a0f717036eb2172b8549d8ab3a34e531c9a29c5102a47370843c0aab"
	runner := &scriptedRunner{results: []runResult{{output: id + " true"}, {output: "17\n"}}}
	exitCode, err := Wait(context.Background(), runner, "job")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"docker", "wait", id}
	if exitCode != 17 || len(runner.calls) != 2 || !reflect.DeepEqual(runner.calls[1], want) {
		t.Fatalf("unexpected result: exitCode=%d calls=%#v", exitCode, runner.calls)
	}
}

func TestWaitRejectsInvalidExitCode(t *testing.T) {
	id := "1b2e6485a0f717036eb2172b8549d8ab3a34e531c9a29c5102a47370843c0aab"
	runner := &scriptedRunner{results: []runResult{{output: id + " true"}, {output: "unknown"}}}
	if _, err := Wait(context.Background(), runner, "job"); err == nil {
		t.Fatal("expected an error")
	}
}

func TestLifecycleRejectsInvalidInspectMetadata(t *testing.T) {
	runner := &scriptedRunner{results: []runResult{{output: "not-an-id true"}}}
	if _, err := Logs(context.Background(), runner, "job"); err == nil {
		t.Fatal("expected an error")
	}
}
