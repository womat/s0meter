# 🚀 S0meter - Smart Meter Data Collector

S0meter is designed for **accurate energy monitoring** from one or more **independent electricity, water, and gas
meters**. It supports all **S0 pulse energy meters** compliant with **DIN 43864**, ensuring reliable data acquisition.

The application runs efficiently on **Raspberry Pi hardware**, with successful testing on **Raspberry Pi Zero**.

## Get data from a smart meter:
```sh
curl -k -H "X-Api-Key: 12345678" https://localhost:443/api/data
```

## Create a new certificate:
```sh
openssl req -x509 -newkey rsa:4096 -keyout key.pem -out cert.pem -days 365 -nodes
```

## Backup configuration
```sh
sudo tar czvf /tmp/opt-s0meter.tar.gz /opt/s0meter
```

## Restore configuration
```sh
sudo tar xzvf /tmp/opt-s0meter.tar.gz -C /
sudo chown -R s0meter:s0meter /opt/s0meter
```

## Installation
```sh
sudo groupadd -f s0meter
sudo useradd -r -s /usr/sbin/nologin -g s0meter s0meter
sudo usermod -aG gpio s0meter

sudo mkdir -p /opt/s0meter
sudo chown -R s0meter:s0meter /opt/s0meter

SERVICE_PATH="/etc/systemd/system/s0meter.service"
sudo tee "$SERVICE_PATH" > /dev/null <<'EOF'
[Unit]
Description=Read data from wallbox
After=network.target

[Service]
User=s0meter
Group=s0meter
Type=simple
ExecStart=/opt/womat/bin/s0meter

[Install]
WantedBy=multi-user.target
EOF

sudo systemctl daemon-reload
sudo systemctl enable s0meter.service
sudo systemctl start s0meter
sudo systemctl status s0meter

journalctl -u s0meter -n 50

# ggfls bei ufw das port freischalten
# 1. Prüfen, ob der Dienst läuft und auf welchem Port er hört
sudo netstat -tulpn | grep s0meter

# 2. Angenommen, der Dienst hört auf Port 4000, dann diesen Port in der Firewall freigeben
sudo ufw allow 4000/tcp

# 3. Überprüfen, ob die Regel hinzugefügt wurde
sudo ufw status

# zugriff auf die API testen
curl http://wallbox:4000/currentdata|json_pp
```
