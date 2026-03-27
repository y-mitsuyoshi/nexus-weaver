# nexus-weaver 開発ガイド — 共通プロジェクトインストラクション

## 言語ルール

- **すべての応答・説明・コメントは日本語で行ってください**
- コード中の識別子（変数名・関数名・型名）は英語のまま
- GoDoc コメントは日本語で記述可
- Git コミットメッセージは日本語で記述可

## プロジェクト概要

nexus-weaver は、YAML 駆動ワークフローで複数の LLM エージェントを協調させ、
PRD 作成 → 設計 → 実装 → テスト → PR 作成までの開発ライフサイクルを自律的に
実行する **CLI ベースのオーケストレーター** です。Go 言語で実装されています。

## ディレクトリ構成

```
nexus-weaver/
├── cmd/nexus-weaver/         # CLI エントリーポイント (main.go)
├── internal/
│   ├── engine/               # ワークフローパーサー & ステートマシン実行エンジン
│   │   ├── workflow.go       # Workflow/Step 構造体, YAML パーサー, バリデーション
│   │   └── runner.go         # Run() メインループ: llm_task / loop / command_task
│   ├── llm/                  # LLMProvider インターフェース & アダプター群
│   │   ├── provider.go       # LLMProvider interface + GetProvider ファクトリ
│   │   ├── gemini_cli.go     # Gemini CLI アダプター (os/exec)
│   │   ├── copilot_cli.go    # GitHub Copilot CLI アダプター (os/exec)
│   │   └── local_qwen.go     # LocalQwen アダプター (OpenAI 互換 HTTP)
│   └── fs/                   # ファイル I/O & Git ユーティリティ
│       ├── file.go           # ReadFile, WriteFile, ExtractCodeBlock
│       └── git.go            # AutoCommit, Rollback
├── workflows/                # パイプライン定義 YAML
├── prompts/                  # エージェントロール別システムプロンプト
├── docs/plans/               # 実装計画書 (gitignore 対象)
├── .agents/                  # 共有ワークフロー & スキル
│   ├── workflows/            # AI ワークフロー定義
│   └── skills/               # 開発スキル定義
├── .github/
│   ├── agents/               # Copilot CLI サブエージェント
│   ├── copilot-instructions.md
│   └── workflows/test.yml    # CI (GitHub Actions)
├── Dockerfile
├── docker-compose.yml
└── GEMINI.md                 # Gemini CLI 用インストラクション
```

## 技術スタック & 規約

- **言語**: Go 1.24+
- **依存**: `gopkg.in/yaml.v3` (YAML パーサー)
- **アーキテクチャ**: クリーンアーキテクチャ。外部依存 (LLM, ファイルシステム, Git) は全て interface で抽象化
- **テスト**: `make test` または `docker compose run --rm test`
- **ログ**: `log/slog` による構造化ログ
- **ビルド**: `make build`（バージョン情報付き）/ `make install`（グローバルインストール）
- **コンテナ**: Docker (golang:1.24-alpine ベース、マルチステージビルド)
- **実行**: `nexus-weaver`（ネイティブ）または `docker compose run --rm nexus-weaver`

## コーディング規約

1. **Go の慣習に従う**: `gofmt`, `go vet`, `golint` 準拠
2. **全ての公開関数・型に GoDoc コメント**を付ける（日本語可）
3. **エラーハンドリング**: `fmt.Errorf("context: %w", err)` でラップし、コンテキストを付加
4. **テスト**: 新しい関数を追加したら、必ず `_test.go` にユニットテストも追加
5. **インターフェース**: 新しい外部依存は必ず interface で抽象化する
6. **ファイル構成**: 1 ファイル 1 責務。構造体ごとにファイルを分離

## コアインターフェース

```go
// LLMProvider — LLM アダプターの共通インターフェース
type LLMProvider interface {
    Generate(systemPrompt, userPrompt string) (string, error)
}
```

サポート中のプロバイダ: `gemini-cli`, `copilot-cli`, `local-qwen`

## ワークフローのステップタイプ

| タイプ | 説明 |
|--------|------|
| `llm_task` | LLM に生成を依頼し、結果をファイルに保存 |
| `loop` | テストコマンド実行 → 失敗時に Fixer LLM で自動修正（リトライ付き） |
| `review` | LLM によるコードレビューと対話的な承認・自動修正（レビューゲート） |
| `git_branch` | 動的に生成したブランチ名で新規ブランチを作成 |
| `git_push` | リモートリポジトリへプッシュ |
| `command_task` | シェルコマンドの実行（PR 作成など） |

## Git 運用ルール（GitHub Flow）

このプロジェクトは **GitHub Flow** に従います。

### 必須ルール

1. **main ブランチへの直接コミット禁止** — 新規開発・バグ修正・リファクタリング等すべての変更は feature ブランチで行う
2. **feature ブランチの命名規則** — 変更内容に応じたプレフィックスを付ける:
   - `feature/<説明>` — 新機能
   - `fix/<説明>` — バグ修正
   - `improvement/<説明>` — リファクタリング・改善
3. **ブランチ作成 → コミット → プッシュ → PR** の流れで進める
4. テスト実行前に変更をコミットする（テスト中の `git checkout` で未コミット変更が失われるのを防止）
5. テスト用に作成した一時ブランチは `t.Cleanup()` で自動削除する

### AI エージェントへの指示

- コード変更を行う際は、**作業開始前に適切な名称の feature ブランチを作成**してください
- main ブランチに直接コミット・プッシュしないでください
- テスト完了後、ブランチを main に戻す必要はありません（feature ブランチ上に留まる）
- プッシュ先はリモートの同名ブランチです（`git push origin feature/xxx`）

## 重要な設計判断

- `runner.go` の `runTestLoop` は各リトライ前に `fs.AutoCommit()` で Git コミットを行う（安全装置）
- `fs.ExtractCodeBlock()` は LLM 出力の Markdown からコードブロックを正規表現で抽出する
- プロバイダ選択は `GetProvider()` ファクトリ関数で一元管理
