
build:
	@go build -o bin/atomicbase

run: build
	@./bin/atomicbase

test:
	@go test -v ./...