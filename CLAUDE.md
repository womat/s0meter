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

Tests use golib's in-memory GPIO emulator (`gpio/rpiemu`) through `pulsecounter.NewWithPin`, so they need no hardware — but they compile on Linux only, like everything here. Run them with `make test` (`go test -race ./...`), on macOS via `docker run --rm -v "$PWD":/src -w /src golang:1.27 make test`. Pure logic is kept testable on purpose: `gaugeAt` takes the time as a parameter, `reading` supplies `time.Now()`. `VERSION` (`app/app.go`), `buildDate` and `buildCommit` (`cmd/main.go`) are all `var`s injected via `-ldflags` — never edit them in source. The Makefile derives `VERSION` from `git describe --tags`; GoReleaser uses the tag itself.

### Releases

**There is one branch, `main`: work is committed to it and a release is a tag on it.** `make release TAG=vX.Y.Z` refuses to run from any other branch, with a dirty tree, or when `main` and `origin/main` differ; `.github/workflows/release.yml` re-checks that the tagged commit is on `main`, so a hand-made `git tag` cannot bypass it. Use a short-lived feature branch for work that must not land on `main` yet. Because releases are tags on the branch everyone builds from, `git describe` — and so a local build's version — always sees the latest release. GitHub Pages serves `main` at <https://womat.github.io/s0meter/>, so it shows the latest commit, which may be ahead of the latest release.

Versioning is SemVer and the Git tag is the single source of truth. `make release TAG=vX.Y.Z` verifies the tag shape and a clean tree, then tags and pushes; `.github/workflows/release.yml` runs `goreleaser release --clean`, which builds linux arm64/armv7/armv6 and publishes a GitHub release with checksums and a grouped changelog.

Two things to keep in mind when touching `.goreleaser.yaml`: its `before` hook must keep running `make ensure_dev_certs` (GoReleaser calls `go build` directly, so the `//go:embed`-ed dev certs would otherwise be missing), and archives must keep shipping `README.md` — it carries the third-party license overview, and the statically linked Paho MQTT client is EPL-2.0. Validate changes with `goreleaser check` and `goreleaser release --snapshot --clean`.

`.github/workflows/ci.yml` runs on every push/PR against `main`: a `test` job (native, `make test`) and a `build` matrix over armv6/armv7/arm64 that vets, builds (also `-tags swagger`) and runs govulncheck. `release.yml` repeats test, vet and govulncheck on the tagged commit before GoReleaser publishes. All actions are pinned to a commit SHA with the release in a comment, and `govulncheck` to a version; `.github/dependabot.yml` updates actions and Go modules weekly, but not the `go install` pins — raise those by hand.

`PI_USER`/`PI_HOST`/`PI_PATH` default to placeholders (the real host name is deliberately not in this public repo — see `f83d16e`). The actual device comes from environment variables (set once for all projects; they win over the `?=` defaults) or, project-specific, from `Makefile.local` (gitignored, pulled in via `-include`); command-line values override both. `PI_PATH` defaults to `.`, the login directory, so it is correct for any user name.

Two ways onto a Pi, deliberately kept apart: `make deploy` builds locally and is the development loop (its binary reports a `-dirty` version, which is how you tell it apart on the device); `make deploy_release TAG=vX.Y.Z` downloads the published archive via `gh`, verifies the checksum and copies that.

**`PI_ARCH` defaults to `arm6`** because the deployment target is a Raspberry Pi Zero (1st gen) — ARMv6, 32-bit only. Every `deploy*` target follows it, so none of them may hardcode an architecture; an arm64 binary dies on that device with `Exec format error`. CI therefore runs a matrix over armv6/armv7/arm64 rather than arm64 alone.

### Swagger

Swagger UI is behind the `swagger` build tag (`app/swagger.go` vs `app/swagger_stub.go`, both defining `registerSwaggerRoute`). Regenerate `docs/` from the annotations after changing API handlers:

```sh
docs/generate.sh   # must run from the project root; needs swaggo/swag installed
```

### Screenshots

The README screenshots and `docs/social-preview.png` are rendered from the real `app/ui/index.html` with a mocked `/health` in headless Chromium. Re-run after visible UI changes (the social preview then has to be uploaded again under the repo's Settings → Social preview):

```sh
docker run --rm -v "$PWD":/src -w /src mcr.microsoft.com/playwright/python:v1.52.0-noble \
  sh -c 'pip install -q playwright==1.52.0 && python3 docs/screenshots/capture.py'
```

### Running locally

```sh
go run ./cmd/main.go --config config/config.yaml --debug   # Linux/Pi only
```

## Architecture

Layering is strict: `cmd` → `app` → `app/service/*` → `pkg/*`. Lower layers never import upward.

- **`cmd/main.go`** — flags, config load/validate, logger init, and the **restart loop**. `run()` subscribes to SIGHUP/SIGTERM/SIGINT **once** and hands that channel to every `App`, so a signal arriving between two lifecycles waits for the next `App` instead of killing the process unsaved — never `signal.Stop`/`Reset` it inside `app`. It then loops forever: build `app.New(...).Run()` — passing a `checkReload` closure that loads and validates the config file, which the SIGHUP handler calls **before** tearing anything down, so a broken file is refused and the running `App` keeps counting — then block on `a.Restart()` (reload config, construct a brand-new `App`) or `a.Shutdown()` (exit). A restart that passes the check but fails in `Run` (TLS, port, GPIO, data file) falls back to `lastGood`, the config of the previous `App`; only a failing first start exits. `Run` undoes a partial start via `abort` (cancel, `wg.Wait`, `Cleanup`), and `Cleanup` saves the counters only when `Init` registered every meter (`metersLoaded`), so a failed start never truncates the data file. `README.md` is `//go:embed`-ed as `--help` output, so keep it accurate.
- **`app/app.go`** — wiring and lifecycle. Owns the `context.Context` that every goroutine (MQTT publish loop, backup loop, web server, signal handler) is cancelled by. The signal goroutine is the **only** caller of `shutdownProcedure`: SIGHUP → `ModeRestart`, SIGTERM/SIGINT → `ModeStop`, and a web server that stops on its own reports on `app.serverErr` → `ModeRestart` (it must not call `shutdownProcedure` itself, because that waits on `app.wg`, which tracks the server goroutine). The web server, backup and publish loops run under `app.wg`, which `shutdownProcedure` waits for before `Cleanup`; `Cleanup` closes the GPIO pins **before** the final save, and `Init` reads the data file **before** registering meters, so no pulse falls between restore/save and counting.
- **`app/config.go`** — YAML config decoded with `KnownFields(true)` (unknown keys are an error), `${VAR}` — and only that form, never bare `$` — expanded on the raw file, yaml.v3 itself refusing a duration without a unit, `0` included (pinned in `config_durations_test.go`), defaults from `NewConfig()`, a `Validate()` that `cmd` calls before `app.New` (GPIO 2–27, unique per meter, precision 0–15), and `Warnings()` for non-fatal findings such as a weak `apiKey`. Authentication is API key only; there is no JWT config.
- **`app/routes.go` / `api_*.go`** — `http.ServeMux` with Go 1.22 method patterns. Middleware chain, outermost first: `WithLogging` → `WithIPFilter` → `WithCORS` → mux. Errors are always JSON `{"error": …}` (`web.ApiError`): `web.WriteError` for 4xx (5xx would hide the message behind "internal server error"), `web.Encode(w, status, web.NewApiError(err))` where a 5xx must keep its reason, like `/ready`. Auth is per-route via `web.WithAuth` (`X-API-Key`, from `womat/golib/web`). `/` (the diagnostic page), `/version` and `/ready` are public (`/ready` is 503 while a configured MQTT broker is not connected); `/health` (including `mqtt` state and per-meter `meters` diagnostics), `/meters`, `/meters/{name}` are protected. CORS allows GET and OPTIONS only.
- **`app/api_ui.go` / `app/ui/index.html`** — the diagnostic page at `GET /{$}`, one `//go:embed`-ed HTML file with inline JS/CSS and a strict CSP; no external resources, since the Pi may be offline. It holds no data: it asks for the API key, keeps it in `localStorage` and polls `/health` only. Everything it shows per meter comes from `s0meters.MeterStatus` there, including `display` — counter and gauge converted to the optional `displayUnit`/`displayGaugeUnit` via the table in `s0meters/units.go` — deliberately not from the telegram, which keeps the configured units.
- **`app/webservices.go`** — HTTPS only. Falls back to the embedded dev cert when `certFile` does not exist, but only with `env: dev`; with `env: prod` that is a start-up error, because the embedded key ships in every release.
- **`app/service/s0meters`** — the domain layer. `Handler` holds `map[name]*MeterInstance` under an `RWMutex`; `meters.go` has the pure calculation helpers (`counterOf`, `gaugeAt`, both on a counter snapshot) and `reading`, which builds the telegram, `backup.go` the YAML persistence (atomic temp-file + rename; an empty or unparsable file is a start-up error, never "start at zero") and backup loop, `mqtt.go` the publish loop. Note the locking convention: `serializeMetricLocked` assumes the caller holds `h.mux` — `collectPending` calls it under `RLock`.
- **`pkg/pulsecounter`** — hardware-facing only. Watches a rising edge on one GPIO pin and keeps `{Pulses, TimeStamp, LastTimeStamp}`. It knows nothing of units or scaling; all unit conversion lives in `s0meters`. Lost edges (golib's `gpio.Event.Missed`, Go-side drops and kernel overflows) are real pulses — the line is kernel-debounced and rising-edge only — so they are added to `Pulses`, and `LastTimeStamp` is cleared so the gauge never uses an interval that spans a gap.

**External dependency `github.com/womat/golib`** supplies `gpio`/`gpio/rpi` (and `gpio/rpiemu` for the tests), `mqtt`, and `web` (auth, CORS, IP filter, `Encode`). Logging is plain `log/slog`, set up by `newLogger` in `cmd/main.go`. It is not vendored — read it in `$(go env GOMODCACHE)/github.com/womat/golib@<version>` when behavior is unclear.

### Meter value model

`counter = pulses / counterPulsesPerUnit`, rounded to `counterPrecision`. `gauge = 3600 / dt_seconds * gaugeScale`, where `dt` is the interval between the last two pulses — stretched to "time since last pulse" when that is longer, so the rate decays toward 0 when pulses stop. A non-positive interval yields 0.

The gauge is "pulses per hour × `gaugeScale`" and does **not** use `counterPulsesPerUnit`, so `gaugeScale` is the amount per pulse in the target unit per hour (1 pulse = 1 l → `l/h` 1, `l/s` 1/3600). Only `pulses` is restored from the data file; the timestamps are not, because after a reboot without RTC they can lie ahead of the clock and produce a spike, so the gauge restarts with the second pulse of a run.

### Telegram contract

`MeterData` is both the MQTT payload and the API response: `meter`, `timestamp`, `counter`, `counterUnit`, `gauge`, `gaugeUnit`. The keys follow ecoflowd's telegrams (myhome `KONZEPT-ECOFLOW.md`); `timestamp` is local time with offset in whole seconds (`now.Truncate(time.Second)` — a deliberate choice, ecoflowd uses UTC). `reading()` is the only place that builds it, and `TestTelegramContract` pins keys and format. The myhome Node-RED flows read `counter`, `gauge` and `counterUnit` — change the contract only together with them.

### MQTT publish model

`RunPeriodicPublish` wakes every `minPublishInterval` and publishes a meter when its **pulse count**
advanced or when the last message is older than `publishInterval` (the heartbeat); meters without
`mqttTopic` are skipped. The trigger deliberately watches the raw pulses — not the gauge, which
`gaugeAt` lets decay continuously between pulses and would fire on every tick, and not the rounded
counter, which a coarse `counterPrecision` would hold still across several pulses.

Two invariants worth preserving: publishing happens **outside** the `RWMutex` (`golib/mqtt.Publish`
waits up to 5 s for the broker's acknowledgement), and a failed publish is not recorded in
`publishState`, so the meter is retried on the next tick. The loop also skips entirely while
`mqtt.Handler.IsConnectionOpen()` is false — that, not `IsConnected()`, is false during a
reconnect, when the client would otherwise drop a QoS 0 message silently.

## Conventions

- Logging is `log/slog` with key/value pairs throughout; `slog.SetDefault` is set once in `cmd`. Do not use `fmt.Print` outside pre-logger startup and `--about`/`--version`/`--help`.
- Doc comments: every package and exported symbol is documented, German-free, and Swagger annotations live directly on the handlers.
- Config field docs live in `README.md` and `config/config.yaml` — update both when adding a config key. `cmd/README.md` is the short `--help` text and only points to them.
- Commit subjects use the prefixes `feat()`, `fix()`, `docu()`, `chore()`, `refactor()`. The release changelog groups on them (`.goreleaser.yaml`), and anything unprefixed lands under "Other". Note it is `docu()`, not `docs()` — the regex accepts both, the repo's history uses `docu`.
