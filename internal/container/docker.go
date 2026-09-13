package container

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/AlankritVerma01/sparenode/internal/execx"
)

const managedLabel = "dev.sparenode.managed=true"

var validName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,62}$`)
var validContainerID = regexp.MustCompile(`^[a-f0-9]{12,64}$`)

type Job struct {
	Name      string
	Image     string
	GPU       bool
	CPUs      string
	Memory    string
	ShmSize   string
	User      string
	Workspace string
	Env       []string
	Publish   []string
	Command   []string
}

type Summary struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
	Image  string `json:"image"`
}

func BuildRunArgs(job Job) ([]string, error) {
	if !validName.MatchString(job.Name) {
		return nil, fmt.Errorf("invalid job name %q", job.Name)
	}
	if strings.TrimSpace(job.Image) == "" || strings.HasPrefix(job.Image, "-") {
		return nil, fmt.Errorf("invalid container image %q", job.Image)
	}

	args := []string{"run", "--detach", "--pull", "missing", "--name", job.Name, "--label", managedLabel}
	if job.CPUs != "" {
		args = append(args, "--cpus", job.CPUs)
	}
	if job.Memory != "" {
		args = append(args, "--memory", job.Memory)
	}
	if job.ShmSize != "" {
		args = append(args, "--shm-size", job.ShmSize)
	}
	if job.User != "" {
		args = append(args, "--user", job.User)
	}
	for _, environment := range job.Env {
		args = append(args, "--env", environment)
	}
	for _, publish := range job.Publish {
		args = append(args, "--publish", "127.0.0.1:"+publish)
	}
	if job.GPU {
		args = append(args, "--gpus", "all", "--label", "dev.sparenode.gpu=true")
	}
	if job.Workspace != "" {
		if !filepath.IsAbs(job.Workspace) {
			return nil, fmt.Errorf("workspace path must be absolute: %q", job.Workspace)
		}
		if strings.Contains(job.Workspace, ",") {
			return nil, fmt.Errorf("workspace path cannot contain a comma: %q", job.Workspace)
		}
		workspace := filepath.Clean(job.Workspace)
		args = append(args, "--mount", "type=bind,source="+workspace+",target=/workspace", "--workdir", "/workspace")
	}
	args = append(args, job.Image)
	args = append(args, job.Command...)
	return args, nil
}

func Start(ctx context.Context, runner execx.Runner, job Job) (string, error) {
	args, err := BuildRunArgs(job)
	if err != nil {
		return "", err
	}
	output, err := runner.Run(ctx, "docker", args...)
	if err != nil {
		return "", fmt.Errorf("start job: %s: %w", output, err)
	}
	id, err := ParseContainerID(output)
	if err != nil {
		return "", err
	}
	return id, nil
}

func ParseContainerID(output string) (string, error) {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	for index := len(lines) - 1; index >= 0; index-- {
		candidate := strings.TrimSpace(lines[index])
		if validContainerID.MatchString(candidate) {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("Docker did not return a container ID")
}

func List(ctx context.Context, runner execx.Runner) ([]Summary, error) {
	output, err := runner.Run(ctx, "docker", "ps", "--all", "--filter", "label="+managedLabel,
		"--format", "{{.ID}}\t{{.Names}}\t{{.Status}}\t{{.Image}}")
	if err != nil {
		return nil, fmt.Errorf("list jobs: %s: %w", output, err)
	}
	if output == "" {
		return []Summary{}, nil
	}

	lines := strings.Split(output, "\n")
	jobs := make([]Summary, 0, len(lines))
	for _, line := range lines {
		fields := strings.SplitN(line, "\t", 4)
		if len(fields) != 4 {
			return nil, fmt.Errorf("Docker returned invalid job list data")
		}
		jobs = append(jobs, Summary{ID: fields[0], Name: fields[1], Status: fields[2], Image: fields[3]})
	}
	return jobs, nil
}

func Logs(ctx context.Context, runner execx.Runner, name, tail string) (string, error) {
	if err := validateLogTail(tail); err != nil {
		return "", err
	}
	id, err := managedContainerID(ctx, runner, name)
	if err != nil {
		return "", err
	}
	logArgs := buildLogArgs(id, tail, false)
	output, err := runner.Run(ctx, "docker", logArgs...)
	if err != nil {
		return "", fmt.Errorf("read job logs: %s: %w", output, err)
	}
	return output, nil
}

func FollowLogs(ctx context.Context, runner execx.StreamingRunner, name, tail string, stdout, stderr io.Writer) error {
	if err := validateLogTail(tail); err != nil {
		return err
	}
	id, err := managedContainerID(ctx, runner, name)
	if err != nil {
		return err
	}
	logArgs := buildLogArgs(id, tail, true)
	if err := runner.Stream(ctx, stdout, stderr, "docker", logArgs...); err != nil {
		return fmt.Errorf("follow job logs: %w", err)
	}
	return nil
}

func validateLogTail(tail string) error {
	if tail != "all" {
		lines, err := strconv.Atoi(tail)
		if err != nil || lines < 0 {
			return fmt.Errorf("invalid log tail %q: use a non-negative number or all", tail)
		}
	}
	return nil
}

func buildLogArgs(id, tail string, follow bool) []string {
	args := []string{"logs", "--tail", tail}
	if follow {
		args = append(args, "--follow")
	}
	return append(args, id)
}

func StreamExec(ctx context.Context, runner execx.StreamingRunner, name string, command []string, stdout, stderr io.Writer) error {
	if len(command) == 0 {
		return fmt.Errorf("exec requires a command")
	}
	id, err := managedContainerID(ctx, runner, name)
	if err != nil {
		return err
	}
	args := append([]string{"exec", id}, command...)
	if err := runner.Stream(ctx, stdout, stderr, "docker", args...); err != nil {
		return fmt.Errorf("execute in job: %w", err)
	}
	return nil
}

func Wait(ctx context.Context, runner execx.Runner, name string) (int, error) {
	id, err := managedContainerID(ctx, runner, name)
	if err != nil {
		return 0, err
	}
	output, err := runner.Run(ctx, "docker", "wait", id)
	if err != nil {
		return 0, fmt.Errorf("wait for job: %s: %w", output, err)
	}
	exitCode, err := strconv.Atoi(strings.TrimSpace(output))
	if err != nil || exitCode < 0 || exitCode > 255 {
		return 0, fmt.Errorf("Docker returned invalid exit code %q", output)
	}
	return exitCode, nil
}

func Stop(ctx context.Context, runner execx.Runner, name string) error {
	id, err := managedContainerID(ctx, runner, name)
	if err != nil {
		return err
	}
	output, err := runner.Run(ctx, "docker", "stop", "--time", "10", id)
	if err != nil {
		return fmt.Errorf("stop job: %s: %w", output, err)
	}
	return nil
}

func Remove(ctx context.Context, runner execx.Runner, name string) error {
	id, err := managedContainerID(ctx, runner, name)
	if err != nil {
		return err
	}
	output, err := runner.Run(ctx, "docker", "rm", id)
	if err != nil {
		return fmt.Errorf("remove job: %s: %w", output, err)
	}
	return nil
}

func managedContainerID(ctx context.Context, runner execx.Runner, name string) (string, error) {
	if !validName.MatchString(name) {
		return "", fmt.Errorf("invalid job name %q", name)
	}
	output, err := runner.Run(ctx, "docker", "inspect", "--format", `{{.Id}}	{{index .Config.Labels "dev.sparenode.managed"}}`, name)
	if err != nil {
		return "", fmt.Errorf("inspect job: %s: %w", output, err)
	}
	fields := strings.Fields(output)
	if len(fields) != 2 || !validContainerID.MatchString(fields[0]) {
		return "", fmt.Errorf("Docker returned invalid metadata for container %q", name)
	}
	if fields[1] != "true" {
		return "", fmt.Errorf("container %q is not managed by SpareNode", name)
	}
	return fields[0], nil
}
