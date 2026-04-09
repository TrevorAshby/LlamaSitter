APP := llamasitter

.PHONY: build test fmt run build-macos-app cli-docs package-release package-checksums

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

cli-docs:
	go run ./scripts/generate_cli_docs.go

package-release:
	bash ./scripts/package-release.sh package --version "$(VERSION)" --target "$(TARGET)"

package-checksums:
	bash ./scripts/package-release.sh checksums
