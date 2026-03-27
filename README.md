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

- **Go 1.24+**（`go install` を使う場合）または **Docker** + **Docker Compose**
- **Git**
- 利用する LLM に応じた外部ツール（後述）

### 利用可能な LLM プロバイダ

| プロバイダ名 | 用途 | 必要なもの |
|-------------|------|-----------|
| `gemini-cli` | 高推論タスク（PRD・設計・レビュー） | `gemini` コマンドが実行可能であること |
| `copilot-cli` | 汎用コーディング | `copilot` コマンドが実行可能であること |
| `local-qwen` | コスト・速度重視の実装タスク | OpenAI 互換 API（Ollama 等）が稼働していること |

> `local-qwen` のエンドポイントは環境変数 `LOCAL_QWEN_ENDPOINT` で設定可能です（デフォルト: `http://localhost:11434/v1/chat/completions`）。

### プロバイダとモデルの分離指定

ワークフロー YAML では、全ステップタイプ共通で `provider` と `model` の2フィールドで LLM を指定します：

```yaml
# llm_task: テキスト生成に使用する LLM
- id: "prd_generation"
  type: "llm_task"
  provider: "gemini-cli"
  model: "gemini-2.5-flash"

# review: レビュー & 修正に使用する LLM（同一LLMで両方実行）
- id: "code_review"
  type: "review"
  provider: "gemini-cli"

# loop: テスト失敗時の Fixer LLM
- id: "test_loop"
  type: "loop"
  provider: "copilot-cli"
  model: "claude-sonnet-4"
```

- `model` 省略時はプロバイダのデフォルトモデル、`"default"` も同義
- `provider` か `model` のいずれかは必須（`model` のみの場合は後方互換としてプロバイダ名を直接指定可）

CLI の `--model` フラグでは `provider:model` 形式も利用可能です（後方互換）：

```bash
nexus-weaver -p "質問" --model "gemini-cli:gemini-2.5-flash"
nexus-weaver -p "質問" --model "copilot-cli:claude-opus-4"
```

## インストール

### 方法 1: `go install`（推奨）

Go がインストールされている環境で、以下のコマンドでグローバルにインストールできます：

```bash
go install github.com/y-mitsuyoshi/nexus-weaver/cmd/nexus-weaver@latest
```

これで `nexus-weaver` コマンドがどのディレクトリからでも利用可能になります。

> **Note**: `$GOPATH/bin`（通常 `~/go/bin`）にパスが通っていることを確認してください。

#### ソースからビルドする場合

```bash
git clone https://github.com/y-mitsuyoshi/nexus-weaver.git
cd nexus-weaver
make install  # バージョン情報付きでインストール
```

### 方法 2: Docker / Docker Compose（推奨）

```bash
git clone https://github.com/y-mitsuyoshi/nexus-weaver.git
cd nexus-weaver
make build    # docker compose build のエイリアス
```

これで環境が整います。開発中のテストやリントも、`make test` や `make lint` を叩くだけで Docker コンテナ内で実行されます。

## 他のリポジトリで使う

nexus-weaver は **任意のリポジトリのカレントディレクトリ** を起点にワークフローを実行します。
Claude Code や Aider と同じ設計思想で、ツール自体のコードは対象プロジェクトに含めません。

### 1. 対象リポジトリでプロジェクトを初期化

```bash
cd /path/to/your-project
nexus-weaver --init
```

これにより、`.nexus/` ディレクトリとワークフローの雛形が作成されます。

### 2. 命令を直接渡して実行（クイックスタート）

```bash
# プロンプトを直接指定（自動的に inbox/idea.txt に書き込まれる）
nexus-weaver "ユーザー一覧取得APIを実装して"

# 参照ファイル（--input）を添えて実行
nexus-weaver "既存のコードをリファクタリングして" --input src/handler.go --input src/model.go
```

このコマンドを実行すると、入力されたメッセージと参照ファイルの内容が自動的にワークフローの入力ファイル（`inbox/idea.txt`）として保存され、パイプラインが開始されます。


### ワークフロー探索順序

`--workflow` を省略した場合、以下の順序で自動探索します：

1. `.nexus/workflow.yml`
2. `.nexus/workflow.yaml`
3. `workflows/workflow.yml`
4. `workflows/workflow.yaml`

## 入力ファイルを準備

ワークフローが参照する入力ファイル（例: `inbox/idea.txt`）を配置します。

```bash
mkdir -p inbox
echo "実装したい機能のアイデアをここに記述" > inbox/idea.txt
```

## 使い方

### Docker Compose 経由（推奨）

```bash
# ビルド（イメージの作成）
make build

# ワークフローのバリデーション（ドライラン）
docker compose run --rm nexus-weaver --dry-run

# ワークフローの実行
docker compose run --rm nexus-weaver

# テストの実行（Docker コンテナ内）
make test

# リントの実行（Docker コンテナ内）
make lint
```

### ネイティブ実行（go install 後）

```bash
# ワークフローのバリデーション
nexus-weaver --dry-run

# ワークフローの実行
nexus-weaver

# 詳細ログの有効化
nexus-weaver --verbose

# カスタムワークフローの指定
nexus-weaver --workflow workflows/custom.yml
```

## CLI オプション

| Flag | Default | Description |
|------|---------|-------------|
| `[PROMPT]` | | 余った引数は初期プロンプトとして `inbox/idea.txt` に保存される |
| `--prompt`, `-p` | | 初期プロンプトを明示的に指定する場合に使用 |
| `--model`, `-m` | `gemini-cli` | 使用するLLMプロバイダ名またはモデル指定（`provider:model` 形式も可） |
| `--input` | | 参照ファイルのパス（複数指定可）。プロンプトと結合される |
| `--workflow` | （自動探索） | ワークフロー定義 YAML のパス |
| `--verbose` | `false` | 詳細ログを出力する |
| `--dry-run` | `false` | 読み込みと検証のみ行い、実行はしない |
| `--version` | | バージョン情報を表示して終了 |
| `--init` | | カレントディレクトリに `.nexus/` 設定ディレクトリを生成 |

## ワークフローの書き方

ワークフローは `name` と `steps` で構成されます。各ステップは `id`（一意）と `type` を持ち、タイプに応じたフィールドを設定します。

```yaml
name: "Feature Development Pipeline"
steps:
  # 1. ブランチ名を LLM に生成させる
  - id: "generate_branch_name"
    type: "llm_task"
    agent_role: "ReleaseEngineer"
    provider: "gemini-cli"
    model: "gemini-2.0-flash"
    system_prompt_file: "./prompts/branch_namer.md"
    input_file: "./inbox/idea.txt"
    output_file: "./docs/branch_name.txt"

  # 2. ブランチを作成（GitHub Flow: feature ブランチで開発）
  - id: "branch_creation"
    type: "git_branch"
    branch_name_file: "./docs/branch_name.txt"

  # 3. PRD を生成
  - id: "prd_generation"
    type: "llm_task"
    agent_role: "ProductManager"
    provider: "gemini-cli"
    system_prompt_file: "./prompts/pm.md"
    input_file: "./inbox/idea.txt"
    output_file: "./docs/prd.md"

  # 4. PRD をレビュー（修正→再レビューのゲート）
  - id: "prd_review"
    type: "review"
    provider: "gemini-cli"
    review_prompt_file: "./prompts/prd_reviewer.md"
    target_file: "./docs/prd.md"
    max_retries: 3
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
| `provider` | △ | LLM プロバイダ名（`gemini-cli` / `copilot-cli` / `local-qwen`） |
| `model` | △ | 使用モデル名（省略時はプロバイダのデフォルト） |
| `agent_role` | | ログ出力用の役割名 |
| `system_prompt_file` | | システムプロンプトのファイルパス |
| `input_file` | | 入力ファイルのパス |
| `output_file` | | 出力先ファイルのパス |

> `provider` か `model` のいずれかは必須です。`model` のみ指定の場合は後方互換として `"gemini-cli"` のようにプロバイダ名を直接指定できます。

### `loop`

テストコマンドを実行し、失敗時に Fixer LLM で自動修正するリトライループです。

| フィールド | 必須 | 説明 |
|-----------|:----:|------|
| `id` | o | ステップの一意な識別子 |
| `type` | o | `loop` |
| `command` | o | 実行するテストコマンド |
| `max_retries` | o | 最大リトライ回数（> 0） |
| `target_file` | o | 修正対象のファイル |
| `provider` | | Fixer に使用するプロバイダ |
| `model` | | Fixer に使用するモデル |
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
| `provider` | △ | レビュー & 修正に使用するプロバイダ |
| `model` | △ | レビュー & 修正に使用するモデル |
| `target_file` | o | レビュー対象のファイル |
| `review_prompt_file` | | レビュアー用システムプロンプト |
| `max_retries` | | レビューゲートの最大ラウンド数 |
| `fixer_prompt_file` | | Fixer 用システムプロンプト |

> `provider` か `model` のいずれかは必須です。レビューと修正（Fix）で同じ LLM を使用します。

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

- カレクトディレクトリを `/app` にマウントし、ワークフローがファイルの読み書き・Git 操作を行える状態にします
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

## 開発フロー（GitHub Flow）

nexus-weaver のワークフローは **GitHub Flow** のパターンに沿って動作します：

1. **`main` ブランチは常にデプロイ可能** — 保護ブランチ（`main`, `master`, `develop`）での直接実行を防止する機構が組み込まれています
2. **feature ブランチで作業** — `git_branch` ステップで自動的に `feature/`, `fix/`, `improvement/` プレフィックス付きのブランチを作成
3. **変更をコミット & プッシュ** — `git_push` ステップでリモートにプッシュ、PR 作成用の `command_task` も利用可能
4. **レビュー → マージ** — `review` ステップで LLM レビュー + ユーザー承認のゲートを実現

```text
main ─────────────────────────────────────────────►
       \                                   /
        feature/add-user-api ────────────►
         │  PRD生成 → レビュー → 実装 → テスト → レビュー → Push → PR
```

### 安全機構

| 機構 | 説明 |
|------|------|
| 保護ブランチ検出 | `main`/`master`/`develop` ではワークフロー起動時にエラー（先頭が `git_branch` でない場合） |
| AutoCommit | `loop`/`review` の各リトライ前に自動コミット。暴走時のロールバック基点を確保 |
| 自動ロールバック | `loop` のリトライ上限到達時・`review` のリジェクト時に Git ロールバック |
| ブランチプレフィックス強制 | ブランチ名に所定のプレフィックスがない場合、`feature/` を自動付与 |

## 開発

開発のメインサイクル（ビルド、テスト、リント）は Docker Compose を通じて行います。

```bash
# テスト実行（Docker）
make test

# リント実行（Docker）
make lint

# ビルド（Docker イメージ）
make build

# ホスト環境に Go 1.24+ がある場合に使用可能なローカルターゲット
make test-local
make lint-local
make build-local

# グローバルインストール（ローカルマシンの $GOPATH/bin へ）
make install
```

### リリース

Git タグを打ってから `go install` すると、バージョン情報が自動的に埋め込まれます：

```bash
git tag v0.1.0
git push origin v0.1.0

# 利用者側
go install github.com/y-mitsuyoshi/nexus-weaver/cmd/nexus-weaver@v0.1.0
nexus-weaver --version
# nexus-weaver v0.1.0 (commit: abc1234)
```

## License

See [LICENSE](LICENSE) file.