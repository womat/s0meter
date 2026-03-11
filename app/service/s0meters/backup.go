package s0meters

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/womat/s0meter/pkg/pulsecounter"
	"gopkg.in/yaml.v3"
)

// StartPeriodicBackup runs a periodic YAML backup in a separate goroutine.
//
// This function saves the current counter values to the configured DataFile
// every BackupInterval seconds. Errors are logged but do not stop execution.
func (h *Handler) StartPeriodicBackup(ctx context.Context, interval time.Duration, filename string) {
	ticker := time.NewTicker(interval)

	go func() {
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				slog.Info("Stopping periodic meter backup")
				return
			case <-ticker.C:
				slog.Debug("Starting meter data backup", "file", filename)
				if err := h.SaveMeterData(filename); err != nil {
					slog.Error("Failed to backup meter data", "error", err)
				}
			}
		}
	}()
}

// LoadMeterData restores the last saved counter values for all meters from a YAML file.
//
// The function reads the meter counters from the configured YAML file and updates
// the registered meters. If the file does not exist, it will be created with default
// counter values by calling saveMeterData().
//
// Thread-safe: uses Lock to update meter counters.
//
// Returns:
// - nil on success
// - an error if reading or unmarshalling the YAML fails
func (h *Handler) LoadMeterData(file string) error {
	data, err := os.ReadFile(file)
	if errors.Is(err, os.ErrNotExist) {
		slog.Info("Meter data file not found, creating default", "file", file)
		return h.SaveMeterData(file)
	}
	if err != nil {
		return err
	}

	var saved map[string]pulsecounter.Counter
	if err = yaml.Unmarshal(data, &saved); err != nil {
		return err
	}

	h.mux.Lock()
	defer h.mux.Unlock()
	for name, counter := range saved {
		if m, ok := h.meters[name]; ok {
			m.Meter.SetCounter(counter)
		} else {
			slog.Debug("Saved meter not registered, skipping", "meter", name)
		}
	}

	return nil
}

// SaveMeterData writes the current counter values of all meters to the configured YAML file.
//
// The function is thread-safe and uses RLock for reading meter data.
// It marshals the meter counters into YAML and writes the file with 0600 permissions.
//
// Returns:
// - nil on success
// - an error if marshalling or file writing fails
func (h *Handler) SaveMeterData(file string) error {
	h.mux.RLock()
	data := make(map[string]pulsecounter.Counter, len(h.meters))
	for name, m := range h.meters {
		data[name] = m.Meter.GetCounter()
	}
	h.mux.RUnlock()

	// marshal to YAML
	yamlData, err := yaml.Marshal(data)
	if err != nil {
		slog.Error("Failed to marshal meter data to YAML", "error", err)
		return err
	}

	dir := filepath.Dir(file)
	if _, err = os.Stat(dir); os.IsNotExist(err) {
		slog.Info("Creating meter data dir", "dir", dir)
		if err = os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}
	}

	if err = os.WriteFile(file, yamlData, 0o600); err != nil {
		slog.Error("Failed to write meter data to file", "file", file, "error", err)
		return err
	}

	return nil
}
