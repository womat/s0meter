# https://gist.github.com/thomaspoignant/5b72d579bd5f311904d973652180c705

GOCMD=go
GOTEST=$(GOCMD) test
GOVET=$(GOCMD) vet
BINARY_NAME=s0counter
VERSION?=0.0.0
SERVICE_PORT?=3000
DOCKER_REGISTRY?= #if set it should finished by /
EXPORT_RESULT?=false # for CI please set EXPORT_RESULT to true

TARGET_NODE=breakout


GREEN  := $(shell tput -Txterm setaf 2)
YELLOW := $(shell tput -Txterm setaf 3)
WHITE  := $(shell tput -Txterm setaf 7)
CYAN   := $(shell tput -Txterm setaf 6)
RESET  := $(shell tput -Txterm sgr0)


.PHONY: all test build vendor copy
#build_arm6 build_arm7 build_arm8

all: help

clean: ## Remove build related file
	rm -fr ../bin/arm6
	rm -fr ../bin/arm7
	rm -fr ../bin/arm8
	rm -fr ../bin/arm64
	rm -fr ../bin/amd64
	rm -fr ../bin/darwin
	rm -fr ../bin/386



# ==================================================================================================================
# Raspberry Pi Kompatibilitätstabelle für GOARCH und GOARM
# ==================================================================================================================
# Modell                    CPU    GOARCH=arm GOARM=6   GOARCH=arm GOARM=7   GOARCH=arm GOARM=8   GOARCH=arm64
#
# Raspberry Pi 1 (A/B/+)    ARMv6  Läuft gut           Nicht kompatibel     Nicht kompatibel     Nicht kompatibel
# Raspberry Pi Zero (1.Gen) ARMv6  Läuft gut           Nicht kompatibel     Nicht kompatibel     Nicht kompatibel
# Raspberry Pi 2            ARMv7  Langsam             Läuft gut            Nicht kompatibel     Nicht kompatibel
# Raspberry Pi 3            ARMv8  Langsam             Läuft gut            Läuft gut            Läuft mit 64-Bit OS
# Raspberry Pi 4            ARMv8  Langsam             Läuft gut            Läuft gut            Läuft mit 64-Bit OS
# Raspberry Pi 5            ARMv8  Nicht kompatibel    Langsam              Läuft gut            Läuft mit 64-Bit OS
# Raspberry Pi Zero 2 W     ARMv8  Langsam             Läuft gut            Läuft gut            Läuft mit 64-Bit OS
# ==================================================================================================================


build_arm6: ## build binary for all raspberry models 32bit ausser Pi5"
	GOOS=linux GOARCH=arm GOARM=6 go build -o ../bin/arm6/${BINARY_NAME} ../cmd/${BINARY_NAME}/main.go

build_arm7: ## build binary for raspberry models 2/3/4/5/Zero2 32bit"
	GOOS=linux GOARCH=arm GOARM=7 go build -o ../bin/arm7/${BINARY_NAME} ../cmd/${BINARY_NAME}/main.go

build_arm8: ## build binary for raspberry 3/4/5/Zero2 32bit"
	GOOS=linux GOARCH=arm64 go build -o ../bin/arm8/${BINARY_NAME} ../cmd/${BINARY_NAME}/main.go

build_arm64: ## build binary for raspberry models 3/4/5/Zero2 64bit"
	GOOS=linux GOARCH=arm64 go build -o ../bin/arm64/${BINARY_NAME} ../cmd/${BINARY_NAME}/main.go

build_windows386: ## build binary for windows"
	GOOS=windows GOARCH=386 go build -o ../bin/386/${BINARY_NAME}.exe ../cmd/${BINARY_NAME}/main.go

build_windows64: ## build binary for windows 64bit"
	GOOS=windows GOARCH=amd64 go build -o ../bin/amd64/${BINARY_NAME}.exe ../cmd/${BINARY_NAME}/main.go

build_linux386: ## build binary for linux"
	GOOS=linux GOARCH=386 go build -o ../bin/386/${BINARY_NAME} ../cmd/${BINARY_NAME}/main.go

build_linux64: ## build binary for linux 64bit"
	GOOS=linux GOARCH=amd64 go build -o ../bin/amd64/${BINARY_NAME} ../cmd/${BINARY_NAME}/main.go

build_mac_arm64: ## build binary mac M1"
	GOOS=darwin GOARCH=arm64 go build -o ../bin/darwin/${BINARY_NAME} ../cmd/${BINARY_NAME}/main.go



deploy: build_arm6 ## build binary and copy binary to ${TARGET_NODE}:/tmp
	scp -q ../bin/arm6/${BINARY_NAME} ${TARGET_NODE}:/tmp
	@echo
	@echo 'logon to "${TARGET_NODE}":'
	@echo '		ssh ${TARGET_NODE}'
	@echo
	@echo 'install "${BINARY_NAME}" on ${TARGET_NODE}'
	@echo '		sudo systemctl stop ${BINARY_NAME};sudo cp /tmp/${BINARY_NAME} /opt/${BINARY_NAME}/bin/${BINARY_NAME};sudo /opt/${BINARY_NAME}/bin/${BINARY_NAME} --version;sudo systemctl start ${BINARY_NAME}'
	@echo


## Help:
help: ## Show this help.
	@echo ''
	@echo 'Usage:'
	@echo '  ${YELLOW}make${RESET} ${GREEN}<target>${RESET}'
	@echo ''
	@echo 'Targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[0-9a-zA-Z_-]+:.*?## / {printf "${YELLOW}%-16s${GREEN}%s${RESET}\n", $$1, $$2}' $(MAKEFILE_LIST)
