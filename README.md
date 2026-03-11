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
curl -k -H "X-Api-Key: your-api-key" https://localhost:8443/meters
curl -k -H "X-Api-Key: your-api-key" https://localhost:8443/meters/{name}

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
