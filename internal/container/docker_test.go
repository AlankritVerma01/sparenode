package container

import (
	"reflect"
	"testing"
)

func TestBuildRunArgsGPUJob(t *testing.T) {
	args, err := BuildRunArgs(Job{
		Name: "cuda-test", Image: "ubuntu:24.04", GPU: true, CPUs: "2.5", Memory: "4g", ShmSize: "1g",
		User: "1000:1000",
		Env:  []string{"MODE=test", "TOKEN"}, Publish: []string{"3000:3000", "8080:80"},
		Command: []string{"nvidia-smi", "-L"},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"run", "--detach", "--pull", "missing", "--name", "cuda-test",
		"--label", "dev.sparenode.managed=true", "--cpus", "2.5", "--memory", "4g",
		"--shm-size", "1g",
		"--user", "1000:1000",
		"--env", "MODE=test", "--env", "TOKEN",
		"--publish", "127.0.0.1:3000:3000", "--publish", "127.0.0.1:8080:80",
		"--gpus", "all",
		"--label", "dev.sparenode.gpu=true", "ubuntu:24.04", "nvidia-smi", "-L",
	}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("args mismatch\n got: %#v\nwant: %#v", args, want)
	}
}

func TestBuildRunArgsRejectsUnsafeName(t *testing.T) {
	_, err := BuildRunArgs(Job{Name: "bad name", Image: "ubuntu"})
	if err == nil {
		t.Fatal("expected an error")
	}
}

func TestBuildRunArgsRejectsFlagAsImage(t *testing.T) {
	_, err := BuildRunArgs(Job{Name: "job", Image: "--privileged"})
	if err == nil {
		t.Fatal("expected an error")
	}
}

func TestBuildRunArgsMountsAbsoluteWorkspace(t *testing.T) {
	args, err := BuildRunArgs(Job{Name: "dev", Image: "ubuntu", Workspace: "/srv/projects/app/../app"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"run", "--detach", "--pull", "missing", "--name", "dev",
		"--label", "dev.sparenode.managed=true",
		"--mount", "type=bind,source=/srv/projects/app,target=/workspace",
		"--workdir", "/workspace", "ubuntu",
	}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("args mismatch\n got: %#v\nwant: %#v", args, want)
	}
}

func TestBuildRunArgsRejectsAmbiguousWorkspace(t *testing.T) {
	for _, workspace := range []string{"relative/path", "/srv/project,readonly"} {
		t.Run(workspace, func(t *testing.T) {
			if _, err := BuildRunArgs(Job{Name: "dev", Image: "ubuntu", Workspace: workspace}); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}
