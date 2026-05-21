.PHONY: build test lint generate docker-build clean migrate-up migrate-down

build:
	go build ./...

test:
	go test ./... -race -cover

lint:
	golangci-lint run

generate:
	buf generate
	sqlc generate

docker-build:
	docker build -t omnitun-server -f deploy/docker/Dockerfile.server .

docker-build-client:
	docker build -t omnitun-client -f deploy/docker/Dockerfile.client .

docker-build-relay:
	docker build -t omnitun-relay -f deploy/docker/Dockerfile.relay .

clean:
	rm -rf gen/

migrate-up:
	go run ./cmd/tools/migrate --direction up

migrate-down:
	go run ./cmd/tools/migrate --direction down
