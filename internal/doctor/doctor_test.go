package doctor

import (
	"context"
	"errors"
	"runtime"
	"strings"
	"testing"
)

type commandResult struct {
	output string
	err    error
}

type fakeRunner struct {
	paths   map[string]bool
	results map[string]commandResult
}

func (f fakeRunner) LookPath(file string) (string, error) {
	if f.paths[file] {
		return file, nil
	}
	return "", errors.New("not found")
}

func (f fakeRunner) Run(_ context.Context, name string, args ...string) (string, error) {
	result, exists := f.results[name+" "+strings.Join(args, " ")]
	if !exists {
		return "", errors.New("unexpected command")
	}
	return result.output, result.err
}

func TestParseNvidiaSMI(t *testing.T) {
	input := "0, NVIDIA GeForce GTX 1650, GPU-abc, 4096, 610.57.04"
	gpus, err := ParseNvidiaSMI(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(gpus) != 1 || gpus[0].Name != "NVIDIA GeForce GTX 1650" || gpus[0].MemoryMiB != 4096 {
		t.Fatalf("unexpected GPUs: %#v", gpus)
	}
}

func TestParseNvidiaSMIRejectsMalformedRows(t *testing.T) {
	if _, err := ParseNvidiaSMI("0, incomplete"); err == nil {
		t.Fatal("expected an error")
	}
}

func TestParseDF(t *testing.T) {
	input := "Filesystem 1-blocks Used Available Capacity Mounted on\n/dev/sda1 1000 250 750 25% /data"
	storage, err := parseDF("/data", input)
	if err != nil {
		t.Fatal(err)
	}
	if storage.SizeBytes != 1000 || storage.AvailableBytes != 750 {
		t.Fatalf("unexpected storage: %#v", storage)
	}
}

func TestHasNvidiaRuntime(t *testing.T) {
	configured, err := HasNvidiaRuntime(`{"runc":{"path":"runc"},"nvidia":{"path":"nvidia-container-runtime"}}`)
	if err != nil {
		t.Fatal(err)
	}
	if !configured {
		t.Fatal("expected NVIDIA runtime")
	}
}

func TestHasNvidiaRuntimeMissing(t *testing.T) {
	configured, err := HasNvidiaRuntime(`{"runc":{"path":"runc"}}`)
	if err != nil {
		t.Fatal(err)
	}
	if configured {
		t.Fatal("did not expect NVIDIA runtime")
	}
}

func TestHasNvidiaRuntimeRejectsMalformedJSON(t *testing.T) {
	if _, err := HasNvidiaRuntime(`not-json`); err == nil {
		t.Fatal("expected an error")
	}
}

func TestRunDoesNotRequireNvidiaToolkitOnCPUNode(t *testing.T) {
	runner := fakeRunner{
		paths: map[string]bool{"docker": true},
		results: map[string]commandResult{
			"docker version --format {{.Server.Version}}": {output: "29.0"},
		},
	}
	report := Run(context.Background(), runner, Options{})
	for _, check := range report.Checks {
		if check.Name == "gpu-containers" {
			t.Fatalf("unexpected GPU container check: %#v", check)
		}
	}
	if runtime.GOOS == "linux" && !report.Healthy() {
		t.Fatalf("CPU-only node should be healthy: %#v", report.Checks)
	}
}

func TestRunDistinguishesDockerPermissionFailure(t *testing.T) {
	runner := fakeRunner{
		paths: map[string]bool{"docker": true},
		results: map[string]commandResult{
			"docker version --format {{.Server.Version}}": {output: "permission denied while connecting", err: errors.New("exit 1")},
		},
	}
	report := Run(context.Background(), runner, Options{})
	if report.Checks[1].Summary != "Docker access denied" {
		t.Fatalf("unexpected Docker result: %#v", report.Checks[1])
	}
	if !strings.Contains(report.Checks[1].Detail, "effective root privileges") || !strings.Contains(report.Checks[1].Detail, "node-setup.md") {
		t.Fatalf("permission failure should explain the security boundary: %#v", report.Checks[1])
	}
}

func TestRunCanRequireGPU(t *testing.T) {
	runner := fakeRunner{
		paths: map[string]bool{"docker": true},
		results: map[string]commandResult{
			"docker version --format {{.Server.Version}}": {output: "29.0"},
		},
	}
	report := Run(context.Background(), runner, Options{RequireGPU: true})
	if report.Healthy() {
		t.Fatal("required GPU failure should make the report unhealthy")
	}
	for _, check := range report.Checks {
		if check.Name == "nvidia" {
			if check.Status != Fail || check.Summary != "Required NVIDIA GPU tooling not detected" {
				t.Fatalf("unexpected NVIDIA result: %#v", check)
			}
			return
		}
	}
	t.Fatal("NVIDIA check missing")
}
