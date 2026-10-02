PREFIX ?= $(HOME)/.local

.PHONY: all generate test install clean

all:
	go build -buildvcs=false -o bin/gohome ./cmd/gohome

generate:
	python3 scripts/assets.py
	gofmt -w internal/compiler/assets.go

test:
	go test ./...

install: all
	mkdir -p $(PREFIX)/bin
	install -m755 bin/gohome $(PREFIX)/bin/gohome

clean:
	rm -rf bin examples/hello-tweak/build examples/hello-app/build
