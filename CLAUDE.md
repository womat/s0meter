# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

`s0meter` is a Go daemon that counts S0 pulses (DIN 43864) from Raspberry Pi GPIO pins, derives counter/gauge values per meter, persists counters to YAML, publishes to MQTT, and serves a TLS REST API. Single binary, cross-compiled for the Pi.

## Build / develop

**The GPIO dependency (`warthog618/go-gpiocdev` via `womat/golib/gpio/rpi`) only compiles for Linux.** On macOS `go build ./...`, `go vet ./...`, and `make build_mac_arm64` all fail with `undefined: uapi.*`. Always set the target explicitly when checking code locally:

```sh
GOOS=linux GOARCH=arm64 go vet ./...
GOOS=linux GOARCH=arm64 go build ./...
```

```sh
make build_arm64       # Pi 3/4/5/Zero2, 64-bit OS
make build_arm7        # Pi 2/3/4/Zero2, 32-bit OS
make build_arm6        # Pi 1 / Zero, 32-bit OS
make build_arm64_dev   # + Swagger UI (-tags swagger)
make deploy            # build_arm64 then scp to $(PI_USER)@$(PI_HOST) — see Makefile vars
make clean
```

`ensure_dev_certs` (a prerequisite of every build target) generates `app/certs/dev_{cert,key}.pem` if missing; these are `//go:embed`-ed and gitignored, so a fresh clone must build via `make`, not bare `go build`.

There are **no tests** in this repo. `VERSION` is a hand-maintained const in `app/app.go`; `buildDate`/`buildCommit` come from `-ldflags` in the Makefile.

### Swagger

Swagger UI is behind the `swagger` build tag (`app/swagger.go` vs `app/swagger_stub.go`, both defining `registerSwaggerRoute`). Regenerate `docs/` from the annotations after changing API handlers:

```sh
docs/generate.sh   # must run from the project root; needs swaggo/swag installed
```

### Running locally

```sh
go run ./cmd/main.go --config config/config.yaml --debug   # Linux/Pi only
```

## Architecture

Layering is strict: `cmd` → `app` → `app/service/*` → `pkg/*`. Lower layers never import upward.

- **`cmd/main.go`** — flags, config load/validate, logger init, and the **restart loop**. `run()` loops forever: build `app.New(...).Run()`, then block on `a.Restart()` (reload config, construct a brand-new `App`) or `a.Shutdown()` (exit). `README.md` is `//go:embed`-ed as `--help` output, so keep it accurate.
- **`app/app.go`** — wiring and lifecycle. Owns the `context.Context` that every goroutine (MQTT publish loop, backup loop, web server, signal handler) is cancelled by. SIGHUP → `shutdownProcedure(ModeRestart)`; SIGTERM/SIGINT → `ModeStop`. Cleanup saves meter counters before closing GPIO.
- **`app/config.go`** — YAML config with `os.ExpandEnv` applied to the raw file (so `${VAR}` works anywhere), defaults from `NewConfig()`, and a `Validate()` that `cmd` calls before `app.New`.
- **`app/routes.go` / `api_*.go`** — `http.ServeMux` with Go 1.22 method patterns. Middleware chain, outermost first: `WithLogging` → `WithIPFilter` → `WithCORS` → mux. Auth is per-route via `web.WithAuth` (`X-API-Key`, from `womat/golib/web`). `/version` and `/ready` are public; `/health`, `/meters`, `/meters/{name}` are protected.
- **`app/webservices.go`** — HTTPS only. Falls back to the embedded dev cert when `certFile` does not exist.
- **`app/service/s0meters`** — the domain layer. `Handler` holds `map[name]*MeterInstance` under an `RWMutex`; `meters.go` has the pure calculation helpers (`calcCounter`, `calcGauge`), `backup.go` the YAML persistence + ticker, `mqtt.go` the publish loop. Note the locking convention: exported `SerializeMetric` takes the lock, `serializeMetricLocked` assumes the caller holds it — `collectPending` relies on this.
- **`pkg/pulsecounter`** — hardware-facing only. Watches a rising edge on one GPIO pin and keeps `{Pulses, TimeStamp, LastTimeStamp}`. It knows nothing of units or scaling; all unit conversion lives in `s0meters`.

**External dependency `github.com/womat/golib`** supplies `gpio`/`gpio/rpi`, `mqtt`, `web` (auth, CORS, IP filter, `Encode`), and `xlog`. It is not vendored — read it in `$(go env GOMODCACHE)/github.com/womat/golib@<version>` when behavior is unclear.

### Meter value model

`counter = pulses / counterPulsesPerUnit`, rounded to `counterPrecision`. `gauge = 3600 / dt_seconds * gaugeScale`, where `dt` is the interval between the last two pulses — stretched to "time since last pulse" when that is longer, so the rate decays toward 0 when pulses stop.

### MQTT publish model

`StartPeriodicPublish` wakes every `minPublishInterval` and publishes a meter when its **counter**
advanced (i.e. a pulse was counted) or when the last message is older than `publishInterval` (the
heartbeat). The trigger deliberately watches the counter, not the gauge: `calcGauge` stretches its
interval to the time since the last pulse, so the gauge decays continuously between pulses and would
otherwise fire on every tick.

Two invariants worth preserving: publishing happens **outside** the `RWMutex` (`golib/mqtt.Publish`
blocks for up to 5 s per message when disconnected), and a failed publish is not recorded in
`publishState`, so the meter is retried on the next tick. The loop also skips entirely while
`App.mqttConnected` is false.

## Conventions

- Logging is `log/slog` with key/value pairs throughout; `slog.SetDefault` is set once in `cmd`. Do not use `fmt.Print` outside pre-logger startup and `--about`/`--version`/`--help`.
- Doc comments: every package and exported symbol is documented, German-free, and Swagger annotations live directly on the handlers.
- Config field docs are duplicated in `README.md`, `cmd/README.md`, and `config/config.yaml` — update all three when adding a config key.
