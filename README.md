# s0meter — S0 Pulse Energy Monitor

s0meter collects and exposes data from one or more **S0 pulse energy meters** compliant with **DIN 43864**,
supporting electricity, water, and gas meters. It runs efficiently on **Raspberry Pi** hardware (tested on Raspberry Pi
Zero and above).

---

## Features

- Reads S0 pulse signals via GPIO
- Calculates energy counters and flow rates (gauge)
- Exposes data via HTTPS REST API (with API key or JWT authentication)
- Publishes data to an MQTT broker
- IP allowlist / blocklist support
- Persists counter data to disk

---

## API Endpoints

| Method | Path       | Auth    | Description                        |
|--------|------------|---------|------------------------------------|
| GET    | `/version` | —       | Application name and version       |
| GET    | `/health`  | API Key | Runtime metrics (memory, uptime …) |
| GET    | `/data`    | API Key | Latest sensor readings             |

Authentication via the `X-API-Key` header.

### Examples

```sh
# List meters
curl -k -H "X-Api-Key: your-api-key" https://localhost:8443/meters

# Get meter data
curl -k -H "X-Api-Key: your-api-key" https://localhost:8443/meters/{name}

# Application version (no auth required)
curl -k https://localhost:8443/version

# Health check
curl -k -H "X-Api-Key: your-api-key" https://localhost:8443/health
```

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

Default location: `/opt/s0meter/etc/config.yaml`
Environment variables are expanded inside the file, e.g. `apiKey: ${TADL_API_KEY}`.


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

  # Port the HTTPS server listens on (default: 8443)
  listenPort: 8443

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
    counterUnit: "kWh"
    counterPulsesPerUnit: 1000
    counterPrecision: 2
    gaugeUnit: "kW"
    gaugeScale: 1
    gaugePrecision: 2
    mqttTopic: test/wallbox/summary

  greywater:
    gpio: 27
    bounceTime: 1
    counterUnit: "l"
    counterPulsesPerUnit: 1
    counterPrecision: 0
    gaugeUnit: "l/h"
    gaugeScale: 1
    gaugePrecision: 0
    mqttTopic: test/rawwater/summary

  drinkingwater:
    gpio: 22
    bounceTime: 1
    counterUnit: "m³"
    counterPulsesPerUnit: 1000
    counterPrecision: 3
    gaugeUnit: "l/s"
    gaugeScale: 0.2777778
    gaugePrecision: 3
    mqttTopic: test/portablewater/summary
```

### Meter Configuration Reference

| Field                  | Type     | Description                                                                      |
|------------------------|----------|----------------------------------------------------------------------------------|
| `gpio`                 | int      | GPIO pin number for S0 pulse input                                               |
| `bounceTime`           | int (ms) | Debounce time in milliseconds to suppress signal noise                           |
| `counterUnit`          | string   | Unit of the total counter (e.g. `kWh`, `m³`, `l`)                                |
| `gaugeUnit`            | string   | Unit of the flow rate (e.g. `kW`, `l/h`, `l/s`)                                  |
| `counterPulsesPerUnit` | float    | Meter constant (Zählerkonstante): pulses per counterUnit                         |
| `gaugeScale`           | float    | Scale factor applied to the gauge value (e.g. `0.2777778` to convert m³/h → l/s) |
| `counterPrecision`     | int      | Number of decimal places for the counter value                                   |
| `gaugePrecision`       | int      | Number of decimal places for the gauge value                                     |
| `mqttTopic`            | string   | MQTT topic to publish to (empty = not published)                                 |

---

## TLS Certificate

Generate a self-signed certificate for development:

```sh
openssl req -x509 -nodes -newkey rsa:2048 \
  -keyout /opt/s0meter/etc/key.pem \
  -out /opt/s0meter/etc/cert.pem \
  -days 825 \
  -subj "/C=AT/ST=Vienna/L=Vienna/O=MyCompany/OU=DEV/CN=localhost"
```

**Subject fields:**

| Field           | Example             | Description                                  |
|-----------------|---------------------|----------------------------------------------|
| `/C`            | `AT`                | Country code (2 letters)                     |
| `/ST`           | `Vienna`            | State or province (optional)                 |
| `/L`            | `Vienna`            | City (optional)                              |
| `/O`            | `MyCompany`         | Organization (optional)                      |
| `/OU`           | `DEV`               | Organizational unit (optional)               |
| `/CN`           | `localhost`         | **Common Name — your domain or `localhost`** |
| `/emailAddress` | `admin@example.com` | E-mail address (optional)                    |

> **Note:** Browsers enforce a maximum certificate validity of 825 days. Use `-days 365` for production-like setups.

---

## Installation

### 1. Create system user and directories

```sh
sudo groupadd -f s0meter
sudo useradd -r -s /usr/sbin/nologin -g s0meter s0meter
sudo usermod -aG gpio s0meter

sudo mkdir -p /opt/s0meter/{bin,etc,data}
sudo chown -R s0meter:s0meter /opt/s0meter
```

### 2. Copy files

```sh
sudo cp s0meter /opt/s0meter/bin/
sudo cp config.yaml /opt/s0meter/etc/
sudo cp cert.pem key.pem /opt/s0meter/etc/
sudo chown -R s0meter:s0meter /opt/s0meter
```

### 3. Create systemd service

```sh
sudo tee /etc/systemd/system/s0meter.service > /dev/null <<'EOF'
[Unit]
Description=s0meter — S0 Pulse Energy Monitor
After=network.target

[Service]
User=s0meter
Group=s0meter
Type=simple
ExecStart=/opt/s0meter/bin/s0meter 
Restart=on-failure

[Install]
WantedBy=multi-user.target
EOF

sudo systemctl daemon-reload
sudo systemctl enable s0meter
sudo systemctl start s0meter
sudo systemctl status s0meter
```

### 4. View logs

```sh
journalctl -u s0meter -n 50 -f
```

---

## Build

```sh
# Raspberry Pi 4/5 (64-bit OS)
make build_arm64

# Raspberry Pi 2/3/4 (32-bit OS)
make build_arm7

# Raspberry Pi 1 / Zero (32-bit OS)
make build_arm6

# Build with Swagger UI (dev only)
make build_arm64_dev

# Build and deploy to Raspberry Pi via SCP
make deploy
```

---

## Hot-Reload

Send `SIGHUP` to reload the configuration without restarting the process:

```sh
sudo systemctl reload s0meter
# or
kill -HUP $(pidof s0meter)
```

---

## Firewall

```sh
# Allow the configured port (default 8443)
sudo ufw allow 8443/tcp
sudo ufw status
```

---

## Backup & Restore

```sh
# Backup
sudo tar czf /tmp/s0meter-backup.tar.gz /opt/s0meter

# Restore
sudo tar xzf /tmp/s0meter-backup.tar.gz -C /
sudo chown -R s0meter:s0meter /opt/s0meter
sudo systemctl restart s0meter
```

---

# License

MIT