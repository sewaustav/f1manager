BIN_DIR := bin
KEYS_DIR := keys

.PHONY: all build build-cli build-web run-web run-cli test test-race vet fmt keys tidy clean

all: build

## build: собрать обе версии (cli + web)
build: build-cli build-web

## build-cli: собрать CLI-версию в bin/sim
build-cli:
	go build -o $(BIN_DIR)/sim ./cmd/sim

## build-web: собрать веб-версию в bin/web
build-web:
	go build -o $(BIN_DIR)/web ./cmd/web

## run-web: запустить веб-сервер
run-web:
	go run ./cmd/web

## run-cli: запустить CLI
run-cli:
	go run ./cmd/sim

## test: прогнать все тесты
test:
	go test ./...

## test-race: тесты с детектором гонок
test-race:
	go test -race ./...

## vet: статический анализ
vet:
	go vet ./...

## fmt: форматирование
fmt:
	gofmt -w $(shell find . -name '*.go' -not -path './vendor/*')

## keys: сгенерировать RSA-ключи для JWT
keys:
	mkdir -p $(KEYS_DIR)
	openssl genrsa -out $(KEYS_DIR)/jwt_private.pem 2048
	openssl rsa -in $(KEYS_DIR)/jwt_private.pem -pubout -out $(KEYS_DIR)/jwt_public.pem

## tidy: привести go.mod/go.sum в порядок
tidy:
	go mod tidy

## clean: удалить артефакты сборки
clean:
	rm -rf $(BIN_DIR)
