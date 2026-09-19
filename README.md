# s0meter — S0 Pulse Energy Monitor

s0meter collects and exposes data from one or more **S0 pulse energy meters** compliant with **DIN 43864**,
supporting electricity, water, and gas meters. It runs efficiently on **Raspberry Pi** hardware (tested on Raspberry Pi
Zero and above).

---

## Features

- Counts **S0 pulses** from GPIO pins (Raspberry Pi)
- Applies **debouncing** to filter signal noise
- Calculates **total counters** and **flow rates** (gauge values)
- **Persists counters** to a YAML file for recovery after restart
- Publishes to an **MQTT broker** on every counted pulse (throttled), plus a periodic heartbeat
- Exposes a secured **HTTPS REST API** (API key authentication)
- **IP allowlist / blocklist** support
- **Hot-reload** of configuration via `SIGHUP`
- Embedded self-signed TLS certificate for development (no setup required)
- Optional **Swagger UI** (build tag `swagger`, dev only)

---

## Where to start

- Runtime, API, build, deploy, and Swagger usage: [`cmd/README.md`](cmd/README.md)
- Example configuration: [`config/config.yaml`](config/config.yaml)
- Swagger generation script: [`docs/generate.sh`](docs/generate.sh)

---

## API Endpoints

| Method | Path             | Auth    | Description                               |
|--------|------------------|---------|-------------------------------------------|
| GET    | `/version`       | —       | Application name and version              |
| GET    | `/ready`         | —       | Readiness probe: 200 ready, 503 otherwise |
| GET    | `/health`        | API Key | Runtime metrics (memory, uptime …)        |
| GET    | `/meters`        | API Key | Current reading of all meters             |
| GET    | `/meters/{name}` | API Key | Current reading of a single meter         |

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

# environment: dev | prod
env: prod

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

# Interval as Go duration string (e.g. 60s) for saving counters to dataFile
backupInterval: 60s

# =============================================================================
# MQTT configuration (disabled when connection is empty)
# =============================================================================
mqtt:
  # Broker connection string (empty = MQTT disabled)
  connection: "tcp://mqtt.example.com:1883"

  # Heartbeat: every meter is published at least this often, as a Go duration string
  publishInterval: 60s

  # How often the publish loop checks for new pulses. A meter is published as soon as its
  # counter advances, but never more than once per minPublishInterval - this throttles a
  # fast pulsing meter. Set to 0 to publish on the heartbeat only.
  minPublishInterval: 2s

# =============================================================================
# S0 Meter configurations
# =============================================================================
meter:
  wallbox:
    gpio: 17
    debounceTime: 1ms
    counterUnit: "kWh"
    counterPulsesPerUnit: 1000
    counterPrecision: 2
    gaugeUnit: "kW"
    gaugeScale: 1
    gaugePrecision: 2
    mqttTopic: test/wallbox/summary
    mqttRetained: true

  greywater:
    gpio: 27
    debounceTime: 1ms
    counterUnit: "l"
    counterPulsesPerUnit: 1
    counterPrecision: 0
    gaugeUnit: "l/h"
    gaugeScale: 1
    gaugePrecision: 0
    mqttTopic: test/rawwater/summary

  drinkingwater:
    gpio: 22
    debounceTime: 1ms
    counterUnit: "m³"
    counterPulsesPerUnit: 1000
    counterPrecision: 3
    gaugeUnit: "l/s"
    gaugeScale: 0.2777778
    gaugePrecision: 3
    mqttTopic: test/portablewater/summary
```

### Meter Configuration Reference

| Field                  | Type   | Description                                                                      |
|------------------------|--------|----------------------------------------------------------------------------------|
| `gpio`                 | int    | GPIO pin number for S0 pulse input                                               |
| `debounceTime`         | string | Debounce as Go duration string — see [Choosing a debounce time](#choosing-a-debounce-time) |
| `counterUnit`          | string | Unit of the total counter (e.g. `kWh`, `m³`, `l`)                                |
| `gaugeUnit`            | string | Unit of the flow rate (e.g. `kW`, `l/h`, `l/s`)                                  |
| `counterPulsesPerUnit` | float  | Meter constant (Zählerkonstante): pulses per counterUnit                         |
| `gaugeScale`           | float  | Scale factor applied to the gauge value (e.g. `0.2777778` to convert m³/h → l/s) |
| `counterPrecision`     | int    | Number of decimal places for the counter value                                   |
| `gaugePrecision`       | int    | Number of decimal places for the gauge value                                     |
| `mqttTopic`            | string | MQTT topic to publish to (empty = not published)                                 |
| `mqttRetained`         | bool   | Broker keeps the last message of this topic (default: `false`)                   |

### Choosing a debounce time

`debounceTime` is handed to the kernel's GPIO debouncer: an edge is reported only after the line has
been stable for the full period. **The value must therefore be shorter than the pulse itself** — a
debounce longer than the pulse swallows it entirely and the meter under-counts.

| Meter type                        | Output                     | Bounce behaviour                       | Suggested |
|-----------------------------------|----------------------------|----------------------------------------|-----------|
| Electricity, S0 per DIN 43864     | opto-coupler, electronic   | does not bounce; noise is picked up on the cable | `10ms`    |
| Water / gas with a pulse output   | reed switch, mechanical    | bounces 0.5–2 ms, more at low flow     | `20ms`    |

DIN 43864 specifies a minimum pulse duration of **30 ms**, so up to roughly 10 ms is safe for an S0
meter — do not approach 30 ms. A reed switch bounces mechanically, and `1ms` sits inside that bounce
window: the bounces are then counted as extra pulses.

Pulse spacing is rarely the limit. At 1000 imp/kWh an 11 kW load still leaves 327 ms between pulses
(164 ms at 22 kW), and a water meter at 1000 imp/m³ leaves over 700 ms even at 5 m³/h.

> `pkg/pulsecounter` watches the rising edge only, so the pulse *width* cannot be measured from the
> logs. Take it from the meter's data sheet and stay well below it.

Compare the counter against the physical meter after a few weeks: a value that runs ahead points to
bouncing (increase the debounce), one that falls behind points to a debounce longer than the pulse,
or to a wiring fault.

### Correcting a counter

`dataFile` stores **pulses**, not the scaled reading:

```
pulses = counter × counterPulsesPerUnit
```

The running process rewrites that file every `backupInterval` and again on shutdown, so it has to be
stopped before editing — otherwise the change is overwritten:

```sh
sudo systemctl stop s0meter
sudo nano /opt/s0meter/data/s0meter.yaml    # adjust "pulses:"
sudo systemctl start s0meter
```

Two worked examples:

| Physical reading | `counterUnit` | `counterPulsesPerUnit` | `pulses:` |
|------------------|---------------|------------------------|-----------|
| 34270.83 kWh     | `Wh`          | 1                      | 34270830  |
| 105.459 m³       | `m³`          | 1000                   | 105459    |

---

## MQTT Publishing

A meter is published **as soon as its counter advances** — that is, as soon as a new pulse has been
counted — and in any case once per `publishInterval` (the heartbeat). Between two pulses only the
gauge decays, and that alone does not trigger a message.

`minPublishInterval` is how often the loop looks for new pulses. It is therefore both the worst-case
delay of a pulse and the shortest spacing between two messages of the same meter, which is what keeps
a fast meter from flooding the broker: a wallbox charging at 11 kW with one pulse per Wh produces
about three pulses per second, which `minPublishInterval: 2s` caps at 30 messages per minute. Set it
to `0` to publish on the heartbeat only.

While the broker connection is down, publishing is skipped instead of blocking on the publish
timeout, and resumes automatically once the client reconnects.

> When pulses stop, the last published gauge stands until the next heartbeat. A long
> `publishInterval` therefore delays the "flow stopped" signal by up to that interval — lower it if
> consumers need to see a stop quickly.

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
sudo groupadd -r -f s0meter
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
After=network-online.target
Wants=network-online.target

[Service]
User=s0meter
Group=s0meter
Type=simple
ExecStart=/opt/s0meter/bin/s0meter
ExecReload=/bin/kill -HUP $MAINPID
Restart=on-failure
RestartSec=5

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

# Build and deploy to Raspberry Pi via SCP (arm64)
make deploy

# Deploy to a 32-bit Pi, overriding the target host
make deploy_arm6 PI_HOST=my-pi PI_USER=pi
```

`PI_USER`, `PI_HOST` and `PI_PATH` can be overridden on the command line; the binary is copied to
`$PI_PATH` and still has to be installed:

```sh
sudo install -o s0meter -g s0meter -m 755 ~/s0meter /opt/s0meter/bin/s0meter
```

---

## Hot-Reload

Send `SIGHUP` to reload the configuration without restarting the process. The GPIO lines are closed
and re-registered, so a changed `debounceTime` takes effect; counters are saved beforehand and
restored afterwards.

```sh
sudo systemctl reload s0meter          # requires ExecReload in the unit (see above)
# or, independent of the unit file:
sudo systemctl kill -s HUP s0meter
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