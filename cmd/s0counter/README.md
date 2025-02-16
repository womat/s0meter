# 🚀 S0Counter - Smart Meter Data Collector

S0Counter is designed for **accurate energy monitoring** from one or more **independent electricity, water, and gas
meters**. It supports all **S0 pulse energy meters** compliant with **DIN 43864**, ensuring reliable data acquisition.

The application runs efficiently on **Raspberry Pi hardware**, with successful testing on **Raspberry Pi Zero**.

If no configuration file is found, **default values** will be applied to ensure seamless operation.

---

## 📌 Usage

```sh
s0counter [-debug] [-trace] [-version] [-about] [-help] [-crypt <text>]
```

### 🛠 Available Flags

| **Flag**        | **Description**                                    |
|-----------------|----------------------------------------------------|
| `-version`      | Prints the application version and exits           |
| `-about`        | Displays details about `s0counter` and exits       |
| `-debug`        | Enables verbose debug logging to stdout            |
| `-trace`        | Enables source code location logging for debugging |
| `-help`         | Prints this help message                           |
| `-crypt <text>` | Encrypts the given string and exits                |

---

## 🔍 Examples

### Print Version:

```sh
s0counter -version
```

### Show About Information:

```sh
s0counter -about
```

### Enable Debug Mode (Verbose Logging):

```sh
s0counter -debug
```

### Enable Trace Logging (Source Code Location in Logs):

```sh
s0counter -trace
```

### Encrypt a String (`mysecret` in this example):

```sh
s0counter -crypt "mysecret"
🔐 **Output:** Encrypted string (useful for securing credentials).
```

### Get data from a smart meter:
```sh
curl -k -H "X-Api-Key: 12345678" https://localhost:4000/api/data
```
---

## 📦 Features

✅ **Smart Meter Data Collection** – Supports multiple smart meters  
✅ **Secure MQTT Integration** – Send data securely to MQTT brokers  
✅ **Data Encryption** – Secure sensitive data using `-crypt`  
✅ **Debugging & Tracing** – Use `-debug` and `-trace` to diagnose issues  
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

`s0counter` is licensed under the **MIT License**.

---

## **🌐 IP Address / IP Network Filter**

S0Counter allows **IP-based access control** via the configuration file.

- **`blockedIPs`**: Defines **blocked** IP addresses/networks.
- **`allowedIPs`**: Defines **allowlisted** IP addresses/networks.  
  If set to an **empty list** or `"ALL"`, all IP addresses/networks are allowed.

🔹 **Priority Rule:** `blockedIPs` **takes precedence** over `allowedIPs`.

### **🔧 Configuration Example (`.env` file)**

```ini
# APP_BLOCKED_IPS is a list of IP addresses or networks that are forbidden to access the application.
# Default: empty (no blocked IPs).
# Multiple IPs or networks can be defined, separated by commas.
# Example:
APP_BLOCKED_IPS=192.168.0.1,192.168.0.0/16,10.0.0.0/8,192.168.254.15

# APP_ALLOWED_IPS is a list of IP addresses or networks that are allowed to access the application.
# Default: empty (all IPs are allowed).
# The value "ALL" allows access from all IP addresses/networks.
# Multiple IPs or networks can be defined, separated by commas.
# Example:
APP_ALLOWED_IPS=127.0.0.1,::1,192.168.0.0/16,10.0.0.0/8
# Note: "::1" is the IPv6 loopback address.


## IP Address / IP Network filter

in the config file, IP Address and IP Network filter can be defined.

* blockedIPs: defines blocked IP Addresses/Networks.
* allowedIPs: defines allowlisted IP Addresses/Networks. An empty list or the value 'ALL' allows access from all IP
  Addresses/Networks.

__Information__: the section blockedIPs has priority over allowedIPs

    // APP_BLOCKED_IPS is a list of IP addresses or networks that are forbidden to access the application.
    // Default is empty which means no IP addresses are blocked.
    // multiple IP addresses or networks can be defined separated by a comma
    // e.g.: APP_BLOCKED_IPS=192.168.0.1,192.168.0.0/16,10.0.0.0/8,192.168.254.15   
    APP_BLOCKED_IPS=

    // AllowedIPs is a list of IP addresses or networks that are allowed to access the application.
    // Default is empty which means all IP addresses are allowed.
    // The value "ALL" allows access from all IP Addresses / IP Networks
    // multiple IP addresses or networks can be defined separated by a comma
    // e.g.: APP_ALLOWED_IPS=127.0.0.1,::1,192.168.0.0/16,10.0.0.0/8
    // Note: ::1 is the IPv6 loopback address. 
    APP_ALLOWED_IPS:ALL
```
---

