package downloader

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
)

// Runner executes external commands and returns their combined output.
type Runner interface {
	Run(ctx context.Context, name string, args ...string) (string, error)
}

// outputStreamer streams command output line by line while still returning the
// full combined stdout/stderr, matching Run's capture behavior.
type outputStreamer interface {
	RunStreaming(ctx context.Context, onLine func(string), name string, args ...string) (string, error)
}

// ExecRunner executes commands using the local shell utilities (yt-dlp, ffmpeg).
type ExecRunner struct{}

func (ExecRunner) Run(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("%s failed: %w (output: %s)", name, err, output)
	}
	return string(output), nil
}

func (ExecRunner) RunStreaming(ctx context.Context, onLine func(string), name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return "", err
	}
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("%s failed: %w", name, err)
	}

	var (
		mu  sync.Mutex
		buf strings.Builder
		wg  sync.WaitGroup
	)
	consume := func(r io.Reader) {
		defer wg.Done()
		scanner := bufio.NewScanner(r)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			line := scanner.Text()
			mu.Lock()
			buf.WriteString(line)
			buf.WriteByte('\n')
			mu.Unlock()
			if onLine != nil {
				onLine(line)
			}
		}
	}
	wg.Add(2)
	go consume(stdout)
	go consume(stderr)
	wg.Wait()

	waitErr := cmd.Wait()
	output := buf.String()
	if waitErr != nil {
		return output, fmt.Errorf("%s failed: %w (output: %s)", name, waitErr, output)
	}
	return output, nil
}
