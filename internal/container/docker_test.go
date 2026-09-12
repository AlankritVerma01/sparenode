package container

import (
	"reflect"
	"testing"
)

func TestBuildRunArgsGPUJob(t *testing.T) {
	args, err := BuildRunArgs(Job{
		Name: "cuda-test", Image: "ubuntu:24.04", GPU: true,
		Command: []string{"nvidia-smi", "-L"},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"run", "--detach", "--pull", "missing", "--name", "cuda-test",
		"--label", "dev.sparenode.managed=true", "--gpus", "all",
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
