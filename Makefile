APP := llamasitter

.PHONY: build test fmt run build-macos-app

build:
	go build -o bin/$(APP) ./cmd/llamasitter

test:
	go test ./...

fmt:
	go fmt ./...

run:
	go run ./cmd/llamasitter serve -config config.example.yaml

build-macos-app:
	bash ./scripts/build-macos-app.sh
