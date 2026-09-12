package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/AlankritVerma01/sparenode/internal/container"
	"github.com/AlankritVerma01/sparenode/internal/doctor"
	"github.com/AlankritVerma01/sparenode/internal/execx"
	"github.com/AlankritVerma01/sparenode/internal/remote"
)

const version = "0.1.0-dev"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) > 0 && args[0] == "rpc" {
		request, err := remote.DecodeRequest(os.Stdin)
		if err != nil {
			return err
		}
		return runLocal(request.Args)
	}

	host, localArgs, err := remote.SplitHost(args, os.Getenv("SPARENODE_HOST"))
	if err != nil {
		return err
	}
	if host != "" {
		if len(localArgs) == 0 {
			return errors.New("a command is required")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		return remote.RunSSH(ctx, host, localArgs, os.Getenv("SPARENODE_SSH_CONFIG"), os.Stdout, os.Stderr)
	}
	return runLocal(localArgs)
}

func runLocal(args []string) error {
	if len(args) == 0 {
		usage()
		return nil
	}

	runner := execx.OSRunner{}

	switch args[0] {
	case "doctor":
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		return runDoctor(ctx, runner, args[1:])
	case "run":
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		return runJob(ctx, runner, args[1:])
	case "jobs":
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		output, err := container.List(ctx, runner)
		if err != nil {
			return err
		}
		if output == "" {
			fmt.Println("No SpareNode jobs.")
		} else {
			fmt.Println("ID\tNAME\tSTATUS\tIMAGE")
			fmt.Println(output)
		}
		return nil
	case "logs":
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		name, err := requireJobName("logs", args[1:])
		if err != nil {
			return err
		}
		output, err := container.Logs(ctx, runner, name)
		if err != nil {
			return err
		}
		fmt.Println(output)
		return nil
	case "stop":
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		name, err := requireJobName("stop", args[1:])
		if err != nil {
			return err
		}
		if err := container.Stop(ctx, runner, name); err != nil {
			return err
		}
		fmt.Printf("Stopped %s\n", name)
		return nil
	case "remove":
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		name, err := requireJobName("remove", args[1:])
		if err != nil {
			return err
		}
		if err := container.Remove(ctx, runner, name); err != nil {
			return err
		}
		fmt.Printf("Removed %s\n", name)
		return nil
	case "version", "--version", "-v":
		fmt.Println(version)
		return nil
	case "help", "--help", "-h":
		usage()
		return nil
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func requireJobName(command string, args []string) (string, error) {
	if len(args) != 1 {
		return "", fmt.Errorf("%s requires exactly one job name", command)
	}
	return args[0], nil
}

func runDoctor(ctx context.Context, runner execx.Runner, args []string) error {
	flags := flag.NewFlagSet("doctor", flag.ContinueOnError)
	jsonOutput := flags.Bool("json", false, "emit JSON")
	dataPath := flags.String("data-path", "", "inspect a persistent data path")
	if err := flags.Parse(args); err != nil {
		return err
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
				fmt.Printf("      %s\n", check.Detail)
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

func runJob(ctx context.Context, runner execx.Runner, args []string) error {
	flags := flag.NewFlagSet("run", flag.ContinueOnError)
	name := flags.String("name", "", "unique job name")
	image := flags.String("image", "", "container image")
	gpu := flags.Bool("gpu", false, "attach all NVIDIA GPUs")
	workspace := flags.String("workspace", "", "host directory mounted at /workspace")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *name == "" || *image == "" {
		return errors.New("run requires --name and --image")
	}
	id, err := container.Start(ctx, runner, container.Job{
		Name: *name, Image: *image, GPU: *gpu, Workspace: *workspace, Command: flags.Args(),
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
  spare run --name NAME --image IMAGE [--gpu] [--workspace PATH] [COMMAND...]
  spare jobs
  spare logs NAME
  spare stop NAME
  spare remove NAME
  spare version
`)
}
