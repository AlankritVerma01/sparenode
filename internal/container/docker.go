package container

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
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
	Workspace string
	Command   []string
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

func List(ctx context.Context, runner execx.Runner) (string, error) {
	output, err := runner.Run(ctx, "docker", "ps", "--all", "--filter", "label="+managedLabel,
		"--format", "{{.ID}}\t{{.Names}}\t{{.Status}}\t{{.Image}}")
	if err != nil {
		return "", fmt.Errorf("list jobs: %s: %w", output, err)
	}
	return output, nil
}

func Logs(ctx context.Context, runner execx.Runner, name string) (string, error) {
	id, err := managedContainerID(ctx, runner, name)
	if err != nil {
		return "", err
	}
	output, err := runner.Run(ctx, "docker", "logs", id)
	if err != nil {
		return "", fmt.Errorf("read job logs: %s: %w", output, err)
	}
	return output, nil
}

func Exec(ctx context.Context, runner execx.Runner, name string, command []string) (string, error) {
	if len(command) == 0 {
		return "", fmt.Errorf("exec requires a command")
	}
	id, err := managedContainerID(ctx, runner, name)
	if err != nil {
		return "", err
	}
	args := append([]string{"exec", id}, command...)
	output, err := runner.Run(ctx, "docker", args...)
	if err != nil {
		return "", fmt.Errorf("execute in job: %s: %w", output, err)
	}
	return output, nil
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
