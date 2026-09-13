package doctor

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/AlankritVerma01/sparenode/internal/execx"
)

type Status string

const (
	Pass Status = "pass"
	Warn Status = "warn"
	Fail Status = "fail"
)

type Check struct {
	Name    string `json:"name"`
	Status  Status `json:"status"`
	Summary string `json:"summary"`
	Detail  string `json:"detail,omitempty"`
}

type GPU struct {
	Index     int    `json:"index"`
	Name      string `json:"name"`
	UUID      string `json:"uuid"`
	MemoryMiB int    `json:"memory_mib"`
	Driver    string `json:"driver"`
}

type Storage struct {
	Path           string `json:"path"`
	SizeBytes      uint64 `json:"size_bytes"`
	AvailableBytes uint64 `json:"available_bytes"`
}

type Report struct {
	SchemaVersion int       `json:"schema_version"`
	GeneratedAt   time.Time `json:"generated_at"`
	Hostname      string    `json:"hostname"`
	OS            string    `json:"os"`
	Architecture  string    `json:"architecture"`
	Checks        []Check   `json:"checks"`
	GPUs          []GPU     `json:"gpus,omitempty"`
	Storage       *Storage  `json:"storage,omitempty"`
}

type Options struct {
	DataPath   string
	RequireGPU bool
}

func Run(ctx context.Context, runner execx.Runner, options Options) Report {
	hostname, _ := os.Hostname()
	report := Report{
		SchemaVersion: 1,
		GeneratedAt:   time.Now().UTC(),
		Hostname:      hostname,
		OS:            runtime.GOOS,
		Architecture:  runtime.GOARCH,
	}

	if runtime.GOOS == "linux" {
		report.Checks = append(report.Checks, Check{Name: "operating-system", Status: Pass, Summary: "Linux host supported"})
	} else {
		report.Checks = append(report.Checks, Check{Name: "operating-system", Status: Fail, Summary: "Node mode currently requires Linux"})
	}

	dockerReady := false
	if _, err := runner.LookPath("docker"); err != nil {
		report.Checks = append(report.Checks, Check{Name: "docker", Status: Fail, Summary: "Docker CLI not found"})
	} else if output, err := runner.Run(ctx, "docker", "version", "--format", "{{.Server.Version}}"); err != nil {
		summary := "Docker daemon unavailable"
		detail := output
		if strings.Contains(strings.ToLower(output), "permission denied") {
			summary = "Docker access denied"
			detail = strings.TrimSpace(output)
			if detail != "" {
				detail += "\n"
			}
			detail += "Docker access grants effective root privileges; grant it only to a trusted node owner. See https://github.com/AlankritVerma01/sparenode/blob/main/docs/node-setup.md"
		}
		report.Checks = append(report.Checks, Check{Name: "docker", Status: Fail, Summary: summary, Detail: detail})
	} else {
		dockerReady = true
		report.Checks = append(report.Checks, Check{Name: "docker", Status: Pass, Summary: "Docker daemon " + output})
	}

	if _, err := runner.LookPath("nvidia-smi"); err != nil {
		status := Warn
		summary := "No NVIDIA GPU tooling detected"
		if options.RequireGPU {
			status = Fail
			summary = "Required NVIDIA GPU tooling not detected"
		}
		report.Checks = append(report.Checks, Check{Name: "nvidia", Status: status, Summary: summary})
	} else {
		output, err := runner.Run(ctx, "nvidia-smi", "--query-gpu=index,name,uuid,memory.total,driver_version", "--format=csv,noheader,nounits")
		if err != nil {
			report.Checks = append(report.Checks, Check{Name: "nvidia", Status: Fail, Summary: "NVIDIA driver is not responding", Detail: output})
		} else if gpus, err := ParseNvidiaSMI(output); err != nil {
			report.Checks = append(report.Checks, Check{Name: "nvidia", Status: Fail, Summary: "Could not parse NVIDIA inventory", Detail: err.Error()})
		} else {
			report.GPUs = gpus
			report.Checks = append(report.Checks, Check{Name: "nvidia", Status: Pass, Summary: fmt.Sprintf("%d NVIDIA GPU(s) ready", len(gpus))})
		}
	}

	if len(report.GPUs) > 0 {
		if _, err := runner.LookPath("nvidia-container-cli"); err != nil {
			report.Checks = append(report.Checks, Check{Name: "gpu-containers", Status: Fail, Summary: "NVIDIA Container Toolkit not detected"})
		} else if !dockerReady {
			report.Checks = append(report.Checks, Check{Name: "gpu-containers", Status: Warn, Summary: "GPU container runtime could not be checked without Docker access"})
		} else {
			output, err := runner.Run(ctx, "docker", "info", "--format", "{{json .Runtimes}}")
			if err != nil {
				report.Checks = append(report.Checks, Check{Name: "gpu-containers", Status: Fail, Summary: "Could not inspect Docker runtimes", Detail: output})
			} else if configured, err := HasNvidiaRuntime(output); err != nil {
				report.Checks = append(report.Checks, Check{Name: "gpu-containers", Status: Fail, Summary: "Could not parse Docker runtimes", Detail: err.Error()})
			} else if !configured {
				report.Checks = append(report.Checks, Check{Name: "gpu-containers", Status: Fail, Summary: "Docker NVIDIA runtime is not configured"})
			} else {
				report.Checks = append(report.Checks, Check{Name: "gpu-containers", Status: Pass, Summary: "Docker NVIDIA runtime configured"})
			}
		}
	}

	if options.DataPath != "" {
		info, err := os.Stat(options.DataPath)
		if err != nil {
			report.Checks = append(report.Checks, Check{Name: "storage", Status: Fail, Summary: "Data path unavailable", Detail: err.Error()})
		} else if !info.IsDir() {
			report.Checks = append(report.Checks, Check{Name: "storage", Status: Fail, Summary: "Data path is not a directory"})
		} else if output, err := runner.Run(ctx, "df", "-P", "-B1", options.DataPath); err != nil {
			report.Checks = append(report.Checks, Check{Name: "storage", Status: Fail, Summary: "Data path unavailable", Detail: output})
		} else if storage, err := parseDF(options.DataPath, output); err != nil {
			report.Checks = append(report.Checks, Check{Name: "storage", Status: Fail, Summary: "Could not inspect data path", Detail: err.Error()})
		} else {
			report.Storage = &storage
			report.Checks = append(report.Checks, Check{Name: "storage", Status: Pass, Summary: "Data path available"})
		}
	}

	return report
}

func HasNvidiaRuntime(input string) (bool, error) {
	var runtimes map[string]json.RawMessage
	if err := json.Unmarshal([]byte(input), &runtimes); err != nil {
		return false, err
	}
	_, exists := runtimes["nvidia"]
	return exists, nil
}

func (r Report) Healthy() bool {
	for _, check := range r.Checks {
		if check.Status == Fail {
			return false
		}
	}
	return true
}

func ParseNvidiaSMI(input string) ([]GPU, error) {
	rows, err := csv.NewReader(strings.NewReader(input)).ReadAll()
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("empty nvidia-smi output")
	}

	gpus := make([]GPU, 0, len(rows))
	for _, row := range rows {
		if len(row) != 5 {
			return nil, fmt.Errorf("expected 5 fields, got %d", len(row))
		}
		index, err := strconv.Atoi(strings.TrimSpace(row[0]))
		if err != nil {
			return nil, fmt.Errorf("invalid GPU index: %w", err)
		}
		memory, err := strconv.Atoi(strings.TrimSpace(row[3]))
		if err != nil {
			return nil, fmt.Errorf("invalid GPU memory: %w", err)
		}
		gpus = append(gpus, GPU{
			Index: index, Name: strings.TrimSpace(row[1]), UUID: strings.TrimSpace(row[2]),
			MemoryMiB: memory, Driver: strings.TrimSpace(row[4]),
		})
	}
	return gpus, nil
}

func parseDF(path, output string) (Storage, error) {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) < 2 {
		return Storage{}, fmt.Errorf("unexpected df output")
	}
	fields := strings.Fields(lines[len(lines)-1])
	if len(fields) < 6 {
		return Storage{}, fmt.Errorf("unexpected df fields")
	}
	size, err := strconv.ParseUint(fields[1], 10, 64)
	if err != nil {
		return Storage{}, fmt.Errorf("invalid storage size: %w", err)
	}
	available, err := strconv.ParseUint(fields[3], 10, 64)
	if err != nil {
		return Storage{}, fmt.Errorf("invalid available storage: %w", err)
	}
	return Storage{Path: path, SizeBytes: size, AvailableBytes: available}, nil
}
