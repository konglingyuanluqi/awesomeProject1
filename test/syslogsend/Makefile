GITCOMMIT:=$(shell git describe --dirty --always)
VS ?= "${GITCOMMIT}"

.PHONY: all
all: build

.PHONY: build
build:
	@go build -ldflags=" -s -w" -o sendsyslog
	@GOOS=linux GOARCH=amd64 go build -ldflags=" -s -w" -o sendsysloglinux

run:
	@go run main.go  -c ./conf/.env.yaml


.PHONY: build-arm
build-arm:
	@go build -ldflags=" -s -w" -o sendsyslog
	@GOOS=linux GOARCH=arm64 go build -ldflags=" -s -w" -o sendsysloglinuxarm
