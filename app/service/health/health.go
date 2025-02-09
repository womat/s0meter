package health

import (
	"os"
	"runtime"
	"time"
)

type Model = struct {
	NumGoroutines      int
	HeapAllocatedBytes uint64
	HeapAllocatedMB    float64
	SysMemoryBytes     uint64
	SysMemoryMB        float64
	Version            string
	ProgLang           string
	HostName           string
	Time               string
	OperatingSystem    string
}

func Health(version string) Model {
	bToMb := func(b uint64) float64 {
		return float64(b) / (1024 * 1024)
	}
	host, err := os.Hostname()
	if err != nil {
		host = "unknown"
	}

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	model := Model{
		NumGoroutines:      runtime.NumGoroutine(),
		HeapAllocatedBytes: m.Alloc,
		HeapAllocatedMB:    bToMb(m.Alloc),
		SysMemoryBytes:     m.Sys,
		SysMemoryMB:        bToMb(m.Sys),
		ProgLang:           runtime.Version(),
		Version:            version,
		HostName:           host,
		Time:               time.Now().Format(time.RFC3339),
		OperatingSystem:    runtime.GOOS,
	}

	return model
}
