.PHONY: run build vet fmt clean

BINARY := urlchecker

run:
	go run .

build:
	go build -o $(BINARY) .

vet:
	go vet ./...

fmt:
	gofmt -w .
	test -z "$$(gofmt -l .)" || (echo "unformatted files"; exit 1)

clean:
	rm -f $(BINARY)
