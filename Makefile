.SILENT:
.PHONY: build

APP:=gosync
ROOT:=$(shell pwd -P)
VERSION:=$(shell grep 'var Version =' ${ROOT}/conf/version.go | cut -d '"' -f2)
GIT_COMMIT:=$(shell git --work-tree ${ROOT} rev-parse --short 'HEAD^{commit}')
BUILD_DATE:=$(shell date "+%Y-%m-%d %H:%M:%S")
LDFLAGS:="-w -s -X 'main.commit=$(GIT_COMMIT)' -X 'main.buildDate=$(BUILD_DATE)'"
STYLE_CHECK_GOFILE  := $$(find . -name '*.go')

all: build

vendor:
	GOPROXY=https://goproxy.cn go mod vendor

vendor-ci:
	go mod vendor

build:
	echo "Building version $(VERSION)..."
	go build -ldflags $(LDFLAGS) -o dist/$(APP) cmd/gosync/main.go
	echo "Build successfully."
