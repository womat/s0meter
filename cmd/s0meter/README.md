# s0meter

**s0meter** reads impulses from S0 interfaces compliant with DIN 43864 standards, calculates energy counters and flow
rates, and publishes the results via MQTT.

## Features

- Counts S0 pulses from GPIO pins (Raspberry Pi)
- Applies debouncing to filter signal noise
- Calculates total counters and flow rates (gauge values)
- Publishes data via MQTT
- Persists counters to a YAML file for recovery after restart
- Exposes an HTTPS API for live readings
- Supports hot-reload of configuration

---

## Command-line Flags

| Flag        | Default                        | Description                                                         |
|-------------|--------------------------------|---------------------------------------------------------------------|
| `--config`  | `/opt/s0meter/etc/config.yaml` | Path to the configuration file                                      |
| `--debug`   | `false`                        | Enable debug logging to stdout (overrides log settings from config) |
| `--version` |                                | Print the application version and exit                              |
| `--about`   |                                | Print application details and exit                                  |
| `--help`    |                                | Print this help message and exit                                    |

The config file path can also be set via the environment variable `CONFIG_FILE`.

**Examples:**

```bash
s0meter --config /etc/s0meter/config.yaml
s0meter --debug
s0meter --version
CONFIG_FILE=/etc/s0meter/config.yaml s0meter
```

---

## Configuration

The configuration file is a YAML file. By default it is loaded from `/opt/s0meter/etc/config.yaml`.

### Full Example

```yaml
# =============================================================================
# s0meter configuration
# =============================================================================

# logLevel defines the minimum log level.
# Allowed values: debug | info | warn | error
logLevel: info

# logDestination defines where logs are written to.
# Supported values: stdout | stderr | /path/to/logfile
logDestination: stdout

# =============================================================================
# Webserver configuration (HTTPS)
# =============================================================================
webserver:
  # Host address the HTTPS server listens on (0.0.0.0 = all interfaces)
  listenHost: 0.0.0.0

  # Port the HTTPS server listens on
  listenPort: 443

  # Global API key for protected endpoints
  apiKey: changeme!

  # TLS private key file
  keyFile: /opt/s0meter/etc/key.pem

  # TLS certificate file
  certFile: /opt/s0meter/etc/cert.pem

  # Blocked IP addresses or networks (empty = none blocked)
  blockedIPs: [ ]
  #  - 192.168.0.1
  #  - 192.168.0.0/16

  # Allowed IP addresses or networks (empty = all allowed)
  allowedIPs: [ ]
  #  - 127.0.0.1
  #  - ::1
  #  - 192.168.0.0/16

# =============================================================================
# Data collection
# =============================================================================

# File where counters are persisted
dataFile: /opt/s0meter/data/s0meter.yaml

# Interval in seconds for saving counters to dataFile
backupInterval: 60

# =============================================================================
# MQTT configuration (disabled when connection is empty)
# =============================================================================
mqtt:
  # Broker connection string (empty = MQTT disabled)
  connection: "tcp://raspberrypi4.fritz.box:1883"

  # Retain messages on the broker
  retained: false

  # Publish interval in seconds
  publishInterval: 10

# =============================================================================
# S0 Meter configurations
# =============================================================================
meter:
  wallbox:
    gpio: 17
    bounceTime: 1
    unitCounter: "kWh"
    ticksPerUnit: 1000
    unitGauge: "kW"
    scaleFactor: 1
    precision: 2
    mqttTopic: test/wallbox/summary

  greywater:
    gpio: 27
    bounceTime: 1
    unitCounter: "l"
    ticksPerUnit: 1
    unitGauge: "l/h"
    scaleFactor: 1
    precision: 0
    mqttTopic: test/rawwater/summary

  drinkingwater:
    gpio: 22
    bounceTime: 1
    unitCounter: "m³"
    ticksPerUnit: 1000
    unitGauge: "l/s"
    scaleFactor: 0.2777778
    precision: 3
    mqttTopic: test/portablewater/summary
```

### Meter Configuration Reference

| Field          | Type     | Description                                                                      |
|----------------|----------|----------------------------------------------------------------------------------|
| `gpio`         | int      | GPIO pin number for S0 pulse input                                               |
| `bounceTime`   | int (ms) | Debounce time in milliseconds to suppress signal noise                           |
| `unitCounter`  | string   | Unit of the total counter (e.g. `kWh`, `m³`, `l`)                                |
| `ticksPerUnit` | float    | Pulses per unit — see meter datasheet (Zählerkonstante)                          |
| `unitGauge`    | string   | Unit of the flow rate (e.g. `kW`, `l/h`, `l/s`)                                  |
| `scaleFactor`  | float    | Scale factor applied to the gauge value (e.g. `0.2777778` to convert m³/h → l/s) |
| `precision`    | int      | Number of decimal places for the gauge value                                     |
| `mqttTopic`    | string   | MQTT topic to publish to (empty = not published)                                 |