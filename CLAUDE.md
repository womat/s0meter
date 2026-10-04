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
make build_arm6        # Pi 1 / Zero, 32-bit OS — the deployment target
make build_arm7        # Pi 2/3/4/Zero2, 32-bit OS
make build_arm64       # Pi 3/4/5/Zero2, 64-bit OS
make build_arm6_dev    # + Swagger UI (-tags swagger); _dev variants exist per arch
make deploy            # build for $(PI_ARCH) then scp to $(PI_USER)@$(PI_HOST)
make clean
```

`ensure_dev_certs` (a prerequisite of every build target) generates `app/certs/dev_{cert,key}.pem` if missing; these are `//go:embed`-ed and gitignored, so a fresh clone must build via `make`, not bare `go build`.

There are **no tests** in this repo. `VERSION` (`app/app.go`), `buildDate` and `buildCommit` (`cmd/main.go`) are all `var`s injected via `-ldflags` — never edit them in source. The Makefile derives `VERSION` from `git describe --tags`; GoReleaser uses the tag itself.

### Releases

**Releases are always cut from `main`, and `develop` must be merged into it first.** `develop` is the default branch and where work happens; `main` carries only `--no-ff` merges of it, every historical release tag is reachable from it, and GitHub Pages serves it at <https://womat.github.io/s0meter/>. So the order is `make merge_to_main`, then `make release TAG=vX.Y.Z` — the release target refuses to run from any other branch, when `main` and `origin/main` differ, or when `origin/develop` is not an ancestor, and `.github/workflows/release.yml` re-checks that the tagged commit is on `main` so a hand-made `git tag` cannot bypass it.

Versioning is SemVer and the Git tag is the single source of truth. `make release TAG=vX.Y.Z` verifies the tag shape and a clean tree, then tags and pushes; `.github/workflows/release.yml` runs `goreleaser release --clean`, which builds linux arm64/armv7/armv6 and publishes a GitHub release with checksums and a grouped changelog.

Two things to keep in mind when touching `.goreleaser.yaml`: its `before` hook must keep running `make ensure_dev_certs` (GoReleaser calls `go build` directly, so the `//go:embed`-ed dev certs would otherwise be missing), and archives must keep shipping `README.md` — it carries the third-party license overview, and the statically linked Paho MQTT client is EPL-2.0. Validate changes with `goreleaser check` and `goreleaser release --snapshot --clean`.

`.github/workflows/ci.yml` vets and builds on every push/PR against `develop`, with `GOOS=linux GOARCH=arm64` set at the job level.

`PI_USER`/`PI_HOST`/`PI_PATH` default to placeholders (the real host name is deliberately not in this public repo — see `f83d16e`). `Makefile.local`, gitignored and pulled in via `-include`, carries the actual device; command-line values still override it. `PI_PATH` defaults to `.`, the login directory, so it is correct for any user name.

Two ways onto a Pi, deliberately kept apart: `make deploy` builds locally and is the development loop (its binary reports a `-dirty` version, which is how you tell it apart on the device); `make deploy_release TAG=vX.Y.Z` downloads the published archive via `gh`, verifies the checksum and copies that.

**`PI_ARCH` defaults to `arm6`** because the deployment target is a Raspberry Pi Zero (1st gen) — ARMv6, 32-bit only. Every `deploy*` target follows it, so none of them may hardcode an architecture; an arm64 binary dies on that device with `Exec format error`. CI therefore runs a matrix over armv6/armv7/arm64 rather than arm64 alone.

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

- **`cmd/main.go`** — flags, config load/validate, logger init, and the **restart loop**. `run()` subscribes to SIGHUP/SIGTERM/SIGINT **once** and hands that channel to every `App`, so a signal arriving between two lifecycles waits for the next `App` instead of killing the process unsaved — never `signal.Stop`/`Reset` it inside `app`. It then loops forever: build `app.New(...).Run()` — passing a `checkReload` closure that loads and validates the config file, which the SIGHUP handler calls **before** tearing anything down, so a broken file is refused and the running `App` keeps counting — then block on `a.Restart()` (reload config, construct a brand-new `App`) or `a.Shutdown()` (exit). `README.md` is `//go:embed`-ed as `--help` output, so keep it accurate.
- **`app/app.go`** — wiring and lifecycle. Owns the `context.Context` that every goroutine (MQTT publish loop, backup loop, web server, signal handler) is cancelled by. SIGHUP → `shutdownProcedure(ModeRestart)`; SIGTERM/SIGINT → `ModeStop`. The web server, backup and publish loops run under `app.wg`, which `shutdownProcedure` waits for before `Cleanup`; `Cleanup` closes the GPIO pins **before** the final save, and `Init` reads the data file **before** registering meters, so no pulse falls between restore/save and counting.
- **`app/config.go`** — YAML config decoded with `KnownFields(true)` (unknown keys are an error), `${VAR}` — and only that form, never bare `$` — expanded on the raw file, defaults from `NewConfig()`, a `Validate()` that `cmd` calls before `app.New` (GPIO 2–27, unique per meter, precision 0–15), and `Warnings()` for non-fatal findings such as a weak `apiKey`. Authentication is API key only; there is no JWT config.
- **`app/routes.go` / `api_*.go`** — `http.ServeMux` with Go 1.22 method patterns. Middleware chain, outermost first: `WithLogging` → `WithIPFilter` → `WithCORS` → mux. Auth is per-route via `web.WithAuth` (`X-API-Key`, from `womat/golib/web`). `/version` and `/ready` are public; `/health`, `/meters`, `/meters/{name}` are protected.
- **`app/webservices.go`** — HTTPS only. Falls back to the embedded dev cert when `certFile` does not exist, but only with `env: dev`; with `env: prod` that is a start-up error, because the embedded key ships in every release.
- **`app/service/s0meters`** — the domain layer. `Handler` holds `map[name]*MeterInstance` under an `RWMutex`; `meters.go` has the pure calculation helpers (`calcCounter`, `calcGauge`), `backup.go` the YAML persistence (atomic temp-file + rename; an empty or unparsable file is a start-up error, never "start at zero") and backup loop, `mqtt.go` the publish loop. Note the locking convention: exported `SerializeMetric` takes the lock, `serializeMetricLocked` assumes the caller holds it — `collectPending` relies on this.
- **`pkg/pulsecounter`** — hardware-facing only. Watches a rising edge on one GPIO pin and keeps `{Pulses, TimeStamp, LastTimeStamp}`. It knows nothing of units or scaling; all unit conversion lives in `s0meters`.

**External dependency `github.com/womat/golib`** supplies `gpio`/`gpio/rpi`, `mqtt`, `web` (auth, CORS, IP filter, `Encode`), and `xlog`. It is not vendored — read it in `$(go env GOMODCACHE)/github.com/womat/golib@<version>` when behavior is unclear.

### Meter value model

`counter = pulses / counterPulsesPerUnit`, rounded to `counterPrecision`. `gauge = 3600 / dt_seconds * gaugeScale`, where `dt` is the interval between the last two pulses — stretched to "time since last pulse" when that is longer, so the rate decays toward 0 when pulses stop. A non-positive interval yields 0.

The gauge is "pulses per hour × `gaugeScale`" and does **not** use `counterPulsesPerUnit`, so `gaugeScale` is the amount per pulse in the target unit per hour (1 pulse = 1 l → `l/h` 1, `l/s` 1/3600). Only `pulses` is restored from the data file; the timestamps are not, because after a reboot without RTC they can lie ahead of the clock and produce a spike, so the gauge restarts with the second pulse of a run.

### MQTT publish model

`RunPeriodicPublish` wakes every `minPublishInterval` and publishes a meter when its **pulse count**
advanced or when the last message is older than `publishInterval` (the heartbeat); meters without
`mqttTopic` are skipped. The trigger deliberately watches the raw pulses — not the gauge, which
`calcGauge` lets decay continuously between pulses and would fire on every tick, and not the rounded
counter, which a coarse `counterPrecision` would hold still across several pulses.

Two invariants worth preserving: publishing happens **outside** the `RWMutex` (`golib/mqtt.Publish`
waits up to 5 s for the broker's acknowledgement), and a failed publish is not recorded in
`publishState`, so the meter is retried on the next tick. The loop also skips entirely while
`mqtt.Handler.IsConnectionOpen()` is false — that, not `IsConnected()`, is false during a
reconnect, when the client would otherwise drop a QoS 0 message silently.

## Conventions

- Logging is `log/slog` with key/value pairs throughout; `slog.SetDefault` is set once in `cmd`. Do not use `fmt.Print` outside pre-logger startup and `--about`/`--version`/`--help`.
- Doc comments: every package and exported symbol is documented, German-free, and Swagger annotations live directly on the handlers.
- Config field docs are duplicated in `README.md`, `cmd/README.md`, and `config/config.yaml` — update all three when adding a config key.
- Commit subjects use the prefixes `feat()`, `fix()`, `docu()`, `chore()`, `refactor()`. The release changelog groups on them (`.goreleaser.yaml`), and anything unprefixed lands under "Other". Note it is `docu()`, not `docs()` — the regex accepts both, the repo's history uses `docu`.
