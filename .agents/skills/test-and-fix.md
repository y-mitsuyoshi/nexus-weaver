---
description: "テスト実行と失敗時の自動修正のためのスキル"
---

# テスト & 自動修正スキル

## テスト実行手順

### 1. 全テスト実行

```bash
docker compose run --rm --entrypoint "" nexus-weaver go test -v -count=1 ./...
```

### 2. 特定パッケージのテスト

```bash
# LLM プロバイダのテスト
docker compose run --rm --entrypoint "" nexus-weaver go test ./internal/llm/ -v

# エンジン（ワークフロー + ランナー）のテスト
docker compose run --rm --entrypoint "" nexus-weaver go test ./internal/engine/ -v

# ファイル I/O のテスト
docker compose run --rm --entrypoint "" nexus-weaver go test ./internal/fs/ -v
```

### 3. カバレッジ確認

```bash
docker compose run --rm --entrypoint "" nexus-weaver sh -c "go test ./... -coverprofile=coverage.out && go tool cover -func=coverage.out"
```

## テスト失敗時の修正フロー

テストが失敗した場合は、以下の手順に従ってください:

### Step 1: エラー出力を正確に読む

```bash
docker compose run --rm --entrypoint "" nexus-weaver go test -v -count=1 ./... 2>&1
```

- `FAIL` の行でどのテスト関数が失敗したか確認
- エラーメッセージからの「expected vs got」の差分を確認
- パニックの場合はスタックトレースから発生箇所を特定

### Step 2: 根本原因を分類する

| 原因カテゴリ | 対処 |
|-------------|------|
| **コンパイルエラー** | 型やインポートの修正 |
| **アサーション失敗** | 実装ロジックまたはテスト期待値の修正 |
| **パニック（nil参照）** | nil チェックの追加 |
| **タイムアウト** | テスト設定の見直し、またはデッドロックの修正 |
| **外部依存エラー** | モック化の検討 |

### Step 3: 最小限の修正を適用する

- 変更は **壊れたテストに直接関連する箇所のみ** に限定
- 修正後、再度テストを実行して全テストがパスすることを確認

### Step 4: 安全装置

nexus-weaver のランナーエンジン (`runner.go`) は、テスト実行前に
自動で Git コミットを行います。手動で作業している場合も、
修正前に以下を実行しておくと安全です:

```bash
git add . && git commit -m "pre-fix: before test fix attempt"
```

万が一修正が悪化した場合:

```bash
git reset --hard HEAD~1
```

## CI 環境でのテスト

GitHub Actions (`.github/workflows/test.yml`) で自動実行されます:
- Push 時: `main`, `master` ブランチ
- Pull Request 時: `main`, `master` ブランチ向け

CI で失敗した場合は、Docker Compose で同じ Go バージョン (1.24) のテストを再現してください。

```bash
docker compose run --rm --entrypoint "" nexus-weaver go test -v -count=1 ./...
```
