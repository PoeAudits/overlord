.PHONY: build test vet fmt check

build:
	go build -o overlord ./cmd/overlord

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -w .

# Everything CI and reviewers expect to pass.
check:
	go test ./...
	go vet ./...
	test -z "$$(gofmt -l .)"
	go build ./...
	git diff --check
