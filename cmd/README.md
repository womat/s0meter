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
| `--version` | `false`                        | Print the application version and exit                              |
| `--about`   | `false`                        | Print application details and exit                                  |
| `--help`    | `false`                        | Print this help message and exit                                    |

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
Environment variables are expanded inside the file, e.g. `apiKey: ${TADL_API_KEY}`.
