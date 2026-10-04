package s0meters

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadMeterDataMissingFile(t *testing.T) {
	got, err := ReadMeterData(filepath.Join(t.TempDir(), "missing.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("got %v, want an empty map", got)
	}
}

func TestReadMeterDataRejectsEmptyOrDamagedFile(t *testing.T) {
	for name, content := range map[string]string{
		"empty":   "",
		"blank":   " \n\t\n",
		"damaged": "wallbox: [unclosed",
	} {
		t.Run(name, func(t *testing.T) {
			file := filepath.Join(t.TempDir(), "data.yaml")
			if err := os.WriteFile(file, []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := ReadMeterData(file); err == nil {
				t.Error("expected an error instead of starting at zero")
			}
		})
	}
}

func TestReadMeterDataRestoresPulsesOnly(t *testing.T) {
	file := filepath.Join(t.TempDir(), "data.yaml")
	content := "wallbox:\n" +
		"    pulses: 34341798\n" +
		"    timestamp: 2026-10-04T20:48:28+02:00\n" +
		"    lastTimestamp: 2026-10-04T20:30:32+02:00\n"
	if err := os.WriteFile(file, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := ReadMeterData(file)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got["wallbox"] != 34341798 {
		t.Errorf("got %v, want wallbox: 34341798", got)
	}
}

func TestSaveMeterDataRoundtripIsAtomic(t *testing.T) {
	file := filepath.Join(t.TempDir(), "sub", "data.yaml")
	h := handlerWith(map[string]*MeterInstance{
		"wallbox": newMeter(t, MeterConfig{Gpio: 17}, 42),
		"water":   newMeter(t, MeterConfig{Gpio: 5}, 7),
	})

	// Repeated saves replace the file instead of leaving temporary files behind.
	for range 3 {
		if err := h.SaveMeterData(file); err != nil {
			t.Fatal(err)
		}
	}

	got, err := ReadMeterData(file)
	if err != nil {
		t.Fatal(err)
	}
	if got["wallbox"] != 42 || got["water"] != 7 {
		t.Errorf("roundtrip = %v", got)
	}

	entries, err := os.ReadDir(filepath.Dir(file))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Errorf("directory holds %d entries, want only the data file", len(entries))
	}

	info, err := os.Stat(file)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("permissions = %v, want 0600", perm)
	}
}
