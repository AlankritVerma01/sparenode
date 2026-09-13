package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/AlankritVerma01/sparenode/internal/container"
	"github.com/AlankritVerma01/sparenode/internal/doctor"
	"github.com/AlankritVerma01/sparenode/internal/execx"
	"github.com/AlankritVerma01/sparenode/internal/remote"
)

var (
	version = "dev"
	commit  = "unknown"
)

type stringList []string

func (values *stringList) String() string {
	return strings.Join(*values, ",")
}

func (values *stringList) Set(value string) error {
	*values = append(*values, value)
	return nil
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	if len(args) > 0 && args[0] == "rpc" {
		request, err := remote.DecodeRequest(os.Stdin)
		if err != nil {
			return err
		}
		return runLocal(ctx, request.Args)
	}

	host, localArgs, err := remote.SplitHost(args, os.Getenv("SPARENODE_HOST"))
	if err != nil {
		return err
	}
	if host != "" {
		if len(localArgs) == 0 {
			return errors.New("a command is required")
		}
		return remote.RunSSH(ctx, host, localArgs, os.Getenv("SPARENODE_SSH_CONFIG"), os.Stdout, os.Stderr)
	}
	return runLocal(ctx, localArgs)
}

func runLocal(ctx context.Context, args []string) error {
	if len(args) == 0 {
		usage()
		return nil
	}

	runner := execx.OSRunner{}

	switch args[0] {
	case "doctor":
		ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		return runDoctor(ctx, runner, args[1:])
	case "run":
		ctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
		defer cancel()
		return runJob(ctx, runner, args[1:])
	case "jobs":
		ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		return runJobs(ctx, runner, args[1:])
	case "logs":
		return runLogs(ctx, runner, args[1:])
	case "exec":
		return runExec(ctx, runner, args[1:])
	case "wait":
		return runWait(ctx, runner, args[1:])
	case "stop":
		ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		return runStop(ctx, runner, args[1:])
	case "remove":
		ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		return runRemove(ctx, runner, args[1:])
	case "version", "--version", "-v":
		return runVersion(args[1:])
	case "help", "--help", "-h":
		usage()
		return nil
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func runVersion(args []string) error {
	flags := commandFlags("version", "spare version [--json]")
	jsonOutput := flags.Bool("json", false, "emit JSON")
	help, err := parseFlags(flags, args)
	if err != nil || help {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("version does not accept positional arguments")
	}
	if *jsonOutput {
		return json.NewEncoder(os.Stdout).Encode(struct {
			Version string `json:"version"`
			Commit  string `json:"commit"`
		}{Version: version, Commit: commit})
	}
	fmt.Printf("SpareNode %s (commit %s)\n", version, commit)
	return nil
}

func requireJobName(command string, args []string) (string, error) {
	if len(args) != 1 {
		return "", fmt.Errorf("%s requires exactly one job name", command)
	}
	return args[0], nil
}

func commandFlags(name, usageLine string) *flag.FlagSet {
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	flags.Usage = func() {
		fmt.Fprintf(flags.Output(), "Usage: %s\n", usageLine)
		flags.PrintDefaults()
	}
	return flags
}

func parseFlags(flags *flag.FlagSet, args []string) (bool, error) {
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return true, nil
		}
		return false, err
	}
	return false, nil
}

func runJobs(ctx context.Context, runner execx.Runner, args []string) error {
	flags := commandFlags("jobs", "spare jobs [--json]")
	jsonOutput := flags.Bool("json", false, "emit JSON")
	help, err := parseFlags(flags, args)
	if err != nil || help {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("jobs does not accept positional arguments")
	}

	jobs, err := container.List(ctx, runner)
	if err != nil {
		return err
	}
	if *jsonOutput {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(jobs)
	}
	if len(jobs) == 0 {
		fmt.Println("No SpareNode jobs.")
		return nil
	}

	writer := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	if _, err := fmt.Fprintln(writer, "ID\tNAME\tSTATUS\tIMAGE"); err != nil {
		return err
	}
	for _, job := range jobs {
		if _, err := fmt.Fprintf(writer, "%s\t%s\t%s\t%s\n", job.ID, job.Name, job.Status, job.Image); err != nil {
			return err
		}
	}
	return writer.Flush()
}

func runLogs(ctx context.Context, runner execx.StreamingRunner, args []string) error {
	flags := commandFlags("logs", "spare logs [--follow] [--tail N|all] NAME")
	follow := flags.Bool("follow", false, "stream new log output")
	tail := flags.String("tail", "100", "number of historical lines, or all")
	help, err := parseFlags(flags, args)
	if err != nil || help {
		return err
	}
	if flags.NArg() != 1 {
		return errors.New("logs requires exactly one job name")
	}
	name := flags.Arg(0)
	if *follow {
		return container.FollowLogs(ctx, runner, name, *tail, os.Stdout, os.Stderr)
	}
	output, err := container.Logs(ctx, runner, name, *tail)
	if err != nil {
		return err
	}
	if output != "" {
		fmt.Println(output)
	}
	return nil
}

func runExec(ctx context.Context, runner execx.StreamingRunner, args []string) error {
	flags := commandFlags("exec", "spare exec NAME COMMAND...")
	help, err := parseFlags(flags, args)
	if err != nil || help {
		return err
	}
	if flags.NArg() < 2 {
		return errors.New("exec requires a job name and command")
	}
	return container.StreamExec(ctx, runner, flags.Arg(0), flags.Args()[1:], os.Stdout, os.Stderr)
}

func runWait(ctx context.Context, runner execx.Runner, args []string) error {
	flags := commandFlags("wait", "spare wait [--json] NAME")
	jsonOutput := flags.Bool("json", false, "emit JSON")
	help, err := parseFlags(flags, args)
	if err != nil || help {
		return err
	}
	if flags.NArg() != 1 {
		return errors.New("wait requires exactly one job name")
	}
	name := flags.Arg(0)
	exitCode, err := container.Wait(ctx, runner, name)
	if err != nil {
		return err
	}
	result := struct {
		Name     string `json:"name"`
		ExitCode int    `json:"exit_code"`
	}{Name: name, ExitCode: exitCode}
	if *jsonOutput {
		return json.NewEncoder(os.Stdout).Encode(result)
	}
	fmt.Printf("%s exited with code %d\n", name, exitCode)
	return nil
}

func runDoctor(ctx context.Context, runner execx.Runner, args []string) error {
	flags := commandFlags("doctor", "spare doctor [--json] [--data-path PATH]")
	jsonOutput := flags.Bool("json", false, "emit JSON")
	dataPath := flags.String("data-path", "", "inspect a persistent data path")
	help, err := parseFlags(flags, args)
	if err != nil || help {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("doctor does not accept positional arguments")
	}

	report := doctor.Run(ctx, runner, *dataPath)
	if *jsonOutput {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(report); err != nil {
			return err
		}
	} else {
		fmt.Printf("SpareNode doctor: %s (%s/%s)\n", report.Hostname, report.OS, report.Architecture)
		for _, check := range report.Checks {
			fmt.Printf("%-5s %-16s %s\n", check.Status, check.Name, check.Summary)
			if check.Detail != "" {
				fmt.Printf("      %s\n", strings.ReplaceAll(check.Detail, "\n", "\n      "))
			}
		}
		for _, gpu := range report.GPUs {
			fmt.Printf("GPU %d: %s, %d MiB, driver %s\n", gpu.Index, gpu.Name, gpu.MemoryMiB, gpu.Driver)
		}
		if report.Storage != nil {
			fmt.Printf("Storage: %s, %.1f GiB available\n", report.Storage.Path, float64(report.Storage.AvailableBytes)/(1024*1024*1024))
		}
	}
	if !report.Healthy() {
		return errors.New("one or more required checks failed")
	}
	return nil
}

func runStop(ctx context.Context, runner execx.Runner, args []string) error {
	flags := commandFlags("stop", "spare stop NAME")
	help, err := parseFlags(flags, args)
	if err != nil || help {
		return err
	}
	name, err := requireJobName("stop", flags.Args())
	if err != nil {
		return err
	}
	if err := container.Stop(ctx, runner, name); err != nil {
		return err
	}
	fmt.Printf("Stopped %s\n", name)
	return nil
}

func runRemove(ctx context.Context, runner execx.Runner, args []string) error {
	flags := commandFlags("remove", "spare remove NAME")
	help, err := parseFlags(flags, args)
	if err != nil || help {
		return err
	}
	name, err := requireJobName("remove", flags.Args())
	if err != nil {
		return err
	}
	if err := container.Remove(ctx, runner, name); err != nil {
		return err
	}
	fmt.Printf("Removed %s\n", name)
	return nil
}

func runJob(ctx context.Context, runner execx.Runner, args []string) error {
	flags := commandFlags("run", "spare run --name NAME --image IMAGE [OPTIONS] [COMMAND...]")
	name := flags.String("name", "", "unique job name")
	image := flags.String("image", "", "container image")
	gpu := flags.Bool("gpu", false, "attach all NVIDIA GPUs")
	cpus := flags.String("cpus", "", "Docker CPU limit, such as 2 or 0.5")
	memory := flags.String("memory", "", "Docker memory limit, such as 4g or 512m")
	workspace := flags.String("workspace", "", "host directory mounted at /workspace")
	var environment stringList
	var publish stringList
	flags.Var(&environment, "env", "container environment value; repeatable")
	flags.Var(&publish, "publish", "loopback port mapping HOST:CONTAINER; repeatable")
	help, err := parseFlags(flags, args)
	if err != nil || help {
		return err
	}
	if *name == "" || *image == "" {
		return errors.New("run requires --name and --image")
	}
	id, err := container.Start(ctx, runner, container.Job{
		Name: *name, Image: *image, GPU: *gpu, CPUs: *cpus, Memory: *memory,
		Workspace: *workspace, Env: environment, Publish: publish, Command: flags.Args(),
	})
	if err != nil {
		return err
	}
	fmt.Printf("Started %s (%s)\n", *name, id)
	return nil
}

func usage() {
	fmt.Print(`SpareNode turns a Linux machine into a private development and GPU node.

Usage:
  spare [--host USER@NODE] COMMAND
  spare doctor [--json] [--data-path PATH]
  spare run --name NAME --image IMAGE [--gpu] [--cpus N] [--memory SIZE] [--workspace PATH] [--env VALUE] [--publish HOST:CONTAINER] [COMMAND...]
  spare jobs [--json]
  spare logs [--follow] [--tail N|all] NAME
  spare exec NAME COMMAND...
  spare wait [--json] NAME
  spare stop NAME
  spare remove NAME
  spare version [--json]
`)
}
