.PHONY: all build build-web build-go test clean run docker-build

all: build

build-web:
	cd web && npm run build

build-go:
	CGO_ENABLED=0 go build -ldflags="-s -w" -o pdswarm ./cmd/pdswarm

build: build-web build-go

test:
	go test -v ./...

clean:
	rm -rf pdswarm pdswarm.db web/dist

run: build
	./pdswarm

docker-build:
	docker build -t pdswarm:latest .
