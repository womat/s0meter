package main

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// freePort returns a TCP port on 127.0.0.1 that was free a moment ago.
func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = l.Close() }()
	return l.Addr().(*net.TCPAddr).Port
}

// writeLoopConfig writes config.yaml into dir for a service on 127.0.0.1:port with the embedded
// development certificate, its data file and the token file "token-hashes" next to it.
func writeLoopConfig(t *testing.T, dir string, port int) string {
	t.Helper()
	missing := filepath.Join(dir, "missing.pem")
	return writeFile(t, dir, "config.yaml", fmt.Sprintf(`env: dev
logDestination: "null"
dataFile: %s
webserver:
  listenHost: 127.0.0.1
  listenPort: %d
  certFile: %s
  keyFile: %s
  tokenFile: token-hashes
`, filepath.Join(dir, "s0meter.yaml"), port, missing, missing))
}

// token runs "s0meter token args..." the way main dispatches it, with config as the default
// config file, and returns what it printed on stdout.
func token(t *testing.T, config string, args ...string) string {
	t.Helper()
	var stdout bytes.Buffer
	cmd := tokenCommand(config)
	cmd.Stdout, cmd.Stderr = &stdout, io.Discard
	if code := cmd.Run(args); code != 0 {
		t.Fatalf("s0meter token %v = %d", args, code)
	}
	return strings.TrimSpace(stdout.String())
}

// loopRun is a loop started in the background.
type loopRun struct {
	signals chan os.Signal
	done    chan int
}

// startLoop runs loop with config and waits until its server on port answers.
func startLoop(t *testing.T, config string, port int) *loopRun {
	t.Helper()
	l := &loopRun{signals: make(chan os.Signal, 1), done: make(chan int, 1)}
	go func() { l.done <- loop(config, false, l.signals) }()
	l.waitFor(t, "server up", func() bool { return status(port, "/version", "") == http.StatusOK })
	return l
}

// waitFor polls cond until it holds; it fails when the loop ends or after ten seconds.
func (l *loopRun) waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.After(10 * time.Second)
	for !cond() {
		select {
		case code := <-l.done:
			t.Fatalf("loop ended with %d while waiting for %s", code, what)
		case <-deadline:
			t.Fatalf("timed out waiting for %s", what)
		case <-time.After(20 * time.Millisecond):
		}
	}
}

// exitCode waits for the loop to end and returns its exit code.
func (l *loopRun) exitCode(t *testing.T) int {
	t.Helper()
	select {
	case code := <-l.done:
		return code
	case <-time.After(10 * time.Second):
		t.Fatal("loop did not end")
		return -1
	}
}

// status sends a GET for path to the server on port, with token as bearer token unless it is
// empty, and returns the status code, or 0 when the server cannot be reached.
func status(port int, path, token string) int {
	client := &http.Client{Timeout: 2 * time.Second, Transport: &http.Transport{
		DisableKeepAlives: true,                                  // every request sees whether the server is up right now
		TLSClientConfig:   &tls.Config{InsecureSkipVerify: true}, // embedded development certificate
	}}
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("https://127.0.0.1:%d%s", port, path), nil)
	if err != nil {
		return 0
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0
	}
	_ = resp.Body.Close()
	return resp.StatusCode
}

// SIGTERM stops the loop with 0, after the counters have been saved.
func TestLoopStopsOnSIGTERM(t *testing.T) {
	dir, port := t.TempDir(), freePort(t)
	config := writeLoopConfig(t, dir, port)
	token(t, config, "add", "nodered")

	l := startLoop(t, config, port)
	l.signals <- syscall.SIGTERM
	if code := l.exitCode(t); code != 0 {
		t.Errorf("loop = %d after SIGTERM, want 0", code)
	}
	if _, err := os.Stat(filepath.Join(dir, "s0meter.yaml")); err != nil {
		t.Errorf("data file not saved: %v", err)
	}
}

// A first start that fails has no previous configuration to fall back to and ends with 1.
func TestLoopEndsWhenFirstStartFails(t *testing.T) {
	busy, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = busy.Close() }()

	dir := t.TempDir()
	config := writeLoopConfig(t, dir, busy.Addr().(*net.TCPAddr).Port)
	token(t, config, "add", "nodered")

	if code := loop(config, false, make(chan os.Signal, 1)); code != 1 {
		t.Errorf("loop = %d with the port taken, want 1", code)
	}
}

// "s0meter token add" and "remove" write the file the service reads, and SIGHUP applies the
// change without a restart of the process.
func TestLoopReloadAppliesTokenCommand(t *testing.T) {
	dir, port := t.TempDir(), freePort(t)
	config := writeLoopConfig(t, dir, port)
	oldToken := token(t, config, "add", "old")

	l := startLoop(t, config, port)
	if got := status(port, "/health", oldToken); got != http.StatusOK {
		t.Fatalf("GET /health with the token from \"s0meter token add\" = %d, want 200", got)
	}

	newToken := token(t, config, "add", "new")
	if got := status(port, "/health", newToken); got != http.StatusUnauthorized {
		t.Errorf("new token before the reload = %d, want 401", got)
	}
	l.signals <- syscall.SIGHUP
	l.waitFor(t, "new token accepted", func() bool { return status(port, "/health", newToken) == http.StatusOK })

	token(t, config, "remove", "old")
	l.signals <- syscall.SIGHUP
	l.waitFor(t, "old token rejected", func() bool { return status(port, "/health", oldToken) == http.StatusUnauthorized })

	l.signals <- syscall.SIGTERM
	if code := l.exitCode(t); code != 0 {
		t.Errorf("loop = %d after SIGTERM, want 0", code)
	}
}

// A SIGHUP whose config file or token file does not load is refused before anything is torn
// down: the server stays up the whole time.
func TestLoopRefusesBrokenReloadWithoutRestart(t *testing.T) {
	for name, breakIt := range map[string]func(t *testing.T, dir string){
		"config file": func(t *testing.T, dir string) {
			writeFile(t, dir, "config.yaml", "unknown: key\n")
		},
		"token file": func(t *testing.T, dir string) {
			writeFile(t, dir, "token-hashes", "nodered not-a-hash\n")
		},
	} {
		t.Run(name, func(t *testing.T) {
			dir, port := t.TempDir(), freePort(t)
			config := writeLoopConfig(t, dir, port)
			nodered := token(t, config, "add", "nodered")
			l := startLoop(t, config, port)

			breakIt(t, dir)
			l.signals <- syscall.SIGHUP
			// The second send blocks until the first SIGHUP has been taken off the channel.
			l.signals <- syscall.SIGHUP

			// A restart would close the port for at least the second the loop waits.
			for end := time.Now().Add(1500 * time.Millisecond); time.Now().Before(end); {
				if got := status(port, "/health", nodered); got != http.StatusOK {
					t.Fatalf("GET /health = %d after a refused reload, want 200 without a restart", got)
				}
				time.Sleep(20 * time.Millisecond)
			}

			l.signals <- syscall.SIGTERM
			if code := l.exitCode(t); code != 0 {
				t.Errorf("loop = %d after SIGTERM, want 0", code)
			}
		})
	}
}

// A reload whose config passes the check but fails to start falls back to the previous
// configuration and keeps counting.
func TestLoopFallsBackToLastGood(t *testing.T) {
	busy, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = busy.Close() }()

	dir, port := t.TempDir(), freePort(t)
	config := writeLoopConfig(t, dir, port)
	token(t, config, "add", "nodered")
	l := startLoop(t, config, port)

	// Valid, but the port is taken, so the new App fails in StartWebServer.
	writeLoopConfig(t, dir, busy.Addr().(*net.TCPAddr).Port)
	l.signals <- syscall.SIGHUP
	l.waitFor(t, "old App stopped", func() bool { return status(port, "/version", "") == 0 })
	l.waitFor(t, "previous configuration running again", func() bool { return status(port, "/version", "") == http.StatusOK })

	l.signals <- syscall.SIGTERM
	if code := l.exitCode(t); code != 0 {
		t.Errorf("loop = %d after SIGTERM, want 0", code)
	}
}

// When the previous configuration fails to start as well, nothing is left to count with and
// the loop ends with 1.
func TestLoopEndsWhenLastGoodFailsToo(t *testing.T) {
	dir, port := t.TempDir(), freePort(t)
	config := writeLoopConfig(t, dir, port)
	token(t, config, "add", "nodered")
	l := startLoop(t, config, port)

	// Replace the data file by a non-empty directory: no App can read it, not even as root.
	dataFile := filepath.Join(dir, "s0meter.yaml")
	if err := os.Remove(dataFile); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dataFile, "keep"), 0o700); err != nil {
		t.Fatal(err)
	}

	// A data file no App can read does not fix itself, so systemd must not restart s0meter.
	l.signals <- syscall.SIGHUP
	if code := l.exitCode(t); code != exitConfig {
		t.Errorf("loop = %d, want %d when the previous configuration fails on the data file too", code, exitConfig)
	}
}

// A damaged data file at the first start ends with exitConfig as well, not with 1, after which
// systemd would restart s0meter every few seconds into the same error.
func TestLoopEndsWithExitConfigOnDamagedDataFile(t *testing.T) {
	dir, port := t.TempDir(), freePort(t)
	config := writeLoopConfig(t, dir, port)
	token(t, config, "add", "nodered")
	writeFile(t, dir, "s0meter.yaml", "wallbox:\n    pulse: 34341798\n")

	if code := loop(config, false, make(chan os.Signal, 1)); code != exitConfig {
		t.Errorf("loop = %d with a damaged data file, want %d", code, exitConfig)
	}
}

// A log file that cannot be opened at the first start does not fix itself either: exitConfig.
// Validate already refuses a log directory that is not one, so the config check ends the start
// here, before xlog.Init is reached.
func TestLoopEndsWithExitConfigWhenLoggerFails(t *testing.T) {
	dir, port := t.TempDir(), freePort(t)
	config := writeLoopConfig(t, dir, port)
	token(t, config, "add", "nodered")
	// The parent of the log file is a regular file, so the log file cannot be created.
	writeFile(t, dir, "log", "")
	content, err := os.ReadFile(config)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, "config.yaml", strings.Replace(string(content),
		`logDestination: "null"`, "logDestination: "+filepath.Join(dir, "log", "s0meter.log"), 1))

	if code := loop(config, false, make(chan os.Signal, 1)); code != exitConfig {
		t.Errorf("loop = %d with a log file that cannot be created, want %d", code, exitConfig)
	}
}
