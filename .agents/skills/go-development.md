---
description: "nexus-weaver プロジェクトにおける Go 開発のベストプラクティスとパターン"
---

# Go 開発スキル

## プロジェクト固有の Go 開発ルール

### 新しいファイルを追加する場合

1. `internal/` 配下の適切なパッケージに配置する
2. ファイル名はスネークケース（例: `gemini_cli.go`）
3. 同じディレクトリに `*_test.go` も作成する

### 新しい LLM プロバイダを追加する場合

```bash
# 1. アダプターファイルを作成
touch internal/llm/new_provider.go

# 2. LLMProvider interface を実装
#    必須メソッド: Generate(systemPrompt, userPrompt string) (string, error)

# 3. GetProvider() ファクトリにルーティングを追加
#    internal/llm/provider.go の switch 文に case を追加

# 4. テストを追加・実行
#    internal/llm/provider_test.go にインターフェース準拠チェックと
#    GetProvider テストを追加

# 5. 全テスト実行で確認
docker compose run --rm --entrypoint "" nexus-weaver go test -v -count=1 ./...
```

### ワークフローに新しいステップタイプを追加する場合

1. `internal/engine/workflow.go` の `Step` 構造体にフィールド追加
2. `validateWorkflow()` にバリデーションルール追加
3. `internal/engine/runner.go` の `Run()` メソッドの switch 文に case 追加
4. テストを追加

### エラーハンドリングパターン

```go
// ✅ 正しい: コンテキスト付きでラップ
if err != nil {
    return fmt.Errorf("failed to load workflow from %s: %w", path, err)
}

// ❌ 間違い: コンテキストなし
if err != nil {
    return err
}
```

### テストパターン

```go
// テーブル駆動テストを推奨
func TestSomething(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        want    string
        wantErr bool
    }{
        {"valid input", "hello", "HELLO", false},
        {"empty input", "", "", true},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := Something(tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("error = %v, wantErr %v", err, tt.wantErr)
            }
            if got != tt.want {
                t.Errorf("got %q, want %q", got, tt.want)
            }
        })
    }
}
```

### よく使うコマンド

```bash
# ビルド
docker compose build

# 全テスト実行
docker compose run --rm --entrypoint "" nexus-weaver go test -v -count=1 ./...

# 特定パッケージのテスト
docker compose run --rm --entrypoint "" nexus-weaver go test ./internal/llm/ -v

# カバレッジ付きテスト
docker compose run --rm --entrypoint "" nexus-weaver sh -c "go test ./... -coverprofile=coverage.out && go tool cover -func=coverage.out"

# コードフォーマット
docker compose run --rm --entrypoint "" nexus-weaver gofmt -w .

# 静的解析
docker compose run --rm --entrypoint "" nexus-weaver go vet ./...

# ドライラン
docker compose run --rm nexus-weaver --dry-run
```
