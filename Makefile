.PHONY: install run build start check clean

install: ## pasang/rapihkan dependensi
	go mod tidy

run: ## jalankan server dev (butuh file .env)
	go run ./cmd/api

build: ## compile binary ke bin/openpos-api
	go build -o bin/openpos-api ./cmd/api

start: build ## build lalu jalankan hasil compile
	./bin/openpos-api

check: ## vet + pastikan semua paket ter-build
	go vet ./... && go build ./...

test: ## jalankan semua test (butuh file .env)
	set -a; . ./.env; set +a; go test ./...

clean: ## hapus artefak build
	rm -rf bin/
