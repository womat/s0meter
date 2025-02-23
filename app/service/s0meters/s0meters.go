// Package s0meters provides functionality for managing multiple S0 meters
// and publishing their data over MQTT.
//
// This package allows the registration of multiple S0 pulse meters, tracks their
// counter-values, calculates flow rates (gauge values), and publishes the data
// via MQTT. It supports both real and emulated GPIO meters.
//
// Features:
// - Manage multiple S0 meters with individual configurations
// - Read pulse counters and compute gauge values
// - Publish meter data over MQTT
// - Supports debouncing for reliable pulse detection
// - Periodic data backup to a YAML file
//
// Example Usage:
//
//	config := meters.Config{
//	    DataFile:               "meter_data.yaml",
//	    BackupInterval:         300,
//	    DataCollectionInterval: 60,
//	    MqttRetained:           true,
//	    MqttTopic:              "s0meters",
//	}
//
//	meterHandler := meters.New(config)
//
//	meterHandler.AddMeter("power", meters.MeterConfig{
//	    Gpio:            17,
//	    BounceTime:      100,
//	    CounterConstant: 1000,
//	    UnitCounter:     "kWh",
//	    ScaleFactor:     1.0,
//	    Precision:       2,
//	    UnitGauge:       "kW",
//	    MqttTopic:       "meters/power",
//	})
//
//	err := meterHandler.Connect("tcp://mqtt.example.com:1883")
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	defer meterHandler.Close()
//
// Note: This package requires a Raspberry Pi with access to GPIO ports.
package s0meters

import (
	"encoding/json"
	"errors"
	"fmt"
	"gopkg.in/yaml.v3"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"s0counter/pkg/mqtt"
	"s0counter/pkg/s0reader"
	"sync"
	"time"
)

type Config struct {
	DataFile               string
	BackupInterval         int
	DataCollectionInterval int
	MqttRetained           bool
	MqttTopic              string
}

type Handler struct {
	sync.RWMutex
	config      Config
	mqttEnabled bool

	// mqtt is the handler to the mqtt broker
	mqtt *mqtt.Handler

	// MetersMap must be a pointer to the MeterHandler type, otherwise RWMutex doesn't work!
	meters map[string]MeterHandler
}

// MeterConfig holds the configuration details for a specific meter.
// It defines the GPIO pin for pulse reading, debounce time for reliability,
// counter constant for scaling, and the units for both counter and gauge measurements.
type MeterConfig struct {
	Gpio            int     `yaml:"gpio"`            // GPIO pin number for pulse reading
	BounceTime      int     `yaml:"bounceTime"`      // Debounce time in milliseconds to stabilize pulse signals
	CounterConstant float64 `yaml:"counterConstant"` // Constant to scale the counter readings (e.g., for calibration)
	UnitCounter     string  `yaml:"unitCounter"`     // Unit for the counter value (e.g., kWh, liters)
	ScaleFactor     float64 `yaml:"scaleFactor"`     // Factor to scale the gauge value (e.g., for calibration or unit conversion)
	Precision       int     `yaml:"precision"`       // Precision for the counter and gauge values
	UnitGauge       string  `yaml:"unitGauge"`       // Unit for the gauge value (e.g., kW, liters per hour)
	MqttTopic       string  `yaml:"mqttTopic"`       // MQTT topic for publishing meter data
}

// MeterHandler is a struct that holds the meter configuration and the meter handler.
type MeterHandler struct {
	Config MeterConfig
	Meter  *s0reader.Handler
}

// MQTTMessage is the struct to store the MQTT message data
type MQTTMessage struct {
	TimeStamp   time.Time // timestamp of last gauge calculation
	Counter     float64   // current counter value (e.g., kWh, l, m³)
	UnitCounter string    // unit of the current meter counter (e.g., kWh, l, m³)
	Gauge       float64   // mass flow rate per time unit  (= counter/time(h)), e.g., kW, l/h, m³/h
	UnitGauge   string    // unit of the gauge (e.g., Wh, l/s, m³/h
}

type Data struct {
	TimeStamp   time.Time `json:"TimeStamp"`   // timestamp of last gauge calculation
	Counter     float64   `json:"Counter"`     // current counter (aktueller Zählerstand), eg kWh, l, m³
	UnitCounter string    `json:"UnitCounter"` // unit of current meter counter e.g., kWh, l, m³
	Gauge       float64   `json:"Gauge"`       // mass flow rate per time unit  (= counter/time(h)), e.g. kW, l/h, m³/h
	UnitGauge   string    `json:"UnitGauge"`   // unit of gauge, eg Wh, l/s, m³/h
}

// New initializes and returns a new meter handler.
//
// This function sets up the meters management system, including MQTT integration.
// The configuration defines file storage, MQTT settings, and data intervals.
func New(config Config) *Handler {
	return &Handler{
		config:      config,
		mqttEnabled: false,
		mqtt:        mqtt.New(),
		meters:      make(map[string]MeterHandler),
	}
}

// AddMeter registers a new meter and initializes its GPIO port.
//
// Parameters:
// - `name`: Unique identifier for the meter.
// - `config`: Configuration details for the meter (GPIO, debounce time, scaling).
//
// Returns an error if the GPIO port initialization fails.
func (h *Handler) AddMeter(name string, config MeterConfig) error {

	m := MeterHandler{
		Config: config,
		Meter:  s0reader.New(),
	}

	slog.Info("Initializing GPIO port", "port", config.Gpio, "bounceTime_ms", config.BounceTime)
	if err := m.Meter.InitializePort(m.Config.Gpio, time.Duration(m.Config.BounceTime)*time.Millisecond); err != nil {
		slog.Error("Failed to initialize GPIO port", "port", m.Config.Gpio, "error", err)
		return err
	}

	h.Lock()
	h.meters[name] = m
	h.Unlock()
	return nil
}

// GetMeters returns the current counter and gauge values for all meters.
//
// The result is a map where keys are meter names, and values contain
// the timestamp, counter-value, and gauge value.
func (h *Handler) GetMeters() map[string]Data {
	h.RLock()
	defer h.RUnlock()

	data := map[string]Data{}
	for n, m := range h.meters {
		data[n] = Data{
			TimeStamp:   time.Now(),
			Counter:     calcCounter(m),
			UnitCounter: m.Config.UnitCounter,
			Gauge:       calcGauge(m),
			UnitGauge:   m.Config.UnitGauge,
		}
	}

	return data
}

// GetMeter retrieves the current counter and gauge values for a specific meter.
//
// This function safely retrieves the latest measurement data for the given meter name.
// It returns the current counter value, gauge calculation, and associated units.
//
// Parameters:
// - `name`: The identifier of the meter to retrieve.
//
// Returns:
// - A `Data` struct containing the timestamp, counter-value, and gauge value.
// - An error if the meter does not exist.
//
// Example Usage:
//
//	meterData, err := meterHandler.GetMeter("power")
//	if err != nil {
//	    log.Println("Meter not found:", err)
//	} else {
//	    fmt.Println("Power Meter Data:", meterData)
//	}
func (h *Handler) GetMeter(name string) (Data, error) {
	h.RLock()
	defer h.RUnlock()

	m, ok := h.meters[name]
	if !ok {
		return Data{}, fmt.Errorf("meter %s not found", name)
	}

	data := Data{
		TimeStamp:   time.Now(),
		Counter:     calcCounter(m),
		UnitCounter: m.Config.UnitCounter,
		Gauge:       calcGauge(m),
		UnitGauge:   m.Config.UnitGauge,
	}

	return data, nil
}

// Connect establishes a connection to an MQTT broker.
//
// This function attempts to connect to the specified MQTT broker using the configured
// MQTT handler. If the connection is successful, MQTT publishing is enabled.
//
// Parameters:
// - `broker`: The address of the MQTT broker (e.g., "tcp://mqtt.example.com:1883").
//
// Returns:
// - `nil` if the connection is successful.
// - An error if the connection attempt fails.
//
// Example Usage:
//
//	err := meterHandler.Connect("tcp://mqtt.example.com:1883")
//	if err != nil {
//	    log.Fatal("Failed to connect to MQTT broker:", err)
//	}
func (h *Handler) Connect(broker string) error {
	slog.Info("Connecting to MQTT broker", "broker", broker)
	if err := h.mqtt.Connect(broker); err != nil {
		slog.Error("Failed to connect to MQTT broker", "broker", broker, "error", err)
		return err
	}

	h.mqttEnabled = true
	return nil
}

// Close shuts down all meters, disconnects from MQTT, and saves meter data.
//
// This function ensures a clean shutdown by:
// - Closing all registered meters to stop pulse monitoring
// - Disconnecting from the MQTT broker (if enabled)
// - Persisting the current meter data to a YAML file
//
// Errors from multiple operations are collected and returned as a single error.
//
// Returns:
// - `nil` if all operations succeed
// - An aggregated error if one or more operations fail
//
// Example Usage:
//
//	err := meterHandler.Close()
//	if err != nil {
//	    log.Fatal("Error closing meters:", err)
//	}
func (h *Handler) Close() error {
	h.RLock()
	defer h.RUnlock()

	var err error
	for _, m := range h.meters {
		slog.Debug("Closing meter", "name", m.Config.Gpio)
		// close the LineHandler channel to stop the meter
		err = errors.Join(err, m.Meter.Close())
	}

	if h.mqttEnabled {
		slog.Debug("Disconnecting from MQTT broker")
		err = errors.Join(err, h.mqtt.Disconnect())
	}

	slog.Debug("Saving meter data")
	err = errors.Join(err, h.saveMeterData())
	return err
}

// PublishMetrics periodically calculates and publishes meter values via MQTT.
//
// This function runs an infinite loop that triggers at a configurable interval
// (defined in `DataCollectionInterval`). It calculates the latest gauge values
// for all registered meters and publishes them asynchronously via MQTT.
//
// Each meter's data is processed concurrently to ensure minimal delay.
//
// Example Usage:
//
//	go meterHandler.PublishMetrics()  // Run in a separate goroutine
//
// Note:
// - This function should be executed in a separate goroutine, as it runs indefinitely.
// - The interval is set in `h.config.DataCollectionInterval` (in seconds).
// - If no meters are registered, the function does nothing.
// - If the MQTT connection is lost, metrics will not be sent but will continue to be calculated.
func (h *Handler) PublishMetrics() {

	t := time.Duration(h.config.DataCollectionInterval) * time.Second
	ticker := time.NewTicker(t)
	defer ticker.Stop()

	for range ticker.C {
		h.RLock()
		for n := range h.meters {
			go h.publishMetric(n)
		}
		h.RUnlock()
	}
}

// publishMetric sends the current counter and gauge values of a specific meter to the MQTT broker.
//
// This function retrieves the latest counter and gauge values for a given meter,
// formats them as a JSON payload, and publishes the data to the configured MQTT topic.
//
// Parameters:
// - `n`: The name of the meter whose values should be published.
//
// Returns:
// - `nil` if the MQTT message was successfully sent.
// - An error if JSON serialization or MQTT publishing fails.
//
// Example Usage:
//
//	err := meterHandler.publishMetric("power")
//	if err != nil {
//	    log.Println("Failed to publish metric:", err)
//	}
//
// Note:
// - The MQTT topic is defined in `h.config.MqttTopic`.
// - The function uses `h.config.MqttRetained` to determine whether messages should be retained.
// - If an error occurs during JSON marshaling or MQTT publishing, it is logged and returned.
func (h *Handler) publishMetric(n string) error {
	h.RLock()
	m := h.meters[n]
	h.RUnlock()

	if m.Config.MqttTopic == "" {
		return nil
	}

	payload := MQTTMessage{
		TimeStamp:   time.Now(),
		Counter:     calcCounter(m),
		UnitCounter: m.Config.UnitCounter,
		Gauge:       calcGauge(m),
		UnitGauge:   m.Config.UnitGauge,
	}

	b, err := json.Marshal(payload)
	if err != nil {
		slog.Error("Failed to marshal MQTT message", "meter", n, "error", err)
		return err
	}

	msg := mqtt.Message{
		Qos:      0,
		Retained: h.config.MqttRetained,
		Topic:    h.config.MqttTopic,
		Payload:  b,
	}

	slog.Debug("Publishing MQTT message", "meter", n, "topic", m.Config.MqttTopic, "record", payload)
	if err = h.mqtt.Publish(msg); err != nil {
		slog.Error("Failed to publish MQTT message", "meter", n, "topic", msg.Topic, "error", err)
	}

	return err
}

// LoadMeterData restores the last saved counter values for all meters from a YAML file.
//
// This function reads the meter data from the configured YAML file and updates the
// counter values of all registered meters accordingly. If the file does not exist,
// it initializes a new file with default values by calling `saveMeterData()`.
//
// If an error occurs while reading or parsing the file, it is returned.
//
// Returns:
// - `nil` if the data is successfully loaded or initialized.
// - An error if the file cannot be read or the YAML content is invalid.
//
// Example Usage:
//
//	err := meterHandler.LoadMeterData()
//	if err != nil {
//	    log.Fatal("Failed to load meter data:", err)
//	}
//
// Note:
// - If a meter exists in the YAML file but is not registered in the handler, its data is ignored.
// - If a registered meter is missing in the YAML file, its counter remains unchanged.
func (h *Handler) LoadMeterData() error {

	// read the YAML file as a byte array.
	data, err := os.ReadFile(h.config.DataFile)
	if os.IsNotExist(err) {
		// if the file doesn't exist, create it with default values
		slog.Info("Config file not found, using default values", "file", h.config.DataFile)
		return h.saveMeterData()
	}

	if err != nil {
		return err
	}

	// unmarshal the byte slice which contains the YAML file's content into SaveMeters struct
	s := make(map[string]s0reader.Counter)
	if err = yaml.Unmarshal(data, &s); err != nil {
		return err
	}

	h.Lock()
	defer h.Unlock()

	for name, loadedMeter := range s {
		if m, ok := h.meters[name]; ok {
			m.Meter.SetCounter(loadedMeter)
		}
	}

	return nil
}

// BackupMeterData periodically saves the current counter and gauge values of each meter to a YAML file.
//
// This function runs a background process that periodically triggers `saveMeterData()` at an interval
// defined in the configuration (`BackupInterval`). The backup ensures that the latest meter readings
// are stored persistently to prevent data loss in case of a system reboot or failure.
//
// If `saveMeterData()` encounters an error, a log entry is generated.
//
// Example Usage:
//
//	go meterHandler.BackupMeterData()  // Run in a separate goroutine
//
// Note:
// - This function runs indefinitely and should be executed in a separate goroutine.
func (h *Handler) BackupMeterData() {

	t := time.Duration(h.config.BackupInterval) * time.Second
	ticker := time.NewTicker(t)
	defer ticker.Stop()
	for range ticker.C {
		slog.Debug("Starting meter data backup", "file", h.config.DataFile)

		if err := h.saveMeterData(); err != nil {
			slog.Error("Failed to backup meter data", "file", h.config.DataFile, "error", err)
		}
	}
}

// saveMeterData stores the current counter and gauge values of all meters in a YAML file.
//
// This function retrieves the latest counter values for all registered meters,
// serializes the data into YAML format, and writes it to the specified file.
// If the file does not exist, it will be created. If it exists, it will be overwritten.
//
// The file is saved with `0600` permissions to restrict access to the owner.
//
// Returns:
// - `nil` if the data is successfully saved.
// - An error if marshaling or file writing fails.
//
// Example Usage:
//
//	err := meterHandler.saveMeterData()
//	if err != nil {
//	    log.Fatal("Failed to save meter data:", err)
//	}
func (h *Handler) saveMeterData() error {

	s := make(map[string]s0reader.Counter)

	h.RLock()
	for name, m := range h.meters {
		c := m.Meter.GetCounter()
		s[name] = c
	}
	h.RUnlock()

	// marshal the byte slice which contains the YAML file's content into SaveMeters struct
	data, err := yaml.Marshal(&s)
	if err != nil {
		return err
	}

	// check if the directory exists
	if _, err = os.Stat(filepath.Dir(h.config.DataFile)); os.IsNotExist(err) {
		return fmt.Errorf("directory does not exist to write file %s", h.config.DataFile)
	}

	if err = os.WriteFile(h.config.DataFile, data, 0o600); err != nil {
		return err
	}

	return nil
}

// calcGauge calculates the real-time consumption rate (gauge value) for a meter.
//
// This function determines the consumption rate (e.g., kW, l/h, m³/h) by
// analyzing the time difference between the last two pulses. If no new pulses
// have been received for a while, it adjusts the duration to avoid a zero reading.
//
// Parameters:
// - `m`: The meter handler containing the counter and configuration.
//
// Returns:
//   - The calculated gauge value as a floating-point number, rounded to the
//     configured precision. Returns `0` if the duration or counter constant is `0`
//     to avoid division by zero.
//
// Calculation Formula:
// - `P = (Tick Count * 3600) / (Time Interval in Seconds * Counter Constant) * Scale Factor`
//
// Logic:
//   - If no new pulse has been received for a long time, it adjusts the duration
//     to use the time difference between now and the last recorded pulse.
//
// Example Usage:
//
//	gauge := calcGauge(meterHandler)
//	fmt.Println("Current gauge value:", gauge)
func calcGauge(m MeterHandler) (f float64) {
	c := m.Meter.GetCounter()
	dt := c.TimeStamp.Sub(c.LastTimeStamp) // duration between last two ticks

	// if duration between "now and last tick" is greater than the duration between "last two ticks"
	// increase duration between "last two ticks" to duration between "now and the penultimate tick".
	// this ensures that the value of the gauge value changes if no new pulse has been received
	if time.Since(c.TimeStamp) > dt {
		dt = time.Since(c.LastTimeStamp)
	}

	// avoid division by zero
	if dt.Seconds() <= 0 || m.Config.CounterConstant == 0 {
		return 0
	}

	// P = 3600 / (t * Cz) (Zählerkonstante in Imp/kWh)
	f = float64(c.Tick) * 3600 / (dt.Seconds() * m.Config.CounterConstant) * m.Config.ScaleFactor
	return toFixed(f, m.Config.Precision)
}

// calcCounter calculates the total counter value for a meter.
//
// This function converts the number of recorded S0 pulses (ticks) into the actual
// measured unit (e.g., kWh, liters, cubic meters) by dividing the tick count
// by the meter's configured counter constant.
//
// Parameters:
// - `m`: The meter handler containing the counter and configuration.
//
// Returns:
//   - The computed counter value as a floating-point number. Returns `0` if the
//     counter constant is `0` to avoid division by zero.
//
// Example Usage:
//
//	counter := calcCounter(meterHandler)
//	fmt.Println("Current counter value:", counter)
func calcCounter(m MeterHandler) (f float64) {
	c := m.Meter.GetCounter()
	if m.Config.CounterConstant == 0 {
		return 0
	}
	return float64(c.Tick) / m.Config.CounterConstant
}

// toFixed rounds a floating-point number to a specified precision.
//
// This function is used to limit the number of decimal places in a
// floating-point result, ensuring consistent output formatting.
//
// Parameters:
// - `num`: The number to be rounded.
// - `precision`: The number of decimal places to round to.
//
// Returns:
// - A floating-point number rounded to the specified precision.
//
// Example Usage:
//
//	roundedValue := toFixed(3.141592, 2)  // Returns: 3.14
func toFixed(num float64, precision int) float64 {
	p := math.Pow(10, float64(precision))
	return math.Round(num*p) / p
}
