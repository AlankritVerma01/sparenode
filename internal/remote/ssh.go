package remote

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"strings"
)

const (
	ProtocolVersion = 1
	maxRequestBytes = 64 * 1024
)

var validHost = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._@:%+\[\]-]{0,254}$`)

type Request struct {
	Version int      `json:"version"`
	Args    []string `json:"args"`
}

func SplitHost(args []string, environmentHost string) (string, []string, error) {
	host := strings.TrimSpace(environmentHost)
	remaining := args

	if len(args) > 0 && (args[0] == "--host" || args[0] == "-H") {
		if len(args) < 2 {
			return "", nil, fmt.Errorf("%s requires a destination", args[0])
		}
		host = args[1]
		remaining = args[2:]
	} else if len(args) > 0 && strings.HasPrefix(args[0], "--host=") {
		host = strings.TrimPrefix(args[0], "--host=")
		remaining = args[1:]
	}

	if host != "" && !validHost.MatchString(host) {
		return "", nil, fmt.Errorf("invalid SSH destination %q", host)
	}
	return host, remaining, nil
}

func EncodeRequest(args []string) ([]byte, error) {
	request := Request{Version: ProtocolVersion, Args: args}
	encoded, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("encode remote request: %w", err)
	}
	if len(encoded) > maxRequestBytes {
		return nil, fmt.Errorf("remote request exceeds %d bytes", maxRequestBytes)
	}
	return append(encoded, '\n'), nil
}

func DecodeRequest(input io.Reader) (Request, error) {
	encoded, err := io.ReadAll(io.LimitReader(input, maxRequestBytes+1))
	if err != nil {
		return Request{}, fmt.Errorf("read remote request: %w", err)
	}
	if len(encoded) > maxRequestBytes {
		return Request{}, fmt.Errorf("remote request exceeds %d bytes", maxRequestBytes)
	}

	var request Request
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		return Request{}, fmt.Errorf("decode remote request: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return Request{}, fmt.Errorf("decode remote request: trailing data")
	}
	if request.Version != ProtocolVersion {
		return Request{}, fmt.Errorf("unsupported protocol version %d", request.Version)
	}
	if len(request.Args) == 0 {
		return Request{}, fmt.Errorf("remote request has no command")
	}
	for _, argument := range request.Args {
		if strings.IndexByte(argument, 0) >= 0 {
			return Request{}, fmt.Errorf("remote request contains a null byte")
		}
	}
	return request, nil
}

func SSHCommandArgs(host, configPath string) ([]string, error) {
	if !validHost.MatchString(host) {
		return nil, fmt.Errorf("invalid SSH destination %q", host)
	}
	args := []string{"-T"}
	if configPath != "" {
		if strings.IndexByte(configPath, 0) >= 0 {
			return nil, fmt.Errorf("SSH config path contains a null byte")
		}
		args = append(args, "-F", configPath)
	}
	return append(args, host, "spare", "rpc"), nil
}

func RunSSH(ctx context.Context, host string, args []string, configPath string, stdout, stderr io.Writer) error {
	encoded, err := EncodeRequest(args)
	if err != nil {
		return err
	}
	sshArgs, err := SSHCommandArgs(host, configPath)
	if err != nil {
		return err
	}
	command := exec.CommandContext(ctx, "ssh", sshArgs...)
	command.Stdin = bytes.NewReader(encoded)
	command.Stdout = stdout
	command.Stderr = stderr
	if err := command.Run(); err != nil {
		return fmt.Errorf("SSH request to %s failed: %w", host, err)
	}
	return nil
}
