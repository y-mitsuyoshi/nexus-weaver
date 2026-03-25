# nexus-weaver

YAML で定義したワークフローに従い、複数の LLM エージェントを協調させて開発ライフサイクルを自律的に実行する Go 製 CLI オーケストレーターです。

**アイデア → PRD → 設計 → 実装 → テスト → コードレビュー → PR 作成** までを 1 本のパイプラインとして自動化します。

## 主な機能

| ステップタイプ | 概要 |
|---------------|------|
| `llm_task` | LLM にテキスト生成を依頼し、結果をファイルに保存 |
| `loop` | テストコマンド実行 → 失敗時に Fixer LLM で自動修正（リトライ付き） |
| `review` | LLM によるコードレビューと対話的な承認・自動修正（レビューゲート） |
| `git_branch` | 動的に生成したブランチ名で新規ブランチを作成 |
| `git_push` | リモートリポジトリへプッシュ |
| `command_task` | 任意のシェルコマンド実行（PR 作成など） |

### 安全機構

- `loop` / `review` ステップでは実行前に Git 自動コミット（AutoCommit）を行い、リトライ上限到達やリジェクト時に自動ロールバック
- 保護ブランチ（main, master, develop）での直接実行を防止
- 連続する `review` ステップはレビューゲートとしてグループ化され、修正発生時にゲート全体を再実行

## ディレクトリ構成

```text
nexus-weaver/
├── .github/
│   ├── agents/           # Copilot カスタムエージェント定義
│   └── workflows/        # GitHub Actions CI
├── cmd/nexus-weaver/     # CLI エントリーポイント
├── internal/
│   ├── engine/           # ワークフローパーサー & 実行エンジン
│   ├── fs/               # ファイル I/O・Git ユーティリティ
│   └── llm/              # LLM プロバイダ実装
├── prompts/              # エージェントロール別システムプロンプト
├── workflows/            # ワークフロー定義 YAML
└── docs/                 # 生成物・設計書
```

## 前提条件

- **Docker** および **Docker Compose**
- **Git**
- 利用する LLM に応じた外部ツール（後述）

### 利用可能な LLM プロバイダ

| モデル名 | 用途 | 必要なもの |
|----------|------|-----------|
| `gemini-cli` | 高推論タスク（PRD・設計・レビュー） | コンテナ内で `gemini` コマンドが実行可能であること |
| `copilot-cli` | 汎用コーディング | コンテナ内で `copilot` コマンドが実行可能であること |
| `local-qwen` | コスト・速度重視の実装タスク | OpenAI 互換 API（Ollama 等）が稼働していること |

> `local-qwen` のエンドポイントは環境変数 `LOCAL_QWEN_ENDPOINT` で設定可能です（デフォルト: `http://localhost:11434/v1/chat/completions`）。
> Docker 環境では `docker-compose.yml` の `network_mode: "host"` により、ホスト側の `localhost` に直接到達します（Linux のみ）。

## セットアップ

### 1. Docker イメージをビルド

```bash
docker compose build
```

### 2. 入力ファイルを準備

ワークフローが参照する入力ファイル（例: `inbox/idea.txt`）を配置します。

```bash
mkdir -p inbox
echo "実装したい機能のアイデアをここに記述" > inbox/idea.txt
```

## 使い方

### ワークフローのバリデーション（ドライラン）

```bash
docker compose run --rm nexus-weaver --dry-run
```

### ワークフローの実行

```bash
docker compose run --rm nexus-weaver
```

### 詳細ログの有効化

```bash
docker compose run --rm nexus-weaver --verbose
```

### カスタムワークフローの指定

```bash
docker compose run --rm nexus-weaver --workflow workflows/custom.yml
```

## CLI オプション

| Flag | Default | Description |
|------|---------|-------------|
| `--workflow` | `workflows/workflow.yml` | ワークフロー定義 YAML のパス |
| `--verbose` | `false` | 詳細ログを出力する |
| `--dry-run` | `false` | 読み込みと検証のみ行い、実行はしない |

## ワークフローの書き方

ワークフローは `name` と `steps` で構成されます。各ステップは `id`（一意）と `type` を持ち、タイプに応じたフィールドを設定します。

```yaml
name: "Feature Development Pipeline"
steps:
  # 1. ブランチ名を LLM に生成させる
  - id: "generate_branch_name"
    type: "llm_task"
    agent_role: "ReleaseEngineer"
    model: "gemini-cli"
    system_prompt_file: "./prompts/branch_namer.md"
    input_file: "./inbox/idea.txt"
    output_file: "./docs/branch_name.txt"

  # 2. ブランチを作成
  - id: "branch_creation"
    type: "git_branch"
    branch_name_file: "./docs/branch_name.txt"

  # 3. PRD を生成
  - id: "prd_generation"
    type: "llm_task"
    agent_role: "ProductManager"
    model: "gemini-cli"
    system_prompt_file: "./prompts/pm.md"
    input_file: "./inbox/idea.txt"
    output_file: "./docs/prd.md"

  # 4. PRD をレビュー（修正→再レビューのゲート）
  - id: "prd_review"
    type: "review"
    reviewer_model: "gemini-cli"
    review_prompt_file: "./prompts/prd_reviewer.md"
    target_file: "./docs/prd.md"
    max_retries: 3
    fixer_model: "gemini-cli"
    fixer_prompt_file: "./prompts/document_fixer.md"

  # 5. リモートにプッシュ
  - id: "push_to_remote"
    type: "git_push"
    remote: "origin"
```

完全なワークフロー例は `workflows/workflow.yml` を参照してください。

## ステップ種別リファレンス

### `llm_task`

LLM にテキスト生成を依頼し、結果を `output_file` に保存します。
出力ファイルの拡張子（`.go`, `.yml` など）に応じて、LLM 応答から自動的にコードブロックが抽出されます。

| フィールド | 必須 | 説明 |
|-----------|:----:|------|
| `id` | o | ステップの一意な識別子 |
| `type` | o | `llm_task` |
| `model` | o | `gemini-cli` / `copilot-cli` / `local-qwen` |
| `agent_role` | | ログ出力用の役割名 |
| `system_prompt_file` | | システムプロンプトのファイルパス |
| `input_file` | | 入力ファイルのパス |
| `output_file` | | 出力先ファイルのパス |

### `loop`

テストコマンドを実行し、失敗時に Fixer LLM で自動修正するリトライループです。

| フィールド | 必須 | 説明 |
|-----------|:----:|------|
| `id` | o | ステップの一意な識別子 |
| `type` | o | `loop` |
| `command` | o | 実行するテストコマンド |
| `max_retries` | o | 最大リトライ回数（> 0） |
| `target_file` | o | 修正対象のファイル |
| `fixer_model` | | 修正に使用するモデル |
| `fixer_prompt_file` | | Fixer 用システムプロンプト |

- 各リトライ前に AutoCommit で安全装置を確保
- 最終リトライ失敗時は自動ロールバック
- Fixer 応答からフェンスドコードブロックを抽出して `target_file` に適用

### `review`

対象ファイルを LLM にレビューさせ、ユーザーに対話的な承認を求めます。

| フィールド | 必須 | 説明 |
|-----------|:----:|------|
| `id` | o | ステップの一意な識別子 |
| `type` | o | `review` |
| `reviewer_model` | o | レビューを行うモデル |
| `target_file` | o | レビュー対象のファイル |
| `review_prompt_file` | | レビュアー用システムプロンプト |
| `max_retries` | | レビューゲートの最大ラウンド数 |
| `fixer_model` | | 修正に使用するモデル（省略時は `reviewer_model`） |
| `fixer_prompt_file` | | Fixer 用システムプロンプト |

- ユーザーに `[a]pprove / [f]ix & approve / [r]eject / [q]uit` の選択肢を提示
- `f` で Fixer に修正を依頼し `target_file` に適用
- 連続する `review` ステップはレビューゲートとして扱われ、修正発生時にゲート全体を再実行
- レビュー結果は `docs/reviews/review-<ID>.md` に保存

### `git_branch`

新しい Git ブランチを作成してチェックアウトします。

| フィールド | 必須 | 説明 |
|-----------|:----:|------|
| `id` | o | ステップの一意な識別子 |
| `type` | o | `git_branch` |
| `branch_name` | △ | ブランチ名（直接指定） |
| `branch_name_file` | △ | ブランチ名が記載されたファイルパス |

- `branch_name` か `branch_name_file` のいずれかが必須
- `feature/`, `fix/`, `improvement/` プレフィックスがない場合、自動的に `feature/` を付与

### `git_push`

現在のブランチをリモートにプッシュします。

| フィールド | 必須 | 説明 |
|-----------|:----:|------|
| `id` | o | ステップの一意な識別子 |
| `type` | o | `git_push` |
| `remote` | | プッシュ先リモート名（デフォルト: `origin`） |

### `command_task`

任意のシェルコマンドを `sh -c` で実行します。

| フィールド | 必須 | 説明 |
|-----------|:----:|------|
| `id` | o | ステップの一意な識別子 |
| `type` | o | `command_task` |
| `command` | o | 実行するコマンド |

## Docker 環境の詳細

### docker-compose.yml

```yaml
services:
  nexus-weaver:
    build: .
    network_mode: "host"
    volumes:
      - .:/app
    command: ["--workflow", "workflows/workflow.yml"]
```

- カレントディレクトリを `/app` にマウントし、ワークフローがファイルの読み書き・Git 操作を行える状態にします
- `network_mode: "host"` により、ホスト上で動作する `local-qwen`（Ollama 等）に `localhost` で到達可能です（**Linux のみ**。macOS/Windows の Docker Desktop では `host.docker.internal` への変更が必要です）
- コンテナ内で Git の `user.name` / `user.email` を自動設定済みのため、AutoCommit が正常に動作します

### LLM CLI ツールの利用

Docker イメージには `gemini` / `copilot` CLI は含まれていません。利用する場合は以下のいずれかの方法で対応してください:

- Dockerfile に CLI のインストールを追加する
- ホスト側のバイナリをボリュームマウントで `/usr/local/bin/` に配置する

## CI

GitHub Actions で自動テスト・リントが実行されます（`.github/workflows/test.yml`）。

- `gofmt` / `go vet` / `golangci-lint` による静的解析
- `go test -v -count=1 ./...` によるユニットテスト

## 開発

開発者向けのローカルビルド・テスト手順です。

```bash
# テスト実行
docker compose run --rm --entrypoint "" nexus-weaver go test -v -count=1 ./...

# Docker を使わない場合（Go 1.24 以上が必要）
go test -v -count=1 ./...
```

## License

See [LICENSE](LICENSE) file.