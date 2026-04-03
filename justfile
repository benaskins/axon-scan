build:
    go build -o bin/axon-scan ./cmd/axon-scan

install: build
    cp bin/axon-scan ~/.local/bin/axon-scan

test:
    go test ./...

vet:
    go vet ./...
