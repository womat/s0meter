package s0meters

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/womat/s0meter/pkg/pulsecounter"
	"gopkg.in/yaml.v3"
)

// RunPeriodicBackup saves the current counter values to filename every interval until ctx is
// cancelled. It blocks, so the caller runs it in a goroutine it can wait for. Errors are logged
// but do not stop the loop.
func (h *Handler) RunPeriodicBackup(ctx context.Context, interval time.Duration, filename string) {
	ticker := time.NewTicker(interval)
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
}

// ReadMeterData returns the saved pulse count per meter from the YAML file.
//
// Only the pulse counts are restored. The timestamps in the file describe pulses of a previous
// run, possibly taken against a wall clock that was later corrected; a gauge derived from them
// could be far off, so the gauge restarts with the first two pulses of this run instead.
//
// A missing file is not an error and yields an empty map: the counters start at zero and the
// file is written by the next save. Everything else that would start a meter at zero without
// saying so is an error, because the save right after the start would overwrite the real
// counter values: an empty or unparsable file, a document without meters (null, ~, only
// comments; a service without meters writes {}), a meter without pulses, and an unknown key
// such as a misspelled "pulse". A counter is set to zero on purpose with "pulses: 0".
func ReadMeterData(file string) (map[string]uint64, error) {
	data, err := os.ReadFile(file)
	if errors.Is(err, os.ErrNotExist) {
		slog.Info("Meter data file not found, counters start at zero", "file", file)
		return map[string]uint64{}, nil
	}
	if err != nil {
		return nil, err
	}

	if len(bytes.TrimSpace(data)) == 0 {
		return nil, fmt.Errorf("meter data file %s is empty; restore it, or delete it to start all counters at zero", file)
	}

	var saved map[string]*savedCounter
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err = dec.Decode(&saved); errors.Is(err, io.EOF) || (err == nil && saved == nil) {
		return nil, fmt.Errorf("meter data file %s holds no meters; restore it, or delete it to start all counters at zero", file)
	}
	if err != nil {
		return nil, fmt.Errorf("meter data file %s is damaged (%w); restore it, or delete it to start all counters at zero", file, err)
	}

	pulses := make(map[string]uint64, len(saved))
	for name, counter := range saved {
		if counter == nil || counter.Pulses == nil {
			return nil, fmt.Errorf(`meter data file %s is damaged: meter %q has no pulses; restore it, set "pulses: 0" to start it at zero, or remove the meter`, file, name)
		}
		pulses[name] = *counter.Pulses
	}
	return pulses, nil
}

// savedCounter is one entry of the data file as ReadMeterData reads it: the fields SaveMeterData
// writes for a pulsecounter.Counter, with Pulses a pointer so that a missing count is told apart
// from zero.
type savedCounter struct {
	Pulses        *uint64   `yaml:"pulses"`
	TimeStamp     time.Time `yaml:"timestamp"`
	LastTimeStamp time.Time `yaml:"lastTimestamp"`
}

// SaveMeterData writes the current counter values of all meters to the YAML file.
//
// The file is replaced atomically: the data goes to a temporary file in the same directory,
// is synced to disk and then renamed over the old file, and the directory is synced so the
// rename itself survives a power loss. A crash at any point leaves either the old or the new
// file, never an empty or half-written one.
func (h *Handler) SaveMeterData(file string) error {
	h.mux.RLock()
	data := make(map[string]pulsecounter.Counter, len(h.meters))
	for name, m := range h.meters {
		data[name] = m.Meter.GetCounter()
	}
	h.mux.RUnlock()

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

	if err = writeFileAtomic(file, yamlData, 0o600); err != nil {
		slog.Error("Failed to write meter data to file", "file", file, "error", err)
		return err
	}

	return nil
}

// writeFileAtomic replaces file with data via a synced temporary file and a rename.
func writeFileAtomic(file string, data []byte, perm os.FileMode) (err error) {
	dir := filepath.Dir(file)

	tmp, err := os.CreateTemp(dir, filepath.Base(file)+".tmp-*")
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = os.Remove(tmp.Name())
		}
	}()

	if _, err = tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err = tmp.Chmod(perm); err != nil {
		_ = tmp.Close()
		return err
	}
	if err = tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}

	if err = os.Rename(tmp.Name(), file); err != nil {
		return err
	}

	// Persist the rename. Failing here no longer risks the data, only the durability of the
	// rename, so it is reported but the new file stays in place.
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	if err = d.Sync(); err != nil {
		_ = d.Close()
		return err
	}
	return d.Close()
}
