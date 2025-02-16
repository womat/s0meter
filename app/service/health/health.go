package health

import (
	"os"
	"runtime"
	"time"
)

// Model holds the system and application health data.
type Model struct {
	// NumGoroutines is the number of currently running goroutines in the application.
	NumGoroutines int `json:"NumGoroutines"`

	// HeapAllocatedBytes represents the amount of memory allocated by the heap in bytes.
	HeapAllocatedBytes uint64 `json:"HeapAllocatedBytes"`

	// HeapAllocatedMB represents the amount of memory allocated by the heap in megabytes.
	HeapAllocatedMB float64 `json:"HeapAllocatedMB"`

	// SysMemoryBytes represents the total system memory in bytes.
	SysMemoryBytes uint64 `json:"SysMemoryBytes"`

	// SysMemoryMB represents the total system memory in megabytes.
	SysMemoryMB float64 `json:"SysMemoryMB"`

	// Version indicates the application version.
	Version string `json:"Version"`

	// ProgLang represents the version of the Go programming language used.
	ProgLang string `json:"ProgLang"`

	// HostName is the name of the host machine running the application.
	HostName string `json:"HostName"`

	// Time is the timestamp when the health data was collected, in RFC3339 format.
	Time string `json:"Time"`

	// OperatingSystem is the name of the operating system on which the application is running.
	OperatingSystem string `json:"OperatingSystem"`
}

// Health returns the current health data of the application and system.
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
