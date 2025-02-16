#!/bin/sh
#
#  Generate Swagger API Docs
#
#  Usage:
#   go install github.com/swaggo/swag/cmd/swag@latest
#   cd /opt/src/app
#   docs/generate.sh # must be called in the src root dir
#
#   go run cmd/app/main.go
#   https://localhost/swagger
#
swag fmt
swag init -g cmd/s0counter/main.go