package s0meters

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/synctest"
	"time"
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

// The loop saves every interval; a failed save is logged and the next one is tried again.
func TestPeriodicBackupSurvivesAFailedSave(t *testing.T) {
	h := handlerWith(map[string]*MeterInstance{
		"wallbox": newMeter(t, MeterConfig{Gpio: 17, CounterPulsesPerUnit: 1, GaugeScale: 1}, 4711),
	})
	// The data directory is a regular file at first, so the first save cannot create its
	// temporary file.
	dir := filepath.Join(t.TempDir(), "data")
	if err := os.WriteFile(dir, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "s0meter.yaml")

	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		done := make(chan struct{})
		go func() {
			h.RunPeriodicBackup(ctx, time.Minute, file)
			close(done)
		}()

		time.Sleep(time.Minute + time.Second)
		synctest.Wait()
		if _, err := os.Stat(file); err == nil {
			t.Fatal("the save into a regular file as directory succeeded")
		}

		if err := os.Remove(dir); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		time.Sleep(time.Minute)
		synctest.Wait()
		got, err := ReadMeterData(file)
		if err != nil || got["wallbox"] != 4711 {
			t.Errorf("after the next interval: %v, %v; want wallbox at 4711", got, err)
		}

		cancel()
		<-done
	})
}

// A rename that fails leaves the old file in place and no temporary file behind.
func TestWriteFileAtomicCleansUpAfterAFailedRename(t *testing.T) {
	dir := t.TempDir()
	// A non-empty directory cannot be replaced by a file, not even by root.
	target := filepath.Join(dir, "s0meter.yaml")
	if err := os.MkdirAll(filepath.Join(target, "keep"), 0o700); err != nil {
		t.Fatal(err)
	}

	if err := writeFileAtomic(target, []byte("wallbox: {}\n"), 0o600); err == nil {
		t.Fatal("writeFileAtomic over a non-empty directory succeeded")
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.Contains(e.Name(), ".tmp-") {
			t.Errorf("temporary file %s left behind", e.Name())
		}
	}
	if _, err := os.Stat(filepath.Join(target, "keep")); err != nil {
		t.Errorf("the old target was touched: %v", err)
	}
}
