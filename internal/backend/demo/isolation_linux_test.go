//go:build linux

package demo

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDemoStartupMakesNoExternalAccess(t *testing.T) {
	strace, err := exec.LookPath("strace")
	if err != nil {
		t.Skip("strace not installed")
	}
	script, err := exec.LookPath("script")
	if err != nil {
		t.Skip("script not installed")
	}
	root := filepath.Join("..", "..", "..")
	tmp := t.TempDir()
	binary := filepath.Join(tmp, "quotary")
	build := exec.Command("go", "build", "-o", binary, "./cmd/quotary")
	build.Dir = root
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, output)
	}

	home := filepath.Join(tmp, "home")
	binDir := filepath.Join(tmp, "bin")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	secret := filepath.Join(home, "config.json")
	if err := os.WriteFile(secret, []byte("private"), 0o644); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(tmp, "executed")
	codexbar := filepath.Join(binDir, "codexbar")
	if err := os.WriteFile(codexbar, []byte("#!/bin/sh\necho executed >"+marker+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	traceFile := filepath.Join(tmp, "trace.log")
	traceCmd := fmt.Sprintf("%s -f -e trace=execve,openat,connect -o %s %s --demo", strace, traceFile, binary)
	cmd := exec.Command(script, "-qec", traceCmd, "/dev/null")
	cmd.Env = append(os.Environ(),
		"HOME="+home,
		"XDG_CONFIG_HOME="+filepath.Join(home, "xdg"),
		"PATH="+binDir+":"+os.Getenv("PATH"),
		"TERM=xterm-256color",
	)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	go func() {
		time.Sleep(500 * time.Millisecond)
		_, _ = io.WriteString(stdin, "qy")
		_ = stdin.Close()
	}()
	if err := cmd.Wait(); err != nil {
		if strings.Contains(output.String(), "ptrace") || strings.Contains(output.String(), "Operation not permitted") {
			t.Skip("sandbox denies ptrace; isolation evidence is recorded in docs/evidence")
		}
		t.Fatalf("demo run: %v\n%s", err, output.String())
	}
	trace, err := os.ReadFile(traceFile)
	if err != nil {
		t.Fatal(err)
	}
	if violations := traceViolations(string(trace), home, binary); len(violations) > 0 {
		t.Fatalf("demo startup accessed external state: %v", violations)
	}
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("demo executed codexbar")
	}

	negativeTrace := filepath.Join(tmp, "negative.log")
	negative := exec.Command(strace, "-f", "-e", "trace=execve,openat", "-o", negativeTrace, "cat", secret)
	if err := negative.Run(); err != nil {
		if strings.Contains(err.Error(), "ptrace") || strings.Contains(err.Error(), "Operation not permitted") {
			t.Skip("sandbox denies ptrace; negative control cannot run")
		}
		t.Fatal(err)
	}
	negativeData, err := os.ReadFile(negativeTrace)
	if err != nil {
		t.Fatal(err)
	}
	if violations := traceViolations(string(negativeData), home, binary); len(violations) == 0 {
		t.Fatal("negative control was not detected")
	}
}

func traceViolations(trace, home, binary string) []string {
	var violations []string
	for _, line := range strings.Split(trace, "\n") {
		switch {
		case strings.Contains(line, "connect("):
			violations = append(violations, "network: "+line)
		case strings.Contains(line, home):
			violations = append(violations, "home/config: "+line)
		case strings.Contains(line, "codexbar"):
			violations = append(violations, "backend executable: "+line)
		case strings.Contains(line, "execve(") && !strings.Contains(line, binary):
			violations = append(violations, "foreign process: "+line)
		}
	}
	return violations
}
