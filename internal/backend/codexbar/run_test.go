package codexbar

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func script(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "script.sh")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestExecRunnerTimeoutAndCancellation(t *testing.T) {
	path := script(t, "sleep 2\n")
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, _, err := (ExecRunner{}).Run(ctx, path, nil, 1024)
	if !errors.Is(err, ErrTimeout) {
		t.Fatalf("err=%v", err)
	}
}

func TestExecRunnerOutputCap(t *testing.T) {
	path := script(t, "printf '1234567890'\n")
	stdout, _, err := (ExecRunner{}).Run(context.Background(), path, nil, 4)
	if !errors.Is(err, ErrOutputCap) {
		t.Fatalf("err=%v stdout=%q", err, stdout)
	}
	if len(stdout) > 4 {
		t.Fatalf("stdout exceeded cap: %d", len(stdout))
	}
}

func TestExecRunnerProcessFailureRetainsDiagnostic(t *testing.T) {
	path := script(t, "echo 'sign in required' >&2\nexit 3\n")
	_, stderr, err := (ExecRunner{}).Run(context.Background(), path, nil, 1024)
	var process *ProcessError
	if !errors.As(err, &process) || process.Code != 3 {
		t.Fatalf("err=%v", err)
	}
	if !strings.Contains(process.Diagnostic(), "sign in required") || !strings.Contains(string(stderr), "sign in required") {
		t.Fatal("diagnostic missing")
	}
}

func TestExecRunnerKillsResistantGroupAfterGrace(t *testing.T) {
	path := script(t, "trap '' TERM\n(sleep 5) &\nwait\n")
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, _, err := (ExecRunner{}).Run(ctx, path, nil, 1024)
	elapsed := time.Since(start)
	if !errors.Is(err, ErrTimeout) {
		t.Fatalf("err=%v", err)
	}
	if elapsed > 3*time.Second {
		t.Fatalf("resistant child blocked cleanup for %v", elapsed)
	}
}

func TestExecRunnerStderrCapIsEnforced(t *testing.T) {
	path := script(t, "head -c 140000 /dev/zero | tr '\\0' 'x' >&2\n")
	_, _, err := (ExecRunner{}).Run(context.Background(), path, nil, 1024)
	if !errors.Is(err, ErrOutputCap) {
		t.Fatalf("err=%v", err)
	}
}

func TestExecRunnerRunEnvDeliversEnvironment(t *testing.T) {
	path := script(t, "printf '%s' \"$CODEXBAR_CONFIG\"\n")
	stdout, _, err := (ExecRunner{}).RunEnv(context.Background(), path, nil, 1024, []string{"CODEXBAR_CONFIG=/tmp/fixture.json"})
	if err != nil || string(stdout) != "/tmp/fixture.json" {
		t.Fatalf("stdout=%q err=%v", stdout, err)
	}
}
