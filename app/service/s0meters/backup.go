package s0meters

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"s0counter/pkg/pulsecounter"
	"time"

	"gopkg.in/yaml.v3"
)

// StartPeriodicBackup runs a periodic YAML backup in a separate goroutine.
//
// This function saves the current counter values to the configured DataFile
// every BackupInterval seconds. Errors are logged but do not stop execution.
func (h *Handler) StartPeriodicBackup(ctx context.Context) {
	interval := time.Duration(h.config.BackupInterval) * time.Second
	ticker := time.NewTicker(interval)

	go func() {
		defer ticker.Stop()
		slog.Info("Started periodic meter backup", "interval", interval)

		for {
			select {
			case <-ctx.Done():
				slog.Info("Stopping periodic meter backup")
				return
			case <-ticker.C:
				if err := h.saveMeterData(); err != nil {
					slog.Error("Failed to backup meter data", "error", err)
				} else {
					slog.Debug("Meter data backed up", "file", h.config.DataFile)
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
func (h *Handler) LoadMeterData() error {
	data, err := os.ReadFile(h.config.DataFile)
	if os.IsNotExist(err) {
		slog.Info("Meter data file not found, creating default", "file", h.config.DataFile)
		return h.saveMeterData()
	}
	if err != nil {
		slog.Error("Failed to read meter data file", "file", h.config.DataFile, "error", err)
		return err
	}

	var saved map[string]pulsecounter.Counter
	if err = yaml.Unmarshal(data, &saved); err != nil {
		slog.Error("Failed to unmarshal meter data YAML", "file", h.config.DataFile, "error", err)
		return err
	}

	h.Lock()
	defer h.Unlock()
	for name, counter := range saved {
		if m, ok := h.meters[name]; ok {
			m.Meter.SetCounter(counter)
		} else {
			slog.Debug("Saved meter not registered, skipping", "meter", name)
		}
	}

	slog.Debug("Meter data loaded", "file", h.config.DataFile)
	return nil
}

// saveMeterData writes the current counter values of all meters to the configured YAML file.
//
// The function is thread-safe and uses RLock for reading meter data.
// It marshals the meter counters into YAML and writes the file with 0600 permissions.
//
// Returns:
// - nil on success
// - an error if marshalling or file writing fails
func (h *Handler) saveMeterData() error {
	h.RLock()
	data := make(map[string]pulsecounter.Counter, len(h.meters))
	for name, m := range h.meters {
		data[name] = m.Meter.GetCounter()
	}
	h.RUnlock()

	// marshal to YAML
	yamlData, err := yaml.Marshal(data)
	if err != nil {
		slog.Error("Failed to marshal meter data to YAML", "error", err)
		return err
	}

	dir := filepath.Dir(h.config.DataFile)
	if _, err = os.Stat(dir); os.IsNotExist(err) {
		return fmt.Errorf("directory does not exist: %s", dir)
	}

	if err = os.WriteFile(h.config.DataFile, yamlData, 0o600); err != nil {
		slog.Error("Failed to write meter data to file", "file", h.config.DataFile, "error", err)
		return err
	}

	slog.Debug("Meter data saved", "file", h.config.DataFile)
	return nil
}
