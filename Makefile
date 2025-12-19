.PHONY: build run migrate clean install

build: tidy
	go build -o bin/dream-ai ./cmd/dream-ai

tidy:
	go mod tidy

run: build
	./bin/dream-ai

migrate:
	go run ./cmd/dream-ai -migrate

install:
	go install ./cmd/dream-ai

clean:
	rm -rf bin/

test:
	go test ./...
