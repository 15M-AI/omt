package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type apiServer struct {
	cmd  *exec.Cmd
	done chan struct{}
}

type Config struct {
	Runtime string
	Source  string
	Host    string
	Port    int
}

type modelInfo struct {
	Name string
	Size string
}

func modelDirectory() (string, error) {
	root := os.Getenv("OPEN_MIND_HOME")
	if root == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		root = filepath.Join(home, ".open-mind")
	}
	return filepath.Join(root, "models"), nil
}

func installedModels() ([]modelInfo, error) {
	dir, err := modelDirectory()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	models := make([]modelInfo, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".omf" {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return nil, err
		}
		models = append(models, modelInfo{
			Name: entry.Name(),
			Size: fmt.Sprintf("%.1f MiB", float64(info.Size())/(1024*1024)),
		})
	}
	return models, nil
}

func runRuntime(runtime string, args ...string) (string, error) {
	if runtime == "" {
		return "", errors.New("Open Mind runtime not found; build it with the Runtime option or set OPEN_MIND_RUNTIME")
	}
	cmd := exec.Command(runtime, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = strings.TrimSpace(stdout.String())
		}
		if detail == "" {
			detail = err.Error()
		}
		return "", errors.New(detail)
	}
	output := strings.TrimSpace(stdout.String())
	if output == "" {
		output = strings.TrimSpace(stderr.String())
	}
	return output, nil
}

type completionResponse struct {
	Choices []struct {
		Text string `json:"text"`
	} `json:"choices"`
}

func generate(ctx context.Context, host string, port int, model, prompt string, maxTokens int) (string, error) {
	body, err := json.Marshal(map[string]any{
		"model":       model,
		"prompt":      prompt,
		"max_tokens":  maxTokens,
		"temperature": 0.8,
		"top_k":       40,
	})
	if err != nil {
		return "", err
	}
	url := fmt.Sprintf("http://%s:%d/v1/generate", host, port)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 5 * time.Minute}
	response, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("could not reach OMA at %s: %w", url, err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		message, _ := io.ReadAll(io.LimitReader(response.Body, 8*1024))
		return "", fmt.Errorf("runtime returned %s: %s", response.Status, strings.TrimSpace(string(message)))
	}
	var result completionResponse
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("invalid OMA response: %w", err)
	}
	if len(result.Choices) == 0 {
		return "", errors.New("runtime returned no completion choices")
	}
	return result.Choices[0].Text, nil
}

func startServer(runtime, model, host string, port int) (*apiServer, error) {
	if runtime == "" {
		return nil, errors.New("Open Mind runtime not found; build it with the Runtime option or set OPEN_MIND_RUNTIME")
	}
	dir, err := modelDirectory()
	if err != nil {
		return nil, err
	}
	logDir := filepath.Join(filepath.Dir(dir), "logs")
	if err := os.MkdirAll(logDir, 0o700); err != nil {
		return nil, err
	}
	logFile, err := os.OpenFile(filepath.Join(logDir, "tui-server.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(runtime, "serve", "--model", model, "--host", host, "--port", fmt.Sprint(port))
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Env = os.Environ()
	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		return nil, err
	}
	server := &apiServer{cmd: cmd, done: make(chan struct{})}
	go func() {
		_ = cmd.Wait()
		_ = logFile.Close()
		close(server.done)
	}()
	address := net.JoinHostPort(host, fmt.Sprint(port))
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case <-server.done:
			return nil, fmt.Errorf("runtime server exited during startup; see %s", filepath.Join(logDir, "tui-server.log"))
		default:
		}
		conn, err := net.DialTimeout("tcp", address, 100*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			time.Sleep(100 * time.Millisecond)
			select {
			case <-server.done:
				return nil, fmt.Errorf("runtime server could not claim %s; see %s", address, filepath.Join(logDir, "tui-server.log"))
			default:
				return server, nil
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	_ = cmd.Process.Kill()
	<-server.done
	return nil, fmt.Errorf("server did not listen on %s; see %s", address, filepath.Join(logDir, "tui-server.log"))
}

func (server *apiServer) running() bool {
	if server == nil {
		return false
	}
	select {
	case <-server.done:
		return false
	default:
		return true
	}
}

func stopServer(server *apiServer) error {
	if server == nil || server.cmd == nil || server.cmd.Process == nil || !server.running() {
		return nil
	}
	if err := server.cmd.Process.Signal(os.Interrupt); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return err
	}
	select {
	case <-server.done:
		return nil
	case <-time.After(3 * time.Second):
		if err := server.cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
			return err
		}
		select {
		case <-server.done:
			return nil
		case <-time.After(2 * time.Second):
			return errors.New("timed out waiting for the OMA server to stop")
		}
	}
}

func buildRuntime(source string) (string, error) {
	if source == "" {
		return "", errors.New("Open Mind source not found; set OPEN_MIND_SOURCE")
	}
	cmd := exec.Command("cargo", "build", "--release")
	cmd.Dir = source
	output, err := cmd.CombinedOutput()
	if err != nil {
		lines := strings.Split(strings.TrimSpace(string(output)), "\n")
		if len(lines) > 12 {
			lines = lines[len(lines)-12:]
		}
		return "", fmt.Errorf("cargo build failed: %s", strings.Join(lines, "\n"))
	}
	return filepath.Join(source, "target", "release", "om"), nil
}
