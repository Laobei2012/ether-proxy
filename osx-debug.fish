#!/usr/bin/env fish

env GORACE="log_path=race.log" go run -race ./cmd/ether-proxy $argv
