APP := safe
MODULE := github.com/davidmuller5273-boop/safe
GOOS ?= linux
GOARCH ?= amd64
BIN := bin/$(APP)

.PHONY: all build build-local test tidy setup-frontend clean

all: build

build:
	@mkdir -p bin
	GOOS=$(GOOS) GOARCH=$(GOARCH) CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o $(BIN) .
	@echo "built $(BIN) ($(GOOS)/$(GOARCH))"

build-local:
	@mkdir -p bin
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o $(BIN) .
	@echo "built $(BIN) (native)"

test:
	go test ./...

tidy:
	go mod tidy

setup-frontend:
	cd chatadmin && npm install && npm run build

clean:
	rm -f bin/$(APP)
