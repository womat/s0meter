# https://gist.github.com/thomaspoignant/5b72d579bd5f311904d973652180c705

GOCMD=go
GOTEST=$(GOCMD) test
GOVET=$(GOCMD) vet
BINARY_NAME=s0meter
DEV_CERT_DIR=./app/certs
DEV_CERT_FILE=$(DEV_CERT_DIR)/dev_cert.pem
DEV_KEY_FILE=$(DEV_CERT_DIR)/dev_key.pem
SERVICE_PORT?=3000
DOCKER_REGISTRY?= #if set it should finished by /
EXPORT_RESULT?=false # for CI please set EXPORT_RESULT to true

# Branch policy: releases are always cut from RELEASE_BRANCH, and
# DEVELOP_BRANCH has to be merged into it first. The release target enforces
# both, because a tag on an unmerged commit produces artifacts that no longer
# match what main publishes (GitHub Pages serves main as well).
RELEASE_BRANCH ?= main
DEVELOP_BRANCH ?= develop

# Raspberry Pi Login / IP. These are placeholders - the real host name stays out
# of this public repository. Put your device in Makefile.local instead (it is
# gitignored and included below), so plain `make deploy` works without repeating
# the address:
#
#   PI_USER := myuser
#   PI_HOST := mypi
#
# PI_PATH defaults to the login directory, which is correct for any user name;
# override it only to land the binary somewhere else.
PI_USER ?= pi
PI_HOST ?= raspberrypi
PI_PATH ?= .

# Architecture of the target device, used by every deploy_* target.
# The deployment target is a Raspberry Pi Zero (1st gen), which is ARMv6 and 32-bit
# only - an arm64 binary does not start on it. Override for a different device:
#   make deploy PI_ARCH=arm64 PI_HOST=my-pi
# See the compatibility table below for which value a model needs.
PI_ARCH ?= arm6

# Local, untracked overrides for the settings above. Missing file is fine.
-include Makefile.local

# Guard the value: PI_REL_ARCH below maps anything unrecognised to arm64, so a
# typo would otherwise silently produce or download the wrong architecture.
ifeq ($(filter $(PI_ARCH),arm6 arm7 arm64),)
$(error PI_ARCH must be one of: arm6 arm7 arm64 (got '$(PI_ARCH)'))
endif

# Release archives are named after the Go arch, not the make target.
PI_REL_ARCH := $(if $(filter arm6,$(PI_ARCH)),armv6,$(if $(filter arm7,$(PI_ARCH)),armv7,arm64))

GREEN  := $(shell tput -Txterm setaf 2)
YELLOW := $(shell tput -Txterm setaf 3)
WHITE  := $(shell tput -Txterm setaf 7)
CYAN   := $(shell tput -Txterm setaf 6)
RESET  := $(shell tput -Txterm sgr0)


BUILD_DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
BUILD_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# The Git tag is the single source of truth for the version. A build on a tag
# yields a plain semver string (4.7.0), any other working copy a descriptive
# fallback (4.7.0-5-g0c13781-dirty).
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null | sed 's/^v//')
VERSION := $(if $(VERSION),$(VERSION),dev)

LDFLAGS := -X 'main.buildDate=$(BUILD_DATE)' \
           -X 'main.buildCommit=$(BUILD_COMMIT)' \
           -X 'github.com/womat/s0meter/app.VERSION=$(VERSION)'

.PHONY: all test build vendor copy release merge_to_main deploy_release build_dev build_arm6 build_arm6_dev build_arm7 build_arm7_dev build_arm64 build_arm64_dev build_windows386 build_windows64 build_linux386 build_linux64 build_mac_arm64 deploy deploy_dev clean help ensure_dev_certs

all: help

clean: ## Remove build related file
	rm -fr ./bin/release
	rm -fr ./bin/arm6
	rm -fr ./bin/arm7
	rm -fr ./bin/arm64
	rm -fr ./bin/amd64
	rm -fr ./bin/darwin
	rm -fr ./bin/386

ensure_dev_certs:
	@mkdir -p $(DEV_CERT_DIR)
	@if [ ! -f "$(DEV_CERT_FILE)" ] || [ ! -f "$(DEV_KEY_FILE)" ]; then \
		echo "Generating development TLS certificate in $(DEV_CERT_DIR)"; \
		openssl req -x509 -nodes -newkey rsa:2048 \
			-keyout "$(DEV_KEY_FILE)" \
			-out "$(DEV_CERT_FILE)" \
			-days 365 \
			-subj "/C=AT/ST=Vienna/L=Vienna/O=modbusgateway/OU=Development/CN=localhost"; \
	fi


# ==================================================================================================================
# Raspberry Pi -> Go build target
# Sources: https://go.dev/wiki/GoArm and `go help environment` (GOARM, GOARM64).
# ==================================================================================================================
# GOARM applies to GOARCH=arm only and accepts 5, 6 or 7 - there is no GOARM=8.
# 64-bit ARM is GOARCH=arm64, where GOARM is ignored and GOARM64 (default v8.0)
# applies instead. When cross-compiling, GOARM defaults to 7, so a GOARCH=arm
# target that omits it silently builds for ARMv7. Floating point follows the
# value: ,hardfloat is the default for 6 and 7, ,softfloat the default for 5.
#
# Model                     CPU core       ISA         32-bit OS   64-bit OS
#
# Raspberry Pi 1 (A/B/+)    ARM1176JZF-S   ARMv6       GOARM=6     -
# Raspberry Pi Zero / W     ARM1176JZF-S   ARMv6       GOARM=6     -
# Raspberry Pi 2            Cortex-A7      ARMv7       GOARM=7     -
# Raspberry Pi 3            Cortex-A53     ARMv8-A     GOARM=7     GOARCH=arm64
# Raspberry Pi 4            Cortex-A72     ARMv8-A     GOARM=7     GOARCH=arm64
# Raspberry Pi 5            Cortex-A76     ARMv8.2-A   GOARM=7     GOARCH=arm64
# Raspberry Pi Zero 2 W     Cortex-A53     ARMv8-A     GOARM=7     GOARCH=arm64
#
# These are the minimum values a model needs. ARM is backward compatible, so a
# GOARM=6 binary also runs on ARMv7/ARMv8 in 32-bit mode; the reverse never
# works. This project deploys to a Pi Zero (1st gen), hence PI_ARCH=arm6.
# ==================================================================================================================


build_arm64_dev: ensure_dev_certs ## ARMv8, 64-bit OS on Pi 3/4/5/Zero 2 W - with Swagger UI
	GOOS=linux GOARCH=arm64 \
	go build -tags swagger -ldflags "$(LDFLAGS)" -o ./bin/arm64/${BINARY_NAME} ./cmd/main.go

build_arm6_dev: ensure_dev_certs ## ARMv6, runs on every Pi in 32-bit mode; required for Pi 1 / Zero - with Swagger UI
	GOOS=linux GOARCH=arm GOARM=6 \
	go build -tags swagger -ldflags "$(LDFLAGS)" -o ./bin/arm6/${BINARY_NAME} ./cmd/main.go

build_arm7_dev: ensure_dev_certs ## ARMv7, 32-bit Pi 2 and newer - with Swagger UI
	GOOS=linux GOARCH=arm GOARM=7 \
	go build -tags swagger -ldflags "$(LDFLAGS)" -o ./bin/arm7/${BINARY_NAME} ./cmd/main.go

build_arm6: ensure_dev_certs ## ARMv6, runs on every Pi in 32-bit mode; required for Pi 1 / Zero
	GOOS=linux GOARCH=arm GOARM=6 \
	go build -ldflags "$(LDFLAGS)" -o ./bin/arm6/${BINARY_NAME} ./cmd/main.go

build_arm7: ensure_dev_certs ## ARMv7, 32-bit Pi 2 and newer
	GOOS=linux GOARCH=arm GOARM=7 \
	go build -ldflags "$(LDFLAGS)" -o ./bin/arm7/${BINARY_NAME} ./cmd/main.go

build_arm64: ensure_dev_certs ## ARMv8, 64-bit OS on Pi 3/4/5/Zero 2 W
	GOOS=linux GOARCH=arm64 \
	go build -ldflags "$(LDFLAGS)" -o ./bin/arm64/${BINARY_NAME} ./cmd/main.go

build_windows386: ensure_dev_certs ## build binary for windows
	GOOS=windows GOARCH=386 \
	go build -ldflags "$(LDFLAGS)" -o ./bin/386/${BINARY_NAME}.exe ./cmd/main.go

build_windows64: ensure_dev_certs ## build binary for windows 64bit
	GOOS=windows GOARCH=amd64 \
	go build -ldflags "$(LDFLAGS)" -o ./bin/amd64/${BINARY_NAME}.exe ./cmd/main.go

build_linux386: ensure_dev_certs ## build binary for linux
	GOOS=linux GOARCH=386 \
	go build -ldflags "$(LDFLAGS)" -o ./bin/386/${BINARY_NAME} ./cmd/main.go

build_linux64: ensure_dev_certs ## build binary for linux 64bit
	GOOS=linux GOARCH=amd64 \
	go build -ldflags "$(LDFLAGS)" -o ./bin/amd64/${BINARY_NAME} ./cmd/main.go

build_mac_arm64: ensure_dev_certs ## build binary mac M1
	GOOS=darwin GOARCH=arm64 \
	go build -ldflags "$(LDFLAGS)" -o ./bin/darwin/${BINARY_NAME} ./cmd/main.go


deploy: build_$(PI_ARCH) ## build for $(PI_ARCH) and copy to $(PI_USER)@$(PI_HOST) (override: make deploy PI_ARCH=arm64 PI_HOST=my-pi)
	@echo "Copying $(PI_ARCH) binary to  $(PI_USER)@$(PI_HOST):$(PI_PATH)"
	scp ./bin/$(PI_ARCH)/${BINARY_NAME} $(PI_USER)@$(PI_HOST):$(PI_PATH)

deploy_dev: build_$(PI_ARCH)_dev ## build for $(PI_ARCH) with Swagger UI and copy to $(PI_USER)@$(PI_HOST)
	@echo "Copying $(PI_ARCH) binary (Swagger UI) to  $(PI_USER)@$(PI_HOST):$(PI_PATH)"
	scp ./bin/$(PI_ARCH)/${BINARY_NAME} $(PI_USER)@$(PI_HOST):$(PI_PATH)

# Checksum verification differs between GNU coreutils and the BSD/perl shasum on macOS.
SHA256SUM := $(shell command -v sha256sum 2>/dev/null || echo "shasum -a 256")

deploy_release: ## download a published $(PI_REL_ARCH) release, verify it and copy it to the Pi (make deploy_release TAG=v4.7.0)
	@test -n "$(TAG)" || { echo "usage: make deploy_release TAG=v4.7.0"; exit 1; }
	@command -v gh >/dev/null || { echo "gh is required, see https://cli.github.com"; exit 1; }
	rm -fr ./bin/release
	mkdir -p ./bin/release
	gh release download $(TAG) --dir ./bin/release \
		--pattern '*_linux_$(PI_REL_ARCH).tar.gz' --pattern 'checksums.txt'
	cd ./bin/release && $(SHA256SUM) -c checksums.txt --ignore-missing
	tar xzf ./bin/release/*_linux_$(PI_REL_ARCH).tar.gz -C ./bin/release ${BINARY_NAME}
	@echo "Copying $(TAG) binary to  $(PI_USER)@$(PI_HOST):$(PI_PATH)"
	scp ./bin/release/${BINARY_NAME} $(PI_USER)@$(PI_HOST):$(PI_PATH)


merge_to_main: ## merge $(DEVELOP_BRANCH) into $(RELEASE_BRANCH) and push - the step that has to precede a release
	@git diff --quiet HEAD || { echo "working tree is dirty, commit first"; exit 1; }
	git fetch origin
	git checkout $(RELEASE_BRANCH)
	git merge --ff-only origin/$(RELEASE_BRANCH)
	git merge --no-ff origin/$(DEVELOP_BRANCH) -m "Merge branch '$(DEVELOP_BRANCH)'"
	git push origin $(RELEASE_BRANCH)
	@echo "$(DEVELOP_BRANCH) is now in $(RELEASE_BRANCH); release with: make release TAG=vX.Y.Z"

release: ## tag $(RELEASE_BRANCH) and push it, triggering the GitHub release workflow (make release TAG=v4.7.0)
	@test -n "$(TAG)" || { echo "usage: make release TAG=v4.7.0"; exit 1; }
	@echo "$(TAG)" | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+$$' || { echo "TAG must be semver with a v prefix, e.g. v4.7.0"; exit 1; }
	@git diff --quiet HEAD || { echo "working tree is dirty, commit first"; exit 1; }
	@test "$$(git rev-parse --abbrev-ref HEAD)" = "$(RELEASE_BRANCH)" || \
		{ echo "releases are cut from $(RELEASE_BRANCH), but you are on $$(git rev-parse --abbrev-ref HEAD) - run: make merge_to_main"; exit 1; }
	@git fetch origin --quiet
	@test "$$(git rev-parse HEAD)" = "$$(git rev-parse origin/$(RELEASE_BRANCH))" || \
		{ echo "$(RELEASE_BRANCH) and origin/$(RELEASE_BRANCH) differ - pull or push first"; exit 1; }
	@git merge-base --is-ancestor origin/$(DEVELOP_BRANCH) HEAD || \
		{ echo "origin/$(DEVELOP_BRANCH) is not merged into $(RELEASE_BRANCH) - run: make merge_to_main"; exit 1; }
	git tag -a $(TAG) -m "release $(TAG)"
	git push origin $(TAG)
	@echo "Tag pushed. Watch the release build with: gh run watch"


## Help:

# awk reads the help comments straight from the file, so make never expands the
# variables in them. Substitute the ones used in help text afterwards, otherwise
# the listing shows a literal $(PI_ARCH) instead of its current value.
HELP_VARS := PI_ARCH PI_REL_ARCH PI_USER PI_HOST PI_PATH BINARY_NAME
HELP_SED := $(foreach v,$(HELP_VARS),-e 's|[$$][({]$(v)[)}]|$($(v))|g')

help: ## Show this help.
	@echo ''
	@echo 'Usage:'
	@echo '  ${YELLOW}make${RESET} ${GREEN}<target>${RESET}'
	@echo ''
	@echo 'Targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[0-9a-zA-Z_-]+:.*?## / {printf "${YELLOW}%-16s${GREEN}%s${RESET}\n", $$1, $$2}' $(MAKEFILE_LIST) \
		| sed $(HELP_SED)