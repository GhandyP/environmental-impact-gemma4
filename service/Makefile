# Common development and container commands for the EIA service.
.PHONY: build run demo test vet fmt tidy docker-build docker-run clean

build:
	go build -o eia-service ./cmd/server

run:
	./eia-service

demo:
	EIA_MOCK=1 ./eia-service

test:
	go test -race ./...

vet:
	go vet ./...

fmt:
	gofmt -w .

tidy:
	go mod tidy

docker-build:
	docker build -t eia-service:local .

docker-run:
	docker run --rm -p 8080:8080 -e EIA_MOCK=1 eia-service:local

clean:
	rm -f eia-service
