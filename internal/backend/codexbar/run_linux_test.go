//go:build linux

package codexbar

import (
	"context"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestExecRunnerCleansOwnedDescendantAfterParentExit(t *testing.T) {
	dir := t.TempDir()
	pidFile := dir + "/pid"
	path := dir + "/script.sh"
	body := "#!/bin/sh\ntrap '' TERM\n(sleep 5) >/dev/null 2>&1 &\necho $! > " + pidFile + "\nexit 0\n"
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	if _, _, err := (ExecRunner{}).Run(context.Background(), path, nil, 1024); err != nil {
		t.Fatal(err)
	}
	if time.Since(start) > 3*time.Second {
		t.Fatal("runner waited for descendant")
	}
	data, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	if err := syscall.Kill(pid, 0); err == nil {
		_ = syscall.Kill(pid, syscall.SIGKILL)
		t.Fatalf("owned descendant %d survived runner completion", pid)
	}
}

func TestExecRunnerCancellationCleansDescendantAfterLeaderExits(t *testing.T) {
	dir := t.TempDir()
	pidFile := dir + "/pid"
	path := dir + "/script.sh"
	body := "#!/bin/sh\ntrap 'exit 0' TERM\n(trap '' TERM; sleep 5) >/dev/null 2>&1 &\necho $! > " + pidFile + "\nwait\n"
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := make(chan error, 1)
	go func() {
		_, _, err := (ExecRunner{}).Run(ctx, path, nil, 1024)
		result <- err
	}()
	deadline := time.Now().Add(2 * time.Second)
	for {
		if _, statErr := os.Stat(pidFile); statErr == nil {
			break
		}
		if time.Now().After(deadline) {
			cancel()
			runErr := <-result
			_ = runErr
			t.Fatal("child did not become ready")
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	if err := <-result; err == nil {
		t.Fatal("cancelled run returned success")
	}
	data, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	if err := syscall.Kill(pid, 0); err == nil {
		_ = syscall.Kill(pid, syscall.SIGKILL)
		t.Fatalf("descendant %d survived cancellation cleanup", pid)
	}
}

func TestExecRunnerBoundsWaitForEscapedInheritedPipe(t *testing.T) {
	if _, err := exec.LookPath("setsid"); err != nil {
		t.Skip("setsid unavailable")
	}
	dir := t.TempDir()
	pidFile := dir + "/pid"
	path := dir + "/script.sh"
	body := "#!/bin/sh\ntrap 'exit 0' TERM\nsetsid sh -c 'sleep 5' &\necho $! > " + pidFile + "\nwait\n"
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, _, err := (ExecRunner{}).Run(ctx, path, nil, 1024)
	if err != ErrTimeout {
		t.Fatalf("err=%v", err)
	}
	if time.Since(start) > 3*time.Second {
		t.Fatalf("escaped inherited pipe blocked shutdown for %v", time.Since(start))
	}
	if data, readErr := os.ReadFile(pidFile); readErr == nil {
		if pid, parseErr := strconv.Atoi(strings.TrimSpace(string(data))); parseErr == nil {
			_ = syscall.Kill(pid, syscall.SIGKILL)
		}
	}
}
