build:
    go build ./...

test:
    go test -race ./...

vet:
    go vet ./...

check: vet test
