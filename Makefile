BINARY := mycli

.PHONY: build run clean test

build:
	go build -o $(BINARY) .

run: build
	./$(BINARY)

test:
	go test ./...

clean:
	rm -f $(BINARY)
