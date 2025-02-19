# 🚀 S0Counter - Smart Meter Data Collector

S0Counter is designed for **accurate energy monitoring** from one or more **independent electricity, water, and gas
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

