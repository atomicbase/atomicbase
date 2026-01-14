
build:
	@CGO_ENABLED=1 go build -tags fts5 -o bin/atomicbase

run: build
	@./bin/atomicbase

test:
	@CGO_ENABLED=1 go test -tags fts5 -v ./...

test-integration:
	@CGO_ENABLED=1 go test -tags "fts5 integration" -v ./database/...
