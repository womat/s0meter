# s0meter

**s0meter** reads impulses from S0 interfaces compliant with DIN 43864 standards, calculates energy counters and flow
rates, publishes the results via MQTT and shows them on a built-in web page (`https://<host>:<listenPort>/`).

----

## Usage

```text
s0meter [--config FILE] [--debug] [--version] [--about] [--help]
s0meter token add <name> [--config FILE] [--dry-run]
s0meter token remove <name> [--config FILE] [--dry-run]
s0meter token list [--config FILE]
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

`--version` prints the semantic version the binary was built from. It is injected from the Git tag
at build time, so an official release reports a plain `7.0.1`, while a local development build
reports a descriptive fallback such as `7.0.1-5-g0c13781-dirty` or `dev`. `--about` additionally
shows the build date and commit.

**Examples:**

```bash
s0meter --config /opt/s0meter/etc/config.yaml
s0meter --debug
s0meter --version
CONFIG_FILE=/opt/s0meter/etc/config.yaml s0meter
```

`SIGHUP` reloads the configuration and the token file; a file that does not load or validate is
refused and the running configuration stays. A config that does not load or validate at start, or a
damaged meter data file, ends s0meter with exit code 78.

---

## Tokens

`/health` and `/meters` need `Authorization: Bearer <token>`. s0meter keeps only the SHA-256 hash of
each token, in the file named by `webserver.tokenFile`, one `<name> <hash>` per line.

- `s0meter token add <name>` writes the hash and prints the token once, on stdout alone.
- `s0meter token remove <name>` deletes it, `s0meter token list` prints the names.
- `--dry-run` prints what would be done and writes nothing.
- `systemctl reload s0meter` applies a change.

Run them with `sudo`: the token file belongs to root.

---

## Configuration

The configuration file is a YAML file. By default it is loaded from `/opt/s0meter/etc/config.yaml`.

Environment variables are expanded inside the file in the `${VAR}` form only, e.g.
`connection: tcp://s0meter:${MQTT_PASSWORD}@broker:1883`. Unknown keys are rejected.

Every key is documented in the example configuration (`config/config.yaml`) and in the project
README: <https://github.com/womat/s0meter#configuration>.
