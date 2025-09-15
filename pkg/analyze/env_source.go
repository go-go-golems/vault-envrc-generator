package analyze

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/subosito/gotenv"
)

// captureEnvironment resolves environment variables from the configured source.
func captureEnvironment(ctx context.Context, opts Options) (map[string]string, error) {
	switch opts.EnvSource {
	case "", EnvSourceCurrent:
		return captureCurrentEnvironment(), nil
	case EnvSourceEnvrc:
		return captureFromEnvrc(ctx, opts)
	case EnvSourceDirenv:
		return captureFromDirenv(ctx, opts)
	case EnvSourceFile:
		return captureFromDotenv(opts)
	default:
		return nil, fmt.Errorf("unsupported env source %q", opts.EnvSource)
	}
}

func captureCurrentEnvironment() map[string]string {
	result := make(map[string]string)
	for _, kv := range os.Environ() {
		parts := strings.SplitN(kv, "=", 2)
		if len(parts) == 2 {
			result[parts[0]] = parts[1]
		}
	}
	return result
}

func captureFromDotenv(opts Options) (map[string]string, error) {
	path := opts.DotenvPath
	if path == "" {
		path = ".env"
	}
	data, err := gotenv.Read(path)
	if err != nil {
		return nil, fmt.Errorf("failed to parse dotenv file %s: %w", path, err)
	}
	return data, nil
}

func captureFromEnvrc(ctx context.Context, opts Options) (map[string]string, error) {
	if !opts.ConfirmExec {
		return nil, errors.New("--confirm-exec is required when using env-source=envrc")
	}
	path := opts.EnvrcPath
	if path == "" {
		path = ".envrc"
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve envrc path %s: %w", path, err)
	}
	script := fmt.Sprintf("set -a; source %s >/dev/null 2>&1 || true; env -0", shellQuote(absPath))
	cmd := exec.CommandContext(ctx, "bash", "-lc", script)
	cmd.Dir = opts.WorkingDir
	if cmd.Dir == "" {
		cmd.Dir = filepath.Dir(absPath)
	}

	if opts.EmptyEnv {
		home := os.Getenv("HOME")
		pathEnv := os.Getenv("PATH")
		cmd.Env = []string{fmt.Sprintf("HOME=%s", home), fmt.Sprintf("PATH=%s", pathEnv)}
	} else {
		cmd.Env = os.Environ()
	}

	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &bytes.Buffer{}

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("failed to source %s: %w", absPath, err)
	}

	scanner := bufio.NewScanner(&stdout)
	scanner.Split(splitNull)

	result := make(map[string]string)
	for scanner.Scan() {
		entry := scanner.Text()
		if entry == "" {
			continue
		}
		parts := strings.SplitN(entry, "=", 2)
		if len(parts) != 2 {
			continue
		}
		result[parts[0]] = parts[1]
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to parse envrc output: %w", err)
	}
	return result, nil
}

func captureFromDirenv(ctx context.Context, opts Options) (map[string]string, error) {
	path := opts.WorkingDir
	if path == "" {
		path = "."
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "direnv", "export", "json")
	cmd.Dir = path
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("direnv export json failed: %w", err)
	}
	var data map[string]string
	if err := json.Unmarshal(output, &data); err != nil {
		// Some versions wrap the payload under "data"; attempt to decode accordingly.
		var wrapper struct {
			Data map[string]string `json:"data"`
		}
		if err2 := json.Unmarshal(output, &wrapper); err2 != nil || len(wrapper.Data) == 0 {
			return nil, fmt.Errorf("failed to parse direnv json: %w", err)
		}
		data = wrapper.Data
	}
	return data, nil
}

func splitNull(data []byte, atEOF bool) (int, []byte, error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	if i := bytes.IndexByte(data, 0); i >= 0 {
		return i + 1, data[0:i], nil
	}
	if atEOF {
		return len(data), data, nil
	}
	return 0, nil, nil
}

func shellQuote(path string) string {
	return "'" + strings.ReplaceAll(path, "'", "'\\''") + "'"
}
