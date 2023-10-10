.PHONY: build
.DEFAULT_GOAL := build

build:
	CGO_ENABLED=false GOARCH=amd64 GOOS=linux go build -o sway-fader main.go
