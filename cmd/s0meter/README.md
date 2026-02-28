# 🚀 S0meter - Smart Meter Data Collector

S0meter is designed for **accurate energy monitoring** from one or more **independent electricity, water, and gas
meters**. It supports all **S0 pulse energy meters** compliant with **DIN 43864**, ensuring reliable data acquisition.

The application runs efficiently on **Raspberry Pi hardware**, with successful testing on **Raspberry Pi Zero**.

If no configuration file is found, **default values** will be applied to ensure seamless operation.

---

## 📌 Usage

```sh
s0meter [-logLevel debug|info|warning|error] [-LogDestination stdout|stderr|null|/path/to/logfile] [-version] [-about] [-help]

```

### 🛠 Available Flags

| **Flag**                   | **Description**                                                |
|----------------------------|----------------------------------------------------------------|
| `-version`                 | Prints the application version and exit                        |
| `-about`                   | Prints details about `s0meter` and exit                        |
| `-help`                    | Prints this help message and exit                              |
| `-logLevel <level>`        | Set the log level: debug, info, warning ,error                 |
| `-logDestination <dest>`   | Set the log destination: stdout, stderr,null, /path/to/logfile |
| `-config </path/file.cfg>` | Specify the path to the config file                            |

---

## 🔍 Examples

### Print Version:

```sh
s0meter -version
```

### Show About Information:

```sh
s0meter -about
```

### Enable Debug Logging (Source Code Location in Logs):

```sh
s0meter -logLevel debug -logDestination stdout
```

### Get monitoring data from a smart meter:

```sh
curl -k -H "X-Api-Key: 12345678" https://localhost:443/api/monitoring
```

### Get data from a smart meter:

```sh
curl -k -H "X-Api-Key: 12345678" https://localhost:443/api/data
```

---

## 📦 Features

✅ **Smart Meter Data Collection** – Supports multiple smart meters  
✅ **Secure MQTT Integration** – Send data securely to MQTT brokers  
✅ **Lightweight & Fast** – Optimized for embedded and IoT environments  
✅ **Supports Electricity, Water, and Gas Meters**    
✅ **Compatible with DIN 43864 Standard for S0 Pulse Meters**  
✅ **Optimized for Raspberry Pi (Tested on Raspberry Pi Zero)**  
✅ **Reliable and Efficient Data Processing**  
✅ **IP Address Filtering for Enhanced Security**

---

## 📖 Documentation

For detailed setup and configuration, visit our **[official documentation]**.

---

## 👨‍💻 Contributing

Want to contribute? Feel free to submit **pull requests** or report issues in the repository.

---

## 📜 License

`s0meter` is licensed under the **MIT License**.

---

## **🌐 IP Address / IP Network Filter**

s0meter allows **IP-based access control** via the configuration file.

- **`blockedIPs`**: Defines **blocked** IP addresses/networks.
- **`allowedIPs`**: Defines **allowlisted** IP addresses/networks.  
  If set to an **empty list** or `"ALL"`, all IP addresses/networks are allowed.

🔹 **Priority Rule:** `blockedIPs` **takes precedence** over `allowedIPs`.

## generate a self-signed certificate for development**

    openssl req -x509 -nodes -newkey rsa:2048 -keyout selfsigned.key -out selfsigned.crt -days 35600 -subj "/C=AT/ST=Vienna/L=Vienna/O=ITDesign/OU=DEV/CN=localhost/emailAddress=support@itdesign.at"
      -subj description
       /C=AT								Country
       /ST=Vienna							State (optional).
       /L=Vienna							Location – City (optional).
       /O=company							company (optional).
       /OU=IT								Organizational Unit – (optional).
       /CN=my-domain.com					Common Name – IMPORTANT! your domain name or localhost.
       /emailAddress=admin@my-domain.com	E-Mail-Address (optional).

