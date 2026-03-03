# 🚀 s0meter — S0 Pulse Energy Monitor

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

## API

### Get meter data

```sh
curl -k -H "X-Api-Key: your-api-key" https://localhost:8443/data
```

### Get application version

```sh
curl -k https://localhost:8443/version
```

### Health check

```sh
curl -k -H "X-Api-Key: your-api-key" https://localhost:8443/health
```

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

### 2. Copy binary and configuration

```sh
sudo cp s0meter /opt/s0meter/bin/
sudo cp config.yaml /opt/s0meter/etc/
sudo cp cert.pem key.pem /opt/s0meter/etc/
sudo chown -R s0meter:s0meter /opt/s0meter
```

### 3. Create systemd service

```sh
SERVICE_PATH="/etc/systemd/system/s0meter.service"
sudo tee "$SERVICE_PATH" > /dev/null <<'EOF'
[Unit]
Description=s0meter - S0 Pulse Energy Monitor
After=network.target

[Service]
User=s0meter
Group=s0meter
Type=simple
ExecStart=/opt/s0meter/bin/s0meter --config /opt/s0meter/etc/config.yaml
Restart=on-failure

[Install]
WantedBy=multi-user.target
EOF

sudo systemctl daemon-reload
sudo systemctl enable s0meter.service
sudo systemctl start s0meter
sudo systemctl status s0meter
```

### 4. View logs

```sh
journalctl -u s0meter -n 50 -f
```

---

## Firewall (optional)

If ufw is active, allow the configured port:

```sh
# Check which port s0meter is listening on
sudo netstat -tulpn | grep s0meter

# Allow the port (replace 8443 with your configured port)
sudo ufw allow 8443/tcp
sudo ufw status
```

---

## Backup & Restore

### Backup

```sh
sudo tar czvf /tmp/s0meter-backup.tar.gz /opt/s0meter
```

### Restore

```sh
sudo tar xzvf /tmp/s0meter-backup.tar.gz -C /
sudo chown -R s0meter:s0meter /opt/s0meter
sudo systemctl restart s0meter
```

---

## Command Line Flags

| Flag        | Default                          | Description                                       |
|-------------|----------------------------------|---------------------------------------------------|
| `--config`  | `/opt/s0meter/etc/config.yaml`   | Path to the config file                           |
| `--debug`   | `false`                          | Enable debug logging to stdout (overrides config) |
| `--version` | `false`                          | Print the app version and exit                    |
| `--about`   | `false`                          | Print app details and exit                        |
| `--help`    | `false`                          | Print a help message and exit                     |

The config file path can also be set via the environment variable `CONFIG_FILE`.

---

## Configuration

The configuration file is located at `/opt/s0meter/etc/config.yaml`. See the included `config.yaml` for all available
options and documentation.

### Meter Configuration Reference

| Field          | Type     | Description                                                                        |
|----------------|----------|------------------------------------------------------------------------------------|
| `gpio`         | int      | GPIO pin number for S0 pulse input                                                 |
| `bounceTime`   | int (ms) | Debounce time in milliseconds to suppress signal noise                             |
| `unitCounter`  | string   | Unit of the total counter (e.g. `kWh`, `m³`, `l`)                                 |
| `ticksPerUnit` | float    | Pulses per unit — see meter datasheet (Zählerkonstante)                            |
| `unitGauge`    | string   | Unit of the flow rate (e.g. `kW`, `l/h`, `l/s`)                                   |
| `scaleFactor`  | float    | Scale factor applied to the gauge value (e.g. `0.2777778` to convert m³/h → l/s)  |
| `precision`    | int      | Number of decimal places for the gauge value                                       |
| `mqttTopic`    | string   | MQTT topic to publish to (empty = not published)                                   |

---

## License

MIT