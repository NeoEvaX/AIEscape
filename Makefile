.PHONY: build
build:
	go build -o bin/ai-escape ./app/cmd

.PHONY: run
run: build
	./bin/ai-escape

.PHONY: test
test:
	go test -v ./...

