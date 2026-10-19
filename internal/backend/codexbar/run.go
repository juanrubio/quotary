package codexbar

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"
)

// Runner executes the resolved CodexBar executable without a shell. Tests inject
// this seam; production uses ExecRunner.
type Runner interface {
	Run(ctx context.Context, path string, args []string, maxOutput int) (stdout, stderr []byte, err error)
}

// ExecRunner runs a direct child process, drains both pipes concurrently, and
// escalates termination after a bounded grace period.
type ExecRunner struct{}

const terminationGrace = 500 * time.Millisecond

func (ExecRunner) Run(ctx context.Context, path string, args []string, maxOutput int) ([]byte, []byte, error) {
	stdout := &limitedBuffer{max: maxOutput}
	stderr := &limitedBuffer{max: 128 << 10}
	outR, outW, err := os.Pipe()
	if err != nil {
		return nil, nil, fmt.Errorf("stdout pipe: %w", err)
	}
	errR, errW, err := os.Pipe()
	if err != nil {
		_ = outR.Close()
		_ = outW.Close()
		return nil, nil, fmt.Errorf("stderr pipe: %w", err)
	}
	cmd := exec.Command(path, args...)
	cmd.Stdout = outW
	cmd.Stderr = errW
	configureProcessGroup(cmd)
	if err := cmd.Start(); err != nil {
		_ = outR.Close()
		_ = outW.Close()
		_ = errR.Close()
		_ = errW.Close()
		return nil, nil, fmt.Errorf("execute: %w", err)
	}
	_ = outW.Close()
	_ = errW.Close()
	readers := make(chan struct{}, 2)
	go func() {
		_, _ = io.Copy(stdout, outR)
		_ = outR.Close()
		readers <- struct{}{}
	}()
	go func() {
		_, _ = io.Copy(stderr, errR)
		_ = errR.Close()
		readers <- struct{}{}
	}()
	wait := make(chan error, 1)
	go func() { wait <- cmd.Wait() }()
	var waitErr error
	cancelled := false
	select {
	case waitErr = <-wait:
	case <-ctx.Done():
		cancelled = true
		_ = terminateProcessGroup(cmd)
		select {
		case waitErr = <-wait:
		case <-time.After(terminationGrace):
			_ = killProcessGroup(cmd)
			select {
			case waitErr = <-wait:
			case <-time.After(terminationGrace):
			}
		}
	}
	// The leader may have exited while owned descendants remain alive.
	_ = killProcessGroup(cmd)
	drainReaders(outR, errR, readers)
	_ = waitErr
	if cancelled {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return stdout.Bytes(), stderr.Bytes(), ErrTimeout
		}
		return stdout.Bytes(), stderr.Bytes(), ctx.Err()
	}
	if stdout.overflow || stderr.overflow {
		return stdout.Bytes(), stderr.Bytes(), ErrOutputCap
	}
	if waitErr != nil {
		var exitErr *exec.ExitError
		if errors.As(waitErr, &exitErr) {
			return stdout.Bytes(), stderr.Bytes(), &ProcessError{Code: exitErr.ExitCode(), Stderr: stderr.Bytes()}
		}
		return stdout.Bytes(), stderr.Bytes(), fmt.Errorf("execute: %w", waitErr)
	}
	return stdout.Bytes(), stderr.Bytes(), nil
}

func drainReaders(outR, errR *os.File, done chan struct{}) {
	closed := false
	for i := 0; i < 2; i++ {
		select {
		case <-done:
		case <-time.After(terminationGrace):
			if !closed {
				_ = outR.Close()
				_ = errR.Close()
				closed = true
			}
			<-done
		}
	}
}

type limitedBuffer struct {
	mu       sync.Mutex
	buf      bytes.Buffer
	max      int
	overflow bool
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	remaining := b.max - b.buf.Len()
	if remaining <= 0 {
		b.overflow = true
		return len(p), nil
	}
	if len(p) > remaining {
		b.buf.Write(p[:remaining])
		b.overflow = true
		return len(p), nil
	}
	return b.buf.Write(p)
}

func (b *limitedBuffer) Bytes() []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]byte(nil), b.buf.Bytes()...)
}

// ProcessError retains a redacted diagnostic and exit code.
type ProcessError struct {
	Code   int
	Stderr []byte
}

func (e *ProcessError) Error() string { return fmt.Sprintf("codexbar exited with status %d", e.Code) }

// Diagnostic returns bounded, redacted stderr for diagnostics, never product copy.
func (e *ProcessError) Diagnostic() string {
	return redactDiagnostic(string(e.Stderr))
}
