#!/bin/bash
# Builds both binaries into ./dist
set -e
mkdir -p dist
go build -ldflags="-s -w" -o dist/ether-proxy ./cmd/ether-proxy
go build -ldflags="-s -w" -o dist/rig-agent ./cmd/rig-agent
