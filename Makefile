.PHONY: web build build-arm64 run clean docker docker-arm64 docker-multiarch docker-up docker-down

web:
	cd web && npm install && npm run build

# Native build for the host arch.
build: web
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o webtermin ./cmd/webtermin

# Cross-compile for OrangePi 5 Pro / any aarch64 Linux box.
build-arm64: web
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 \
	  go build -trimpath -ldflags="-s -w" -o webtermin-arm64 ./cmd/webtermin
	@echo "Built webtermin-arm64 — scp it to the OrangePi and run."

run: build
	./webtermin -config config.yaml

clean:
	rm -rf webtermin webtermin-arm64 data web/dist web/node_modules

# Docker — uses buildx so arm64 builds work from an x86 dev machine.
docker:
	docker build -t webtermin:local .

docker-arm64:
	docker buildx build --platform linux/arm64 -t webtermin:arm64 --load .

docker-multiarch:
	docker buildx build --platform linux/amd64,linux/arm64 -t webtermin:latest --push .

docker-up:
	docker compose up -d --build

docker-down:
	docker compose down
