package main

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// writeFile writes content to name in dir and returns the path.
func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// A config that does not load or validate ends the start with exitConfig, so systemd leaves
// the service stopped instead of restarting it every few seconds.
func TestRunRefusesBrokenConfigWithExitConfig(t *testing.T) {
	dir := t.TempDir()
	for name, config := range map[string]string{
		"missing file":       filepath.Join(dir, "missing.yaml"),
		"unknown key":        writeFile(t, dir, "unknown.yaml", "webserver:\n  apikey: x\n"),
		"no tokenFile":       writeFile(t, dir, "no-tokens.yaml", "env: dev\n"),
		"missing token file": writeFile(t, dir, "missing-tokens.yaml", "webserver:\n  tokenFile: nope\n"),
	} {
		t.Run(name, func(t *testing.T) {
			if got := run(config, false); got != exitConfig {
				t.Errorf("run = %d, want %d", got, exitConfig)
			}
		})
	}
}

// --debug overrides the log settings of the config file.
func TestLoadConfigDebugOverridesLogging(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "token-hashes", "")
	config := writeFile(t, dir, "config.yaml",
		"env: dev\nlogLevel: error\nlogDestination: /var/log/s0meter.log\nwebserver:\n  tokenFile: token-hashes\n")

	c, err := loadConfig(config, true)
	if err != nil {
		t.Fatal(err)
	}
	if c.LogLevel != "debug" || c.LogDestination != "stdout" {
		t.Errorf("logLevel %q, logDestination %q; want debug to stdout", c.LogLevel, c.LogDestination)
	}

	if c, err = loadConfig(config, false); err != nil {
		t.Fatal(err)
	}
	if c.LogLevel != "error" {
		t.Errorf("logLevel = %q without --debug, want the configured error", c.LogLevel)
	}
}

// The unit keeps systemd from restarting s0meter after exitConfig; both must name the same code.
func TestUnitPreventsRestartOnExitConfig(t *testing.T) {
	unit, err := os.ReadFile("../contrib/s0meter.service")
	if err != nil {
		t.Fatal(err)
	}
	if want := "\nRestartPreventExitStatus=" + strconv.Itoa(exitConfig) + "\n"; !strings.Contains(string(unit), want) {
		t.Errorf("contrib/s0meter.service lacks %q", strings.TrimSpace(want))
	}
}
