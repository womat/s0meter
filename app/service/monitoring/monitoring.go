package monitoring

import (
	"fmt"
	"runtime"
	"strings"
	"time"
)

type Model struct {
	Service string `json:"Service"`
	Host    string `json:"Host"`
	// bei einem leeren State wird der State von WATCHIT aufgrund von Thresholds ermittelt
	State       string `json:"State"`
	Value       any    `json:"Value,omitempty"`
	Description string `json:"Description"`
	// supported metric types: gauge and counter
	// bei einer leeren Metric wird keine im Odin keine Statistic geschrieben
	Metric string `json:"Metric,omitempty"`
}

// supported metric types
const (
	MetricGauge   = "gauge"
	MetricCounter = "counter"
)

var startTime = time.Now()

// Monitoring returns monitoring data.
func Monitoring(host, version string) ([]Model, error) {

	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	h := strings.Split(host, ":")
	host = h[0]

	services := []Model{
		{
			Service:     "Uptime",
			Host:        host,
			State:       "OK",
			Value:       time.Since(startTime).Truncate(time.Hour).Hours(),
			Description: fmt.Sprintf("Uptime (h): %v", time.Since(startTime).Truncate(time.Hour).Hours()),
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
			Description: fmt.Sprintf("Sys Memory: %v kB", m.Sys/1024),
			Metric:      MetricCounter},
		{
			Service:     "Total Memory Alloc",
			Host:        host,
			State:       "OK",
			Value:       m.TotalAlloc,
			Description: fmt.Sprintf("Total Memory Alloc: %v kB", m.TotalAlloc/1024),
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
			Description: fmt.Sprintf("Heap Alloc: %v kB", m.HeapAlloc/1024),
			Metric:      MetricCounter},
		{
			Service:     "Heap Sys",
			Host:        host,
			State:       "OK",
			Value:       m.HeapSys,
			Description: fmt.Sprintf("Heap Sys: %v kB", m.HeapSys/1024),
			Metric:      MetricCounter},
		{
			Service:     "Heap Idle",
			Host:        host,
			State:       "OK",
			Value:       m.HeapIdle,
			Description: fmt.Sprintf("Heap Idle: %v kB", m.HeapIdle/1024),
			Metric:      MetricCounter},
		{
			Service:     "Heap Inuse",
			Host:        host,
			State:       "OK",
			Value:       m.HeapInuse,
			Description: fmt.Sprintf("Heap Inuse: %v kB", m.HeapInuse/1024),
			Metric:      MetricCounter},
		{
			Service:     "Heap Released",
			Host:        host,
			State:       "OK",
			Value:       m.HeapReleased,
			Description: fmt.Sprintf("Heap Released: %v kB", m.HeapReleased/1024),
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
