.PHONY: build build-local install test test-local lint lint-local clean

# バージョン情報
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
LDFLAGS := -ldflags "-s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT)"

# --- Docker ターゲット (推奨) ---

# Docker イメージをビルド
build:
	docker compose build

# テストを Docker で実行
test:
	docker compose run --rm test

# リントを Docker で実行
lint:
	docker compose run --rm --entrypoint "sh" test -c "gofmt -l . && go vet ./..."

# --- ローカル ターゲット (Go 1.24+ が必要) ---

# バイナリをローカルでビルド
build-local:
	go build $(LDFLAGS) -o nexus-weaver ./cmd/nexus-weaver

# グローバルインストール（$GOPATH/bin に配置）
install:
	go install $(LDFLAGS) ./cmd/nexus-weaver

# テストをローカルで実行
test-local:
	go test -v -count=1 ./...

# リントをローカルで実行
lint-local:
	gofmt -l . && go vet ./...

# --- ユーティリティ ---

# クリーン
clean:
	rm -f nexus-weaver
	docker compose down --remove-orphans
