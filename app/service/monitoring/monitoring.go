package monitoring

import (
	"fmt"
	"runtime"
	"strings"
	"time"
)

// Model represents a monitoring metric for a service, including its state and associated data.
type Model struct {
	// Service is the name of the service being monitored.
	Service string `json:"Service"`

	// Host is the host on which the service is running.
	Host string `json:"Host"`

	// State represents the current state of the service (e.g., "OK", "Error").
	// If the state is empty, it will be determined based on WATCHIT thresholds.
	State string `json:"State"`

	// Value holds the actual metric data. The type is `any` to accommodate various types of values.
	// If empty, no statistic will be written in Odin.
	Value any `json:"Value,omitempty"`

	// Description provides a human-readable description of the service's status.
	Description string `json:"Description"`

	// Metric indicates the type of metric being reported (e.g., "gauge" or "counter").
	// If empty, no statistic will be written in Odin.
	Metric string `json:"Metric,omitempty"`
}

// Constants representing supported metric types.
const (
	MetricGauge   = "gauge"   // A metric that represents a value that can go up or down.
	MetricCounter = "counter" // A metric that only increases over time.
)

var startTime = time.Now() // Tracks the application's start time.

// Monitoring collects and returns monitoring data for various system metrics.
func Monitoring(host, version string) ([]Model, error) {

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	// Remove the port number from the host if present (e.g., "localhost:8080" -> "localhost").
	h := strings.Split(host, ":")
	host = h[0]

	// Create a slice of monitoring data for various system metrics.
	services := []Model{
		{
			Service:     "Uptime",
			Host:        host,
			State:       "OK",
			Value:       time.Since(startTime).Truncate(time.Hour).Hours(),
			Description: fmt.Sprintf("Uptime: %vh", time.Since(startTime).Truncate(time.Hour).Hours()),
			Metric:      MetricCounter},
		{
			Service:     "Version",
			Host:        host,
			State:       "OK",
			Description: version},
		{
			Service:     "Prog Lang",
			Host:        host,
			State:       "OK",
			Description: runtime.Version()},
		{
			Service:     "Operating System",
			Host:        host,
			State:       "OK",
			Description: runtime.GOOS},
		{
			Service:     "Number of Goroutines",
			Host:        host,
			State:       "OK",
			Value:       runtime.NumGoroutine(),
			Description: fmt.Sprintf("Number of Goroutines: %v", runtime.NumGoroutine()),
			Metric:      MetricCounter},
		{
			Service:     "Number of Cgo Calls",
			Host:        host,
			State:       "OK",
			Value:       runtime.NumCgoCall(),
			Description: fmt.Sprintf("Number of Cgo Calls: %v", runtime.NumCgoCall()),
			Metric:      MetricCounter},
		{
			Service:     "Sys Memory",
			Host:        host,
			State:       "OK",
			Value:       m.Sys,
			Description: fmt.Sprintf("Sys Memory: %vkB", m.Sys/1024),
			Metric:      MetricCounter},
		{
			Service:     "Total Memory Alloc",
			Host:        host,
			State:       "OK",
			Value:       m.TotalAlloc,
			Description: fmt.Sprintf("Total Memory Alloc: %vkB", m.TotalAlloc/1024),
			Metric:      MetricCounter},
		{
			Service:     "Count of Heap Objects allocated",
			Host:        host,
			State:       "OK",
			Value:       m.Mallocs,
			Description: fmt.Sprintf("Mallocs: %v", m.Mallocs),
			Metric:      MetricCounter},
		{
			Service:     "Free Count of Heap Objects",
			Host:        host,
			State:       "OK",
			Value:       m.Frees,
			Description: fmt.Sprintf("Frees: %v", m.Frees),
			Metric:      MetricCounter},
		{
			Service:     "Heap Alloc",
			Host:        host,
			State:       "OK",
			Value:       m.HeapAlloc,
			Description: fmt.Sprintf("Heap Alloc: %vkB", m.HeapAlloc/1024),
			Metric:      MetricCounter},
		{
			Service:     "Heap Sys",
			Host:        host,
			State:       "OK",
			Value:       m.HeapSys,
			Description: fmt.Sprintf("Heap Sys: %vkB", m.HeapSys/1024),
			Metric:      MetricCounter},
		{
			Service:     "Heap Idle",
			Host:        host,
			State:       "OK",
			Value:       m.HeapIdle,
			Description: fmt.Sprintf("Heap Idle: %vkB", m.HeapIdle/1024),
			Metric:      MetricCounter},
		{
			Service:     "Heap Inuse",
			Host:        host,
			State:       "OK",
			Value:       m.HeapInuse,
			Description: fmt.Sprintf("Heap Inuse: %vkB", m.HeapInuse/1024),
			Metric:      MetricCounter},
		{
			Service:     "Heap Released",
			Host:        host,
			State:       "OK",
			Value:       m.HeapReleased,
			Description: fmt.Sprintf("Heap Released: %vkB", m.HeapReleased/1024),
			Metric:      MetricCounter},
		{
			Service:     "Heap Objects",
			Host:        host,
			State:       "OK",
			Value:       m.HeapObjects,
			Description: fmt.Sprintf("Heap Objects: %v", m.HeapObjects),
			Metric:      MetricCounter},
	}

	return services, nil
}
